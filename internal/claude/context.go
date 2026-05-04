package claude

import (
	"fmt"
	"strings"
	"sync"

	"github.com/mab-go/golem/internal/grpc/pb"
	"github.com/mab-go/golem/internal/memory"
	"github.com/mab-go/golem/internal/perception"
)

// ChatMessage is a player chat message buffered for inclusion in perception.
type ChatMessage struct {
	Sender  string
	Message string
}

// PerceptionSnapshot bundles the raw perception RPC responses captured at the
// start of a turn, plus events drained from the event buffer and any player
// chat that should be surfaced.
type PerceptionSnapshot struct {
	Vitals       *pb.GetVitalSignsResponse
	Surroundings *pb.GetSurroundingsResponse
	Inventory    *pb.GetInventoryResponse
	Events       []*pb.GameEvent
	// ClassifiedEvents is the optional output of ClassifyEvents. When non-nil
	// the context builder uses it in place of Events so the formatter can
	// emphasize or collapse by priority. A nil slice means classification was
	// skipped; the builder falls back to chronological Events rendering.
	ClassifiedEvents []perception.ClassifiedEvent
	// ActiveTask is a snapshot of the running background task, or nil.
	ActiveTask *perception.TaskStatus
	// ChatMessages holds player chat messages to surface this turn.
	ChatMessages []ChatMessage
	// HeartbeatNote is a short message injected on heartbeat wakes to give the
	// conscious mind temporal context.
	HeartbeatNote string
}

// History holds the trailing conversation. Access is mutex-guarded because the
// agent loop writes from the main goroutine while the event goroutine may read
// during status reporting.
type History struct {
	mu       sync.Mutex
	max      int
	messages []Message
}

// NewHistory constructs a History that retains up to maxMessages. A value <= 0
// disables trimming (unbounded growth -- not recommended for long sessions).
func NewHistory(maxMessages int) *History {
	return &History{max: maxMessages}
}

// Snapshot returns a deep copy of the stored messages so mutations by the
// caller cannot leak back into history.
func (h *History) Snapshot() []Message {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]Message, len(h.messages))
	for i, m := range h.messages {
		out[i] = Message{
			Role:    m.Role,
			Content: append([]Block(nil), m.Content...),
		}
	}
	return out
}

// Append stores one or more messages, trimming on conversation-start
// boundaries when the retained count exceeds the configured maximum. A
// conversation start is a user message with no tool_result blocks -- i.e.
// the fresh perception message produced at the top of each tick. Trimming
// in the middle of a tool_use/tool_result pair would orphan the result and
// get rejected by the Messages API as a 400, so when no safe boundary
// exists within the overflow window we leave history alone and let a
// future append (the next tick's perception) do the trim.
func (h *History) Append(msgs ...Message) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.messages = append(h.messages, msgs...)
	if h.max <= 0 || len(h.messages) <= h.max {
		return
	}
	desired := len(h.messages) - h.max
	for i := desired; i < len(h.messages); i++ {
		if isConversationStart(h.messages[i]) {
			h.messages = append([]Message(nil), h.messages[i:]...)
			return
		}
	}
}

// isConversationStart reports whether msg can legally be the first message
// sent to the API: a user message whose content has no tool_result blocks.
func isConversationStart(msg Message) bool {
	if msg.Role != RoleUser {
		return false
	}
	for _, b := range msg.Content {
		if b.Type == BlockToolResult {
			return false
		}
	}
	return true
}

// Reset clears history.
func (h *History) Reset() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.messages = nil
}

// Len reports the number of retained messages.
func (h *History) Len() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.messages)
}

// ContextBuilder renders per-turn user messages from memory + perception.
type ContextBuilder struct {
	Memory    *memory.Manager
	Formatter *perception.Formatter
	History   *History
}

// NewContextBuilder constructs a builder with required collaborators.
func NewContextBuilder(mem *memory.Manager, fmtr *perception.Formatter, hist *History) *ContextBuilder {
	return &ContextBuilder{Memory: mem, Formatter: fmtr, History: hist}
}

// BuildUserMessage renders the per-turn user message that carries memory, live
// perception, and any events since the last turn. Returns a single-text-block
// Message so token accounting is simple.
func (cb *ContextBuilder) BuildUserMessage(snap PerceptionSnapshot) (Message, error) {
	state, err := cb.Memory.ReadAll()
	if err != nil {
		return Message{}, fmt.Errorf("read memory: %w", err)
	}

	var b strings.Builder
	b.WriteString("# Memory (persistent)\n\n")
	b.WriteString("## goals.md\n\n")
	b.WriteString(trimFile(state.Goals))
	b.WriteString("\n\n## journal.md (most recent)\n\n")
	b.WriteString(tailLines(state.Journal, 80))
	b.WriteString("\n\n## world-knowledge.md\n\n")
	b.WriteString(trimFile(state.WorldKnowledge))
	b.WriteString("\n\n## inventory-notes.json\n\n")
	b.WriteString(trimFile(state.InventoryNotes))
	b.WriteString("\n\n## social.md\n\n")
	b.WriteString(trimFile(state.Social))
	b.WriteString("\n\n")

	b.WriteString("# Perception (this turn)\n\n")
	fmt.Fprintf(&b, "Vitals: %s\n\n", cb.Formatter.FormatVitalSigns(snap.Vitals))
	fmt.Fprintf(&b, "Surroundings:\n%s\n\n", cb.Formatter.FormatSurroundings(snap.Surroundings))
	fmt.Fprintf(&b, "%s\n\n", cb.Formatter.FormatInventory(snap.Inventory))

	b.WriteString("# Events (since last turn)\n\n")
	if snap.ClassifiedEvents != nil {
		b.WriteString(cb.Formatter.FormatClassifiedEvents(snap.ClassifiedEvents))
	} else {
		b.WriteString(cb.Formatter.FormatEvents(snap.Events))
	}
	b.WriteString("\n")

	if snap.HeartbeatNote != "" {
		fmt.Fprintf(&b, "\n%s\n", snap.HeartbeatNote)
	}

	if snap.ActiveTask != nil {
		b.WriteString("\n# Active Background Task\n\n")
		b.WriteString(cb.Formatter.FormatActiveTask(snap.ActiveTask))
		b.WriteString("\n")
	}

	if len(snap.ChatMessages) > 0 {
		b.WriteString("\n# Player Chat\n\n")
		for _, cm := range snap.ChatMessages {
			fmt.Fprintf(&b, "%s: %q\n", cm.Sender, cm.Message)
		}
		b.WriteString("\nRespond to the player(s) using send_chat when appropriate.\n")
	}

	return Message{
		Role:    RoleUser,
		Content: []Block{{Type: BlockText, Text: b.String()}},
	}, nil
}

// BuildMessages returns history + the new user message for SendMessage.
func (cb *ContextBuilder) BuildMessages(newUser Message) []Message {
	hist := cb.History.Snapshot()
	out := make([]Message, 0, len(hist)+1)
	out = append(out, hist...)
	out = append(out, newUser)
	return out
}

// trimFile limits a file's body to a sane size for context inclusion. Very
// long files are truncated from the start so the tail (most recent changes)
// is preserved.
func trimFile(s string) string {
	const limit = 4096
	s = strings.TrimSpace(s)
	if len(s) <= limit {
		return s
	}
	return "...(truncated)...\n" + s[len(s)-limit:]
}

// tailLines keeps only the last n non-empty lines of s.
func tailLines(s string, n int) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	lines := strings.Split(s, "\n")
	if len(lines) <= n {
		return s
	}
	return "...(older journal entries trimmed)...\n" + strings.Join(lines[len(lines)-n:], "\n")
}
