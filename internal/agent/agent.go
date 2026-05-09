package agent

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mab-go/golem/internal/claude"
	"github.com/mab-go/golem/internal/game"
	sidecar "github.com/mab-go/golem/internal/grpc"
	"github.com/mab-go/golem/internal/grpc/pb"
	"github.com/mab-go/golem/internal/logging"
	"github.com/mab-go/golem/internal/memory"
	"github.com/mab-go/golem/internal/perception"
	"github.com/mab-go/golem/internal/publisher"
	"github.com/mab-go/golem/internal/task"
)

// Config bundles the tunables the agent needs at construction.
type Config struct {
	BotUsername            string
	PerceptionFormat       perception.Format
	PerceptionRadius       int32
	HistoryMessages        int
	MaxToolRounds          int
	TaskTimeout            time.Duration
	PerceptionTickInterval time.Duration
	HeartbeatInterval      time.Duration
	GatekeeperTimeout      time.Duration
}

func (c *Config) applyDefaults() {
	if c.BotUsername == "" {
		c.BotUsername = "claude"
	}
	if c.PerceptionRadius <= 0 {
		c.PerceptionRadius = 16
	}
	if c.HistoryMessages <= 0 {
		c.HistoryMessages = 80
	}
	if c.MaxToolRounds <= 0 {
		c.MaxToolRounds = 12
	}
	if c.TaskTimeout <= 0 {
		c.TaskTimeout = 10 * time.Minute
	}
	if c.PerceptionTickInterval <= 0 {
		c.PerceptionTickInterval = 3 * time.Second
	}
	if c.HeartbeatInterval <= 0 {
		c.HeartbeatInterval = 45 * time.Second
	}
	if c.GatekeeperTimeout <= 0 {
		c.GatekeeperTimeout = 5 * time.Second
	}
}

// Agent owns the Perceive -> Think -> Act -> Remember loop.
type Agent struct {
	cfg       Config
	client    SidecarClient
	ai        AIClient
	memory    *memory.Manager
	formatter *perception.Formatter
	pacing    *PacingState
	history   *claude.History
	builder   *claude.ContextBuilder
	tools     []claude.Tool
	gkHealth  *gatekeeperHealth
	log       logging.Logger
	pub       publisher.EventPublisher

	// Event plumbing
	eventMu       sync.Mutex
	pendingEvents []*pb.GameEvent
	chatBuf       []pendingChat
	wakeChan      chan WakeSignal
	chatHandler   *ChatInterceptor
	dispatcher    ToolDispatcher
	taskMgr       *task.Manager
	shutdownFunc  context.CancelFunc

	// flight tracks whether a tool call is currently blocking the think loop.
	flight *toolFlight

	// lastWakeAt stores the unix nanosecond timestamp of the last think cycle.
	lastWakeAt atomic.Int64

	// cycle is a monotonic per-agent counter incremented at the top of every
	// think cycle. Disambiguates log entries across the whole session.
	cycle uint64
}

// New constructs an Agent and wires all collaborators. ctx is the
// agent's long-lived root context used for background tasks;
// shutdownFunc is its cancel func, called by `/golem shutdown`.
func New(
	ctx context.Context,
	cfg Config,
	client *sidecar.Client,
	ai *claude.Client,
	mem *memory.Manager,
	shutdownFunc context.CancelFunc,
	pub publisher.EventPublisher,
	log logging.Logger,
) *Agent {
	cfg.applyDefaults()
	if log == nil {
		log = logging.NewDefaultLogger()
	}
	if pub == nil {
		pub = publisher.Nop()
	}
	formatter := perception.NewFormatter(cfg.PerceptionFormat, perception.VerbosityStandard)
	pacing := NewPacingState(perception.VerbosityStandard)
	history := claude.NewHistory(cfg.HistoryMessages)
	builder := claude.NewContextBuilder(mem, formatter, history)

	a := &Agent{
		cfg:          cfg,
		client:       client,
		ai:           ai,
		memory:       mem,
		formatter:    formatter,
		pacing:       pacing,
		history:      history,
		builder:      builder,
		tools:        claude.Tools(),
		log:          log,
		pub:          pub,
		wakeChan:     make(chan WakeSignal, 1),
		flight:       &toolFlight{},
		shutdownFunc: shutdownFunc,
	}
	a.taskMgr = task.NewManager(ctx, a.appendEvent, func() {
		a.sendWake(WakeSignal{Reason: WakeBypassTask})
	}, cfg.TaskTimeout, log)
	a.gkHealth = newGatekeeperHealth(5*time.Minute, 0.05, log)
	a.dispatcher = game.NewDispatcher(cfg.BotUsername, client, mem, a, a.taskMgr, pub, log)
	a.chatHandler = NewChatInterceptor(client, pacing, mem, history, cfg.BotUsername, shutdownFunc, log)
	return a
}

// SetVerbosity implements game.State -- dispatcher calls this when Claude uses
// set_verbosity. It keeps the formatter and pacing state in sync.
func (a *Agent) SetVerbosity(v perception.VerbosityMode) {
	a.formatter.SetVerbosity(v)
	a.pacing.SetVerbosity(v)
}

// sendWake sends a wake signal, coalescing redundant signals.
func (a *Agent) sendWake(sig WakeSignal) {
	select {
	case a.wakeChan <- sig:
	default:
	}
}

// appendEvent stores an event for the next turn. Thread-safe.
func (a *Agent) appendEvent(evt *pb.GameEvent) {
	a.eventMu.Lock()
	defer a.eventMu.Unlock()
	a.pendingEvents = append(a.pendingEvents, evt)
}

// drainEvents returns and clears the pending event buffer. Thread-safe.
func (a *Agent) drainEvents() []*pb.GameEvent {
	a.eventMu.Lock()
	defer a.eventMu.Unlock()
	out := a.pendingEvents
	a.pendingEvents = nil
	return out
}

// pendingChat is a chat message buffered for the next perception cycle.
type pendingChat struct {
	Message string
	Sender  string
}

// appendChat stores a player chat message. Multiple messages accumulate until
// drainChat is called. Thread-safe.
func (a *Agent) appendChat(msg, from string) {
	a.eventMu.Lock()
	defer a.eventMu.Unlock()
	a.chatBuf = append(a.chatBuf, pendingChat{Message: msg, Sender: from})
}

// drainChat returns and clears all pending chat messages. Thread-safe.
func (a *Agent) drainChat() []pendingChat {
	a.eventMu.Lock()
	defer a.eventMu.Unlock()
	out := a.chatBuf
	a.chatBuf = nil
	return out
}

// fetchPerception pulls vitals, surroundings, and inventory from the sidecar
// concurrently. On any error the loop continues with partial data.
func (a *Agent) fetchPerception(ctx context.Context) claude.PerceptionSnapshot {
	var (
		snap claude.PerceptionSnapshot
		wg   sync.WaitGroup
	)

	fetch := func(name string, fn func() error) {
		defer wg.Done()
		if err := fn(); err != nil {
			a.log.WithError(err).WithField("source", name).Warn(eventPerceptionFailed)
		}
	}

	wg.Add(3)
	go fetch("vitals", func() error {
		v, err := a.client.GetVitalSigns(ctx)
		if err != nil {
			return err
		}
		snap.Vitals = v
		return nil
	})
	go fetch("surroundings", func() error {
		s, err := a.client.GetSurroundings(ctx, a.cfg.PerceptionRadius, false)
		if err != nil {
			return err
		}
		snap.Surroundings = s
		return nil
	})
	go fetch("inventory", func() error {
		inv, err := a.client.GetInventory(ctx, true)
		if err != nil {
			return err
		}
		snap.Inventory = inv
		return nil
	})
	wg.Wait()
	return snap
}

// fetchLightPerception pulls vitals and surroundings (no inventory) for the
// gatekeeper's compact evaluation.
func (a *Agent) fetchLightPerception(ctx context.Context) (*pb.GetVitalSignsResponse, *pb.GetSurroundingsResponse) {
	var (
		vitals *pb.GetVitalSignsResponse
		surr   *pb.GetSurroundingsResponse
		wg     sync.WaitGroup
	)
	wg.Add(2)
	go func() {
		defer wg.Done()
		v, err := a.client.GetVitalSigns(ctx)
		if err != nil {
			a.log.WithError(err).Debug(eventVitalsFailed)
			return
		}
		vitals = v
	}()
	go func() {
		defer wg.Done()
		s, err := a.client.GetSurroundings(ctx, a.cfg.PerceptionRadius, false)
		if err != nil {
			a.log.WithError(err).Debug(eventSurroundingsFailed)
			return
		}
		surr = s
	}()
	wg.Wait()
	return vitals, surr
}

// sleepFor waits for `d`, cancellation, or a wake-up signal. Returns true if
// the sleep completed uninterrupted; false if context cancelled.
func (a *Agent) sleepFor(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		select {
		case <-ctx.Done():
			return false
		default:
			return true
		}
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	case <-a.wakeChan:
		return true
	}
}

// formatToolUsesForLog is a small helper for structured logging in loop.go.
func formatToolUsesForLog(uses []claude.ToolUse) string {
	if len(uses) == 0 {
		return ""
	}
	return fmt.Sprintf("%v", extractToolNames(uses))
}

func extractToolNames(uses []claude.ToolUse) []string {
	if len(uses) == 0 {
		return nil
	}
	names := make([]string, len(uses))
	for i, u := range uses {
		names[i] = u.Name
	}
	return names
}
