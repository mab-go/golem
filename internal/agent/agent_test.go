package agent

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/mab-go/golem/internal/claude"
	"github.com/mab-go/golem/internal/grpc/pb"
	"github.com/mab-go/golem/internal/perception"
)

func TestApplyDefaultsZeroConfig(t *testing.T) {
	var c Config
	c.applyDefaults()
	if c.BotUsername != "claude" {
		t.Errorf("BotUsername = %q, want %q", c.BotUsername, "claude")
	}
	if c.PerceptionRadius != 16 {
		t.Errorf("PerceptionRadius = %d, want 16", c.PerceptionRadius)
	}
	if c.HistoryMessages != 80 {
		t.Errorf("HistoryMessages = %d, want 80", c.HistoryMessages)
	}
	if c.MaxToolRounds != 12 {
		t.Errorf("MaxToolRounds = %d, want 12", c.MaxToolRounds)
	}
	if c.TaskTimeout != 10*time.Minute {
		t.Errorf("TaskTimeout = %v, want 10m", c.TaskTimeout)
	}
	if c.PerceptionTickInterval != 3*time.Second {
		t.Errorf("PerceptionTickInterval = %v, want 3s", c.PerceptionTickInterval)
	}
	if c.HeartbeatInterval != 45*time.Second {
		t.Errorf("HeartbeatInterval = %v, want 45s", c.HeartbeatInterval)
	}
	if c.GatekeeperTimeout != 5*time.Second {
		t.Errorf("GatekeeperTimeout = %v, want 5s", c.GatekeeperTimeout)
	}
}

func TestApplyDefaultsPreservesSetFields(t *testing.T) {
	c := Config{
		BotUsername:      "custom",
		PerceptionRadius: 32,
		MaxToolRounds:    5,
	}
	c.applyDefaults()
	if c.BotUsername != "custom" {
		t.Errorf("BotUsername = %q, want %q", c.BotUsername, "custom")
	}
	if c.PerceptionRadius != 32 {
		t.Errorf("PerceptionRadius = %d, want 32", c.PerceptionRadius)
	}
	if c.MaxToolRounds != 5 {
		t.Errorf("MaxToolRounds = %d, want 5", c.MaxToolRounds)
	}
	if c.HistoryMessages != 80 {
		t.Errorf("HistoryMessages should default to 80, got %d", c.HistoryMessages)
	}
}

func TestExtractToolNames(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		if got := extractToolNames(nil); got != nil {
			t.Errorf("expected nil, got %v", got)
		}
	})

	t.Run("empty", func(t *testing.T) {
		if got := extractToolNames([]claude.ToolUse{}); got != nil {
			t.Errorf("expected nil, got %v", got)
		}
	})

	t.Run("populated", func(t *testing.T) {
		uses := []claude.ToolUse{
			{Name: "move_to"},
			{Name: "dig_block"},
		}
		got := extractToolNames(uses)
		if len(got) != 2 || got[0] != "move_to" || got[1] != "dig_block" {
			t.Errorf("got %v, want [move_to dig_block]", got)
		}
	})
}

func TestFormatToolUsesForLog(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		if got := formatToolUsesForLog(nil); got != "" {
			t.Errorf("expected empty, got %q", got)
		}
	})

	t.Run("non_empty", func(t *testing.T) {
		uses := []claude.ToolUse{{Name: "look_around"}}
		got := formatToolUsesForLog(uses)
		if got == "" {
			t.Error("expected non-empty string")
		}
	})
}

func TestTailJournal(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		if got := tailJournal("", 5); got != "" {
			t.Errorf("expected empty, got %q", got)
		}
	})

	t.Run("whitespace_only", func(t *testing.T) {
		if got := tailJournal("   \n  ", 5); got != "" {
			t.Errorf("expected empty, got %q", got)
		}
	})

	t.Run("fewer_than_n", func(t *testing.T) {
		s := "line1\nline2\nline3"
		if got := tailJournal(s, 5); got != s {
			t.Errorf("expected full string, got %q", got)
		}
	})

	t.Run("exactly_n", func(t *testing.T) {
		s := "a\nb\nc"
		if got := tailJournal(s, 3); got != s {
			t.Errorf("expected full string, got %q", got)
		}
	})

	t.Run("more_than_n", func(t *testing.T) {
		s := "a\nb\nc\nd\ne"
		got := tailJournal(s, 2)
		if got != "d\ne" {
			t.Errorf("expected last 2 lines, got %q", got)
		}
	})
}

func TestTruncateResult(t *testing.T) {
	t.Run("within_limit", func(t *testing.T) {
		if got := truncateResult("hello", 10); got != "hello" {
			t.Errorf("got %q", got)
		}
	})

	t.Run("over_limit", func(t *testing.T) {
		got := truncateResult("hello world", 5)
		if got != "hello...(truncated)" {
			t.Errorf("got %q", got)
		}
	})

	t.Run("exact_limit", func(t *testing.T) {
		if got := truncateResult("hello", 5); got != "hello" {
			t.Errorf("got %q", got)
		}
	})
}

func TestMinDuration(t *testing.T) {
	if got := minDuration(1*time.Second, 2*time.Second); got != 1*time.Second {
		t.Errorf("min(1s,2s) = %v", got)
	}
	if got := minDuration(3*time.Second, 1*time.Second); got != 1*time.Second {
		t.Errorf("min(3s,1s) = %v", got)
	}
	if got := minDuration(5*time.Second, 5*time.Second); got != 5*time.Second {
		t.Errorf("min(5s,5s) = %v", got)
	}
}

func TestTruncForLog(t *testing.T) {
	t.Run("within_limit", func(t *testing.T) {
		if got := truncForLog("hello", 10); got != "hello" {
			t.Errorf("got %q", got)
		}
	})

	t.Run("over_limit", func(t *testing.T) {
		got := truncForLog("hello world", 5)
		want := "hello…"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})
}

func TestFormatName(t *testing.T) {
	if got := formatName(perception.FormatStructured); got != "structured" {
		t.Errorf("FormatStructured -> %q, want %q", got, "structured")
	}
	if got := formatName(perception.FormatProse); got != "prose" {
		t.Errorf("FormatProse -> %q, want %q", got, "prose")
	}
}

func TestCloneRaw(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		if got := cloneRaw(nil); got != nil {
			t.Errorf("expected nil, got %v", got)
		}
	})

	t.Run("empty", func(t *testing.T) {
		if got := cloneRaw(json.RawMessage{}); got != nil {
			t.Errorf("expected nil, got %v", got)
		}
	})

	t.Run("independent_copy", func(t *testing.T) {
		src := json.RawMessage(`{"x":1}`)
		got := cloneRaw(src)
		if string(got) != `{"x":1}` {
			t.Errorf("got %q", got)
		}
		src[0] = '!'
		if got[0] == '!' {
			t.Error("clone should be independent of source")
		}
	})
}

func TestAssistantMessageFromResponse(t *testing.T) {
	t.Run("text_and_tool_use", func(t *testing.T) {
		resp := &claude.Response{
			Blocks: []claude.Block{
				{Type: claude.BlockText, Text: "thinking..."},
				{Type: claude.BlockToolUse, ID: "t1", Name: "move_to", Input: json.RawMessage(`{"x":1}`)},
			},
		}
		msg := assistantMessageFromResponse(resp)
		if msg.Role != claude.RoleAssistant {
			t.Errorf("role = %q, want assistant", msg.Role)
		}
		if len(msg.Content) != 2 {
			t.Fatalf("expected 2 blocks, got %d", len(msg.Content))
		}
		if msg.Content[0].Text != "thinking..." {
			t.Errorf("text = %q", msg.Content[0].Text)
		}
		if msg.Content[1].Name != "move_to" {
			t.Errorf("tool name = %q", msg.Content[1].Name)
		}
	})

	t.Run("empty_text_skipped", func(t *testing.T) {
		resp := &claude.Response{
			Blocks: []claude.Block{
				{Type: claude.BlockText, Text: ""},
				{Type: claude.BlockText, Text: "real text"},
			},
		}
		msg := assistantMessageFromResponse(resp)
		if len(msg.Content) != 1 {
			t.Fatalf("expected 1 block (empty skipped), got %d", len(msg.Content))
		}
		if msg.Content[0].Text != "real text" {
			t.Errorf("text = %q", msg.Content[0].Text)
		}
	})

	t.Run("no_blocks", func(t *testing.T) {
		resp := &claude.Response{}
		msg := assistantMessageFromResponse(resp)
		if len(msg.Content) != 0 {
			t.Errorf("expected 0 blocks, got %d", len(msg.Content))
		}
	})
}

func TestInterruptedResult(t *testing.T) {
	ie := &toolInterruptError{Reason: CancelEmergencyStop, Detail: "player issued stop"}
	got := interruptedResult("navigate_to", ie)
	if got.Text == "" {
		t.Fatal("expected non-empty text")
	}
	if len(got.ImagePNG) != 0 {
		t.Error("expected no image")
	}
}

func TestToolInterruptErrorString(t *testing.T) {
	ie := &toolInterruptError{Reason: CancelCriticalEvent, Detail: "bot died"}
	got := ie.Error()
	if got == "" {
		t.Fatal("expected non-empty error string")
	}
}

// --- Minimal-setup Agent method tests ---

func TestSendWake(t *testing.T) {
	a := &Agent{wakeChan: make(chan WakeSignal, 1)}

	a.sendWake(WakeSignal{Reason: WakeHeartbeat})
	select {
	case sig := <-a.wakeChan:
		if sig.Reason != WakeHeartbeat {
			t.Errorf("reason = %v, want WakeHeartbeat", sig.Reason)
		}
	default:
		t.Fatal("expected signal on channel")
	}

	// Fill channel, then send again (should not block)
	a.wakeChan <- WakeSignal{Reason: WakeHeartbeat}
	a.sendWake(WakeSignal{Reason: WakeGatekeeper})
}

func TestSleepFor(t *testing.T) {
	t.Run("zero_duration_alive", func(t *testing.T) {
		a := &Agent{wakeChan: make(chan WakeSignal, 1)}
		if !a.sleepFor(context.Background(), 0) {
			t.Error("expected true for zero duration with live context")
		}
	})

	t.Run("zero_duration_cancelled", func(t *testing.T) {
		a := &Agent{wakeChan: make(chan WakeSignal, 1)}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if a.sleepFor(ctx, 0) {
			t.Error("expected false for zero duration with cancelled context")
		}
	})

	t.Run("wake_signal_interrupts", func(t *testing.T) {
		a := &Agent{wakeChan: make(chan WakeSignal, 1)}
		a.wakeChan <- WakeSignal{Reason: WakeHeartbeat}
		if !a.sleepFor(context.Background(), 10*time.Second) {
			t.Error("expected true when wake signal received")
		}
	})

	t.Run("context_cancel_returns_false", func(t *testing.T) {
		a := &Agent{wakeChan: make(chan WakeSignal, 1)}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if a.sleepFor(ctx, 10*time.Second) {
			t.Error("expected false when context cancelled")
		}
	})
}

func TestSetVerbosity(t *testing.T) {
	a := &Agent{
		formatter: perception.NewFormatter(perception.FormatProse, perception.VerbosityStandard),
		pacing:    NewPacingState(perception.VerbosityStandard),
	}

	a.SetVerbosity(perception.VerbosityTerse)

	if a.pacing.Verbosity() != perception.VerbosityTerse {
		t.Errorf("pacing verbosity = %v, want terse", a.pacing.Verbosity())
	}
}

// --- Sidecar mock tests ---

func TestFetchPerception(t *testing.T) {
	t.Run("all_succeed", func(t *testing.T) {
		a, ms, _, _ := newTestAgent(t)
		ms.GetVitalSignsFunc = func(_ context.Context) (*pb.GetVitalSignsResponse, error) {
			return &pb.GetVitalSignsResponse{Health: 20, Food: 18}, nil
		}
		ms.GetSurroundingsFunc = func(_ context.Context, _ int32, _ bool) (*pb.GetSurroundingsResponse, error) {
			return &pb.GetSurroundingsResponse{Biome: "plains"}, nil
		}
		ms.GetInventoryFunc = func(_ context.Context, _ bool) (*pb.GetInventoryResponse, error) {
			return &pb.GetInventoryResponse{EmptySlots: 30}, nil
		}

		snap := a.fetchPerception(context.Background())
		if snap.Vitals == nil || snap.Vitals.Health != 20 {
			t.Errorf("vitals = %v", snap.Vitals)
		}
		if snap.Surroundings == nil || snap.Surroundings.Biome != "plains" {
			t.Errorf("surroundings = %v", snap.Surroundings)
		}
		if snap.Inventory == nil || snap.Inventory.EmptySlots != 30 {
			t.Errorf("inventory = %v", snap.Inventory)
		}
	})

	t.Run("partial_failure", func(t *testing.T) {
		a, ms, _, _ := newTestAgent(t)
		ms.GetVitalSignsFunc = func(_ context.Context) (*pb.GetVitalSignsResponse, error) {
			return &pb.GetVitalSignsResponse{Health: 15}, nil
		}
		ms.GetSurroundingsFunc = func(_ context.Context, _ int32, _ bool) (*pb.GetSurroundingsResponse, error) {
			return nil, context.DeadlineExceeded
		}
		ms.GetInventoryFunc = func(_ context.Context, _ bool) (*pb.GetInventoryResponse, error) {
			return &pb.GetInventoryResponse{EmptySlots: 10}, nil
		}

		snap := a.fetchPerception(context.Background())
		if snap.Vitals == nil {
			t.Error("vitals should still be populated")
		}
		if snap.Surroundings != nil {
			t.Error("surroundings should be nil on error")
		}
		if snap.Inventory == nil {
			t.Error("inventory should still be populated")
		}
	})
}

func TestFetchLightPerception(t *testing.T) {
	t.Run("both_succeed", func(t *testing.T) {
		a, ms, _, _ := newTestAgent(t)
		ms.GetVitalSignsFunc = func(_ context.Context) (*pb.GetVitalSignsResponse, error) {
			return &pb.GetVitalSignsResponse{Health: 20}, nil
		}
		ms.GetSurroundingsFunc = func(_ context.Context, _ int32, _ bool) (*pb.GetSurroundingsResponse, error) {
			return &pb.GetSurroundingsResponse{Biome: "forest"}, nil
		}

		vitals, surr := a.fetchLightPerception(context.Background())
		if vitals == nil || vitals.Health != 20 {
			t.Errorf("vitals = %v", vitals)
		}
		if surr == nil || surr.Biome != "forest" {
			t.Errorf("surr = %v", surr)
		}
	})

	t.Run("vitals_fail", func(t *testing.T) {
		a, ms, _, _ := newTestAgent(t)
		ms.GetVitalSignsFunc = func(_ context.Context) (*pb.GetVitalSignsResponse, error) {
			return nil, context.DeadlineExceeded
		}
		ms.GetSurroundingsFunc = func(_ context.Context, _ int32, _ bool) (*pb.GetSurroundingsResponse, error) {
			return &pb.GetSurroundingsResponse{Biome: "desert"}, nil
		}

		vitals, surr := a.fetchLightPerception(context.Background())
		if vitals != nil {
			t.Error("expected nil vitals on error")
		}
		if surr == nil {
			t.Error("surroundings should still be populated")
		}
	})
}

func TestShouldThrottleRoutine(t *testing.T) {
	t.Run("routine_healthy_recent", func(t *testing.T) {
		a := &Agent{cfg: Config{HeartbeatInterval: 45 * time.Second}}
		a.lastWakeAt.Store(time.Now().UnixNano())
		events := []*pb.GameEvent{
			{Type: pb.EventType_EVENT_WEATHER_CHANGE, Priority: pb.EventPriority_EVENT_PRIORITY_LOW},
		}
		vitals := &pb.GetVitalSignsResponse{Health: 20, Food: 20}
		if !a.shouldThrottleRoutine(events, vitals) {
			t.Error("expected throttle for routine events with healthy vitals and recent wake")
		}
	})

	t.Run("non_routine_event", func(t *testing.T) {
		a := &Agent{cfg: Config{HeartbeatInterval: 45 * time.Second}}
		a.lastWakeAt.Store(time.Now().UnixNano())
		events := []*pb.GameEvent{
			{Type: pb.EventType_EVENT_CHAT_MESSAGE, Priority: pb.EventPriority_EVENT_PRIORITY_HIGH},
		}
		vitals := &pb.GetVitalSignsResponse{Health: 20, Food: 20}
		if a.shouldThrottleRoutine(events, vitals) {
			t.Error("should not throttle non-routine events")
		}
	})

	t.Run("low_health", func(t *testing.T) {
		a := &Agent{cfg: Config{HeartbeatInterval: 45 * time.Second}}
		a.lastWakeAt.Store(time.Now().UnixNano())
		events := []*pb.GameEvent{
			{Type: pb.EventType_EVENT_WEATHER_CHANGE, Priority: pb.EventPriority_EVENT_PRIORITY_LOW},
		}
		vitals := &pb.GetVitalSignsResponse{Health: 5, Food: 20}
		if a.shouldThrottleRoutine(events, vitals) {
			t.Error("should not throttle when health is low")
		}
	})

	t.Run("nil_vitals", func(t *testing.T) {
		a := &Agent{cfg: Config{HeartbeatInterval: 45 * time.Second}}
		a.lastWakeAt.Store(time.Now().UnixNano())
		if a.shouldThrottleRoutine(nil, nil) {
			t.Error("should not throttle with nil vitals")
		}
	})

	t.Run("stale_wake", func(t *testing.T) {
		a := &Agent{cfg: Config{HeartbeatInterval: 45 * time.Second}}
		a.lastWakeAt.Store(time.Now().Add(-time.Minute).UnixNano())
		events := []*pb.GameEvent{
			{Type: pb.EventType_EVENT_WEATHER_CHANGE, Priority: pb.EventPriority_EVENT_PRIORITY_LOW},
		}
		vitals := &pb.GetVitalSignsResponse{Health: 20, Food: 20}
		if a.shouldThrottleRoutine(events, vitals) {
			t.Error("should not throttle when wake is stale")
		}
	})
}
