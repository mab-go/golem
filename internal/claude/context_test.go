package claude

import (
	"strings"
	"testing"

	"github.com/mab-go/golem/internal/grpc/pb"
	"github.com/mab-go/golem/internal/memory"
	"github.com/mab-go/golem/internal/perception"
)

func TestHistoryAppendTrims(t *testing.T) {
	h := NewHistory(3)
	for range 5 {
		h.Append(Message{Role: RoleUser, Content: []Block{{Type: BlockText, Text: "m"}}})
	}
	if got := h.Len(); got != 3 {
		t.Errorf("Len=%d want 3", got)
	}
}

func TestHistoryAppendNeverLeavesOrphanToolResult(t *testing.T) {
	// Simulates the agent loop: perception -> assistant(tool_use) ->
	// user(tool_result) -> perception -> ... With a small bound, a naive
	// count-based trim would drop the assistant tool_use and leave the
	// following user(tool_result) as the new front of history -- a 400
	// from the Messages API.
	h := NewHistory(3)
	h.Append(Message{Role: RoleUser, Content: []Block{{Type: BlockText, Text: "perception 1"}}})
	h.Append(Message{Role: RoleAssistant, Content: []Block{{Type: BlockToolUse, ID: "tu_1", Name: "look"}}})
	h.Append(Message{Role: RoleUser, Content: []Block{{Type: BlockToolResult, ToolUseID: "tu_1", Content: "ok"}}})
	h.Append(Message{Role: RoleAssistant, Content: []Block{{Type: BlockToolUse, ID: "tu_2", Name: "move"}}})
	h.Append(Message{Role: RoleUser, Content: []Block{{Type: BlockToolResult, ToolUseID: "tu_2", Content: "ok"}}})
	h.Append(Message{Role: RoleUser, Content: []Block{{Type: BlockText, Text: "perception 2"}}})

	snap := h.Snapshot()
	if len(snap) == 0 {
		t.Fatal("snapshot empty after trimming")
	}
	first := snap[0]
	if first.Role != RoleUser {
		t.Errorf("first message role = %q, want user", first.Role)
	}
	for _, b := range first.Content {
		if b.Type == BlockToolResult {
			t.Errorf("first message must not carry tool_result (orphan): %+v", first)
		}
	}
}

func TestHistoryUnbounded(t *testing.T) {
	h := NewHistory(0)
	for range 10 {
		h.Append(Message{Role: RoleUser, Content: []Block{{Type: BlockText, Text: "m"}}})
	}
	if h.Len() != 10 {
		t.Errorf("Len=%d want 10", h.Len())
	}
}

func TestHistorySnapshotIsolated(t *testing.T) {
	h := NewHistory(5)
	h.Append(Message{Role: RoleUser, Content: []Block{{Type: BlockText, Text: "a"}}})
	snap := h.Snapshot()
	snap[0].Content[0].Text = "mutated"
	if h.Snapshot()[0].Content[0].Text != "a" {
		t.Error("snapshot shared storage with history")
	}
}

func TestBuildUserMessageIncludesMemoryAndPerception(t *testing.T) {
	dir := t.TempDir()
	mem, err := memory.New(dir)
	if err != nil {
		t.Fatalf("memory.New: %v", err)
	}
	if err := mem.UpdateGoals("- find iron"); err != nil {
		t.Fatalf("UpdateGoals: %v", err)
	}
	cb := NewContextBuilder(mem, perception.NewFormatter(perception.FormatProse, perception.VerbosityStandard), NewHistory(10))

	snap := PerceptionSnapshot{
		Vitals:       &pb.GetVitalSignsResponse{Health: 20, MaxHealth: 20, Food: 18},
		Surroundings: &pb.GetSurroundingsResponse{Biome: "plains", TimeOfDay: "noon", Position: &pb.Vec3{X: 1, Y: 2, Z: 3}},
		Inventory:    &pb.GetInventoryResponse{EmptySlots: 30},
		Events:       []*pb.GameEvent{{Type: pb.EventType_EVENT_CHAT_MESSAGE, Timestamp: 1000, Description: "alice: hi"}},
	}

	msg, err := cb.BuildUserMessage(snap)
	if err != nil {
		t.Fatalf("BuildUserMessage: %v", err)
	}
	body := msg.Content[0].Text
	for _, want := range []string{"goals.md", "find iron", "Vitals:", "plains", "alice: hi"} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %q in user message:\n%s", want, body)
		}
	}
}

func TestBuildUserMessagePrefersClassifiedEvents(t *testing.T) {
	dir := t.TempDir()
	mem, err := memory.New(dir)
	if err != nil {
		t.Fatalf("memory.New: %v", err)
	}
	cb := NewContextBuilder(mem, perception.NewFormatter(perception.FormatProse, perception.VerbosityStandard), NewHistory(10))

	raw := []*pb.GameEvent{
		{Type: pb.EventType_EVENT_WEATHER_CHANGE, Timestamp: 1000, Description: "it rains"},
		{Type: pb.EventType_EVENT_BOT_DEATH, Timestamp: 2000, Description: "bot died", Priority: pb.EventPriority_EVENT_PRIORITY_CRITICAL},
	}
	snap := PerceptionSnapshot{
		Vitals:       &pb.GetVitalSignsResponse{Health: 20, MaxHealth: 20},
		Surroundings: &pb.GetSurroundingsResponse{},
		Inventory:    &pb.GetInventoryResponse{},
		Events:       raw,
		ClassifiedEvents: []perception.ClassifiedEvent{
			{Event: raw[0], Priority: pb.EventPriority_EVENT_PRIORITY_LOW},
			{Event: raw[1], Priority: pb.EventPriority_EVENT_PRIORITY_CRITICAL, Reason: "lava"},
		},
	}
	msg, err := cb.BuildUserMessage(snap)
	if err != nil {
		t.Fatalf("BuildUserMessage: %v", err)
	}
	body := msg.Content[0].Text
	// FormatClassifiedEvents collapses LOW into a one-line summary. If the
	// builder had fallen back to FormatEvents, we'd see "it rains" verbatim.
	if strings.Contains(body, "it rains") {
		t.Errorf("ClassifiedEvents path should collapse LOW events, got:\n%s", body)
	}
	if !strings.Contains(body, "[LOW]") {
		t.Errorf("expected LOW summary line, got:\n%s", body)
	}
	if !strings.Contains(body, "bot died") || !strings.Contains(body, "lava") {
		t.Errorf("critical event reason missing:\n%s", body)
	}
}

func TestBuildUserMessageSurfacesPlayerChat(t *testing.T) {
	dir := t.TempDir()
	mem, _ := memory.New(dir)
	cb := NewContextBuilder(mem, perception.NewFormatter(perception.FormatProse, perception.VerbosityStandard), NewHistory(10))

	snap := PerceptionSnapshot{
		Vitals:       &pb.GetVitalSignsResponse{Health: 20, MaxHealth: 20},
		Surroundings: &pb.GetSurroundingsResponse{},
		Inventory:    &pb.GetInventoryResponse{},
		ChatMessages: []ChatMessage{
			{Sender: "alice", Message: "come help me"},
			{Sender: "bob", Message: "where are you?"},
		},
	}
	msg, err := cb.BuildUserMessage(snap)
	if err != nil {
		t.Fatalf("BuildUserMessage: %v", err)
	}
	body := msg.Content[0].Text
	if strings.Contains(body, "URGENT") {
		t.Errorf("should not contain URGENT: %s", body)
	}
	for _, want := range []string{"Player Chat", "alice", "come help me", "bob", "where are you?"} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %q in user message:\n%s", want, body)
		}
	}
}

func TestBuildMessagesPrependsHistory(t *testing.T) {
	dir := t.TempDir()
	mem, _ := memory.New(dir)
	cb := NewContextBuilder(mem, perception.NewFormatter(perception.FormatProse, perception.VerbosityStandard), NewHistory(10))
	cb.History.Append(Message{Role: RoleUser, Content: []Block{{Type: BlockText, Text: "past"}}})
	cb.History.Append(Message{Role: RoleAssistant, Content: []Block{{Type: BlockText, Text: "ok"}}})

	newUser := Message{Role: RoleUser, Content: []Block{{Type: BlockText, Text: "now"}}}
	msgs := cb.BuildMessages(newUser)
	if len(msgs) != 3 {
		t.Fatalf("want 3 messages, got %d", len(msgs))
	}
	if msgs[0].Content[0].Text != "past" || msgs[2].Content[0].Text != "now" {
		t.Errorf("ordering wrong: %+v", msgs)
	}
}
