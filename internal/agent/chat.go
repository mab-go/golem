package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/mab-go/golem/internal/claude"
	sidecar "github.com/mab-go/golem/internal/grpc"
	"github.com/mab-go/golem/internal/grpc/pb"
	"github.com/mab-go/golem/internal/logging"
	"github.com/mab-go/golem/internal/memory"
	"github.com/mab-go/golem/internal/perception"
)

// OperatorCommandPrefix prefixes operator commands in chat. Messages starting
// with this are handled locally and never reach Claude.
const OperatorCommandPrefix = "/golem"

// ChatInterceptor classifies CHAT_MESSAGE events and handles operator commands
// in-process. When a regular chat message passes through, it returns the text
// for the agent loop to surface as an Interrupt.
type ChatInterceptor struct {
	client      *sidecar.Client
	pacing      *PacingState
	memory      *memory.Manager
	history     *claude.History
	botUsername string
	shutdown    context.CancelFunc
	log         logging.Logger
}

// NewChatInterceptor constructs a chat interceptor. `shutdown` is invoked for
// `/golem shutdown`.
func NewChatInterceptor(
	client *sidecar.Client,
	pacing *PacingState,
	mem *memory.Manager,
	hist *claude.History,
	botUsername string,
	shutdown context.CancelFunc,
	log logging.Logger,
) *ChatInterceptor {
	if log == nil {
		log = logging.NewDefaultLogger()
	}
	return &ChatInterceptor{
		client:      client,
		pacing:      pacing,
		memory:      mem,
		history:     hist,
		botUsername: botUsername,
		shutdown:    shutdown,
		log:         log,
	}
}

// HandleResult tells the agent loop what to do with a chat event.
type HandleResult struct {
	// Handled is true when the interceptor fully processed the event (the
	// agent loop should drop it).
	Handled bool
	// HumanMessage is the plain chat text to surface to Claude (non-command,
	// from a human). Empty when Handled or when the event is the bot's own.
	HumanMessage string
	// Sender is the in-game name of the human who sent the message.
	Sender string
	// EmergencyStop is true when the player issued /golem stop. The agent
	// loop should cancel any in-flight tool with a structured reason.
	EmergencyStop bool
}

// Handle examines a CHAT_MESSAGE event and reacts. Results:
//   - Self-chat -> Handled=true, no surface.
//   - /golem command -> Handled=true, reply already sent via SendChat.
//   - Regular human chat -> Handled=false, HumanMessage/Sender populated.
func (c *ChatInterceptor) Handle(ctx context.Context, evt *pb.GameEvent) HandleResult {
	if evt == nil || evt.Type != pb.EventType_EVENT_CHAT_MESSAGE {
		return HandleResult{Handled: true}
	}
	sender := evt.PlayerName
	msg := strings.TrimSpace(evt.ChatMessage)
	if sender == c.botUsername || msg == "" {
		return HandleResult{Handled: true}
	}

	if rest, ok := strings.CutPrefix(msg, OperatorCommandPrefix); ok {
		reply, isStop := c.handleCommand(ctx, strings.TrimSpace(rest))
		if reply != "" {
			if err := c.client.SendChat(ctx, reply); err != nil {
				c.log.WithError(err).Warn(eventChatSendFailed)
			}
		}
		if isStop {
			return HandleResult{Handled: true, EmergencyStop: true, Sender: sender}
		}
		return HandleResult{Handled: true}
	}

	return HandleResult{Handled: false, HumanMessage: msg, Sender: sender}
}

// handleCommand parses and executes an operator command. It returns the reply
// to send in-chat, or "" if nothing should be sent.
func (c *ChatInterceptor) handleCommand(_ context.Context, cmd string) (string, bool) {
	cmd = strings.TrimSpace(cmd)
	switch strings.ToLower(cmd) {
	case "", "help":
		return "commands: pause, resume, stop, verbose, standard, terse, status, shutdown", false

	case "pause":
		c.pacing.SetPaused(true)
		return "paused; /golem resume to continue", false

	case "resume":
		c.pacing.SetPaused(false)
		return "resumed", false

	case "stop":
		return "stopping current action", true

	case "verbose", "standard", "terse":
		v, err := perception.ParseVerbosity(cmd)
		if err != nil {
			return "verbosity error: " + err.Error(), false
		}
		c.pacing.SetVerbosity(v)
		return fmt.Sprintf("verbosity: %s", v), false

	case "status":
		return c.statusLine(), false

	case "shutdown":
		if c.shutdown != nil {
			c.shutdown()
		}
		return "shutting down", false

	default:
		return fmt.Sprintf("unknown command %q (try /golem help)", cmd), false
	}
}

func (c *ChatInterceptor) statusLine() string {
	verbosity := c.pacing.Verbosity()
	paused := c.pacing.Paused()
	history := 0
	if c.history != nil {
		history = c.history.Len()
	}
	return fmt.Sprintf("paused=%t verbosity=%s history=%d memory=%s",
		paused, verbosity, history, c.memory.Dir())
}
