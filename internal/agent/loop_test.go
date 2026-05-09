package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/mab-go/golem/internal/claude"
	"github.com/mab-go/golem/internal/game"
	"github.com/mab-go/golem/internal/grpc/pb"
	"github.com/mab-go/golem/internal/perception"
)

func TestBuildMidRoundTextEmpty(t *testing.T) {
	a := &Agent{
		formatter: perception.NewFormatter(perception.FormatProse, perception.VerbosityStandard),
	}
	if text := a.buildMidRoundText(); text != "" {
		t.Errorf("expected empty, got %q", text)
	}
}

func TestBuildMidRoundTextWithEvents(t *testing.T) {
	a := &Agent{
		formatter: perception.NewFormatter(perception.FormatProse, perception.VerbosityStandard),
	}
	a.appendEvent(&pb.GameEvent{
		Type:        pb.EventType_EVENT_CHAT_MESSAGE,
		Description: "alice: hello",
	})

	text := a.buildMidRoundText()
	if !strings.Contains(text, "[Mid-round update]") {
		t.Errorf("missing header, got %q", text)
	}
	if !strings.Contains(text, "New events since last tool call") {
		t.Errorf("missing events section, got %q", text)
	}
}

func TestBuildMidRoundTextWithChat(t *testing.T) {
	a := &Agent{
		formatter: perception.NewFormatter(perception.FormatProse, perception.VerbosityStandard),
	}
	a.appendChat("where are you?", "bob")

	text := a.buildMidRoundText()
	if !strings.Contains(text, "[Mid-round update]") {
		t.Errorf("missing header, got %q", text)
	}
	if !strings.Contains(text, "Player chat:") {
		t.Errorf("missing chat section, got %q", text)
	}
	if !strings.Contains(text, "bob") || !strings.Contains(text, "where are you?") {
		t.Errorf("missing chat content, got %q", text)
	}
}

func TestBuildMidRoundTextWithBoth(t *testing.T) {
	a := &Agent{
		formatter: perception.NewFormatter(perception.FormatProse, perception.VerbosityStandard),
	}
	a.appendEvent(&pb.GameEvent{
		Type:        pb.EventType_EVENT_WEATHER_CHANGE,
		Description: "it started raining",
	})
	a.appendChat("nice weather", "alice")

	text := a.buildMidRoundText()
	if !strings.Contains(text, "New events since last tool call") {
		t.Errorf("missing events section, got %q", text)
	}
	if !strings.Contains(text, "Player chat:") {
		t.Errorf("missing chat section, got %q", text)
	}
}

func TestBuildMidRoundTextDrains(t *testing.T) {
	a := &Agent{
		formatter: perception.NewFormatter(perception.FormatProse, perception.VerbosityStandard),
	}
	a.appendEvent(&pb.GameEvent{
		Type:        pb.EventType_EVENT_CHAT_MESSAGE,
		Description: "test",
	})
	a.appendChat("hello", "alice")

	first := a.buildMidRoundText()
	if first == "" {
		t.Fatal("first call should return content")
	}

	second := a.buildMidRoundText()
	if second != "" {
		t.Errorf("second call should return empty after drain, got %q", second)
	}
}

// --- Stage 4: AI + dispatcher mock tests ---

func stubSidecar(ms *mockSidecar) {
	ms.GetVitalSignsFunc = func(_ context.Context) (*pb.GetVitalSignsResponse, error) {
		return &pb.GetVitalSignsResponse{Health: 20, Food: 18}, nil
	}
	ms.GetSurroundingsFunc = func(_ context.Context, _ int32, _ bool) (*pb.GetSurroundingsResponse, error) {
		return &pb.GetSurroundingsResponse{Biome: "plains", TimeOfDay: "noon"}, nil
	}
	ms.GetInventoryFunc = func(_ context.Context, _ bool) (*pb.GetInventoryResponse, error) {
		return &pb.GetInventoryResponse{EmptySlots: 30}, nil
	}
}

func TestAssemblePerceptionBasic(t *testing.T) {
	a, ms, _, _ := newTestAgent(t)
	stubSidecar(ms)
	a.appendEvent(&pb.GameEvent{Description: "weather changed"})

	snap := a.assemblePerception(context.Background(), WakeSignal{Reason: WakeHeartbeat})
	if snap.Vitals == nil {
		t.Error("expected vitals")
	}
	if len(snap.Events) != 1 {
		t.Errorf("expected 1 event, got %d", len(snap.Events))
	}
}

func TestAssemblePerceptionGatekeeperSnap(t *testing.T) {
	a, ms, _, _ := newTestAgent(t)
	stubSidecar(ms)

	preEvents := []*pb.GameEvent{{Description: "pre-classified"}}
	preClassified := []perception.ClassifiedEvent{{Reason: "important"}}

	sig := WakeSignal{
		Reason: WakeGatekeeper,
		GatekeeperSnap: &GatekeeperResult{
			Events:           preEvents,
			ClassifiedEvents: preClassified,
		},
	}
	snap := a.assemblePerception(context.Background(), sig)
	if len(snap.Events) != 1 || snap.Events[0].Description != "pre-classified" {
		t.Error("expected gatekeeper pre-fetched events")
	}
	if len(snap.ClassifiedEvents) != 1 {
		t.Error("expected gatekeeper pre-classified events")
	}
}

func TestAssemblePerceptionChatMessages(t *testing.T) {
	a, ms, _, _ := newTestAgent(t)
	stubSidecar(ms)
	a.appendChat("hello there", "alice")
	a.appendChat("hi", "bob")

	snap := a.assemblePerception(context.Background(), WakeSignal{Reason: WakeBypassChat})
	if len(snap.ChatMessages) != 2 {
		t.Fatalf("expected 2 chat messages, got %d", len(snap.ChatMessages))
	}
	if snap.ChatMessages[0].Sender != "alice" {
		t.Errorf("first sender = %q", snap.ChatMessages[0].Sender)
	}
}

func TestAssemblePerceptionHeartbeat(t *testing.T) {
	a, ms, _, _ := newTestAgent(t)
	stubSidecar(ms)
	a.lastWakeAt.Store(time.Now().Add(-30 * time.Second).UnixNano())

	snap := a.assemblePerception(context.Background(), WakeSignal{Reason: WakeHeartbeat})
	if snap.HeartbeatNote == "" {
		t.Error("expected HeartbeatNote for WakeHeartbeat")
	}
	if !strings.Contains(snap.HeartbeatNote, "Heartbeat") {
		t.Errorf("HeartbeatNote = %q", snap.HeartbeatNote)
	}
}

func TestReadGoalsSummary(t *testing.T) {
	t.Run("default_seed", func(t *testing.T) {
		a, _, _, _ := newTestAgent(t)
		got := a.readGoalsSummary()
		if got != "# Goals" {
			t.Errorf("expected seed header, got %q", got)
		}
	})

	t.Run("single_line", func(t *testing.T) {
		a, _, _, _ := newTestAgent(t)
		_ = a.memory.UpdateGoals("Survive the first night")
		got := a.readGoalsSummary()
		if got != "Survive the first night" {
			t.Errorf("got %q", got)
		}
	})

	t.Run("multi_line", func(t *testing.T) {
		a, _, _, _ := newTestAgent(t)
		_ = a.memory.UpdateGoals("Primary: build shelter\nSecondary: gather wood")
		got := a.readGoalsSummary()
		if got != "Primary: build shelter" {
			t.Errorf("expected first line, got %q", got)
		}
	})
}

func TestBuildGatekeeperSnapshotWithSurr(t *testing.T) {
	a, _, _, _ := newTestAgent(t)
	vitals := &pb.GetVitalSignsResponse{Health: 20}
	surr := &pb.GetSurroundingsResponse{
		Biome:     "forest",
		TimeOfDay: "dusk",
		Position:  &pb.Vec3{X: 10, Y: 64, Z: 20},
	}
	events := []*pb.GameEvent{{Description: "test"}}

	snap := a.buildGatekeeperSnapshot(vitals, surr, events)
	if snap.Biome != "forest" {
		t.Errorf("biome = %q", snap.Biome)
	}
	if snap.TimeOfDay != "dusk" {
		t.Errorf("timeOfDay = %q", snap.TimeOfDay)
	}
	if snap.Position == nil {
		t.Error("expected position")
	}
	if len(snap.Events) != 1 {
		t.Errorf("events = %d", len(snap.Events))
	}
}

func TestBuildGatekeeperSnapshotNilSurr(t *testing.T) {
	a, _, _, _ := newTestAgent(t)
	vitals := &pb.GetVitalSignsResponse{Health: 20}

	snap := a.buildGatekeeperSnapshot(vitals, nil, nil)
	if snap.Biome != "" {
		t.Errorf("expected empty biome, got %q", snap.Biome)
	}
	if snap.Position != nil {
		t.Error("expected nil position")
	}
}

// --- Event handling tests ---

func TestHandleEventNil(t *testing.T) {
	a, _, _, _ := newTestAgent(t)
	a.handleEvent(context.Background(), nil)
}

func TestHandleEventCritical(t *testing.T) {
	a, _, _, _ := newTestAgent(t)
	a.flight.enter("move_to", func(error) {})
	defer a.flight.exit()

	evt := &pb.GameEvent{
		Type:        pb.EventType_EVENT_ENTITY_HURT,
		Priority:    pb.EventPriority_EVENT_PRIORITY_CRITICAL,
		Description: "bot died",
	}
	a.handleEvent(context.Background(), evt)

	events := a.drainEvents()
	if len(events) != 1 {
		t.Fatalf("expected 1 event appended, got %d", len(events))
	}

	select {
	case sig := <-a.wakeChan:
		if sig.Reason != WakeBypassCritical {
			t.Errorf("reason = %v, want WakeBypassCritical", sig.Reason)
		}
	default:
		t.Fatal("expected wake signal")
	}
}

func TestHandleEventTaskTerminal(t *testing.T) {
	a, _, _, _ := newTestAgent(t)
	evt := &pb.GameEvent{
		Type:        pb.EventType_EVENT_TASK_PROGRESS,
		Description: "Task completed: harvest_block",
	}
	a.handleEvent(context.Background(), evt)

	select {
	case sig := <-a.wakeChan:
		if sig.Reason != WakeBypassTask {
			t.Errorf("reason = %v, want WakeBypassTask", sig.Reason)
		}
	default:
		t.Fatal("expected wake signal for terminal task")
	}
}

func TestHandleEventNormal(t *testing.T) {
	a, _, _, _ := newTestAgent(t)
	evt := &pb.GameEvent{
		Type:        pb.EventType_EVENT_WEATHER_CHANGE,
		Priority:    pb.EventPriority_EVENT_PRIORITY_LOW,
		Description: "rain started",
	}
	a.handleEvent(context.Background(), evt)

	events := a.drainEvents()
	if len(events) != 1 {
		t.Fatalf("expected event appended, got %d", len(events))
	}

	select {
	case <-a.wakeChan:
		t.Fatal("normal event should not send wake")
	default:
	}
}

func TestHandleChatEventEmergency(t *testing.T) {
	a, ms, _, _ := newTestAgent(t)
	ms.SendChatFunc = func(_ context.Context, _ string) error { return nil }

	evt := &pb.GameEvent{
		Type:        pb.EventType_EVENT_CHAT_MESSAGE,
		PlayerName:  "alice",
		ChatMessage: "/golem stop",
	}
	a.handleChatEvent(context.Background(), evt)

	select {
	case sig := <-a.wakeChan:
		if sig.Reason != WakeBypassCritical {
			t.Errorf("reason = %v, want WakeBypassCritical", sig.Reason)
		}
	default:
		t.Fatal("expected wake signal for emergency stop")
	}
}

func TestHandleChatEventRegular(t *testing.T) {
	a, _, _, _ := newTestAgent(t)
	evt := &pb.GameEvent{
		Type:        pb.EventType_EVENT_CHAT_MESSAGE,
		PlayerName:  "bob",
		ChatMessage: "come help me",
	}
	a.handleChatEvent(context.Background(), evt)

	chats := a.drainChat()
	if len(chats) != 1 || chats[0].Message != "come help me" {
		t.Errorf("expected chat buffered, got %v", chats)
	}

	events := a.drainEvents()
	if len(events) != 1 {
		t.Errorf("expected event appended, got %d", len(events))
	}

	select {
	case sig := <-a.wakeChan:
		if sig.Reason != WakeBypassChat {
			t.Errorf("reason = %v, want WakeBypassChat", sig.Reason)
		}
	default:
		t.Fatal("expected wake signal for chat")
	}
}

// --- Tool execution tests ---

func TestExecuteToolNormal(t *testing.T) {
	a, _, _, md := newTestAgent(t)
	md.ExecuteResultFunc = func(_ context.Context, _ string, _ json.RawMessage) (game.Result, error) {
		return game.Result{Text: "Moved to (10, 64, 20)"}, nil
	}

	use := claude.ToolUse{ID: "t1", Name: "move_to", Input: json.RawMessage(`{}`)}
	result, isError := a.executeTool(context.Background(), use)
	if isError {
		t.Error("expected isError=false")
	}
	if result.Text != "Moved to (10, 64, 20)" {
		t.Errorf("text = %q", result.Text)
	}
}

func TestExecuteToolTransportError(t *testing.T) {
	a, _, _, md := newTestAgent(t)
	md.ExecuteResultFunc = func(_ context.Context, _ string, _ json.RawMessage) (game.Result, error) {
		return game.Result{}, errors.New("rpc connection lost")
	}

	use := claude.ToolUse{ID: "t1", Name: "dig_block", Input: json.RawMessage(`{}`)}
	result, isError := a.executeTool(context.Background(), use)
	if !isError {
		t.Error("expected isError=true for transport error")
	}
	if !strings.Contains(result.Text, "transport error") {
		t.Errorf("text = %q, expected transport error", result.Text)
	}
}

func TestExecuteToolThinkDeeply(t *testing.T) {
	a, ms, ma, _ := newTestAgent(t)
	ms.GetVitalSignsFunc = func(_ context.Context) (*pb.GetVitalSignsResponse, error) {
		return &pb.GetVitalSignsResponse{Health: 20}, nil
	}
	ms.GetSurroundingsFunc = func(_ context.Context, _ int32, _ bool) (*pb.GetSurroundingsResponse, error) {
		return &pb.GetSurroundingsResponse{Biome: "plains"}, nil
	}
	ma.ThinkDeeplyFunc = func(_ context.Context, _, _ string) (*claude.Response, error) {
		return &claude.Response{Text: "Strategic advice: build shelter first"}, nil
	}

	use := claude.ToolUse{
		ID:    "t1",
		Name:  claude.ToolThinkDeeply,
		Input: json.RawMessage(`{"situation":"what should I do?"}`),
	}
	result, isError := a.executeTool(context.Background(), use)
	if isError {
		t.Error("expected isError=false")
	}
	if !strings.Contains(result.Text, "build shelter") {
		t.Errorf("text = %q", result.Text)
	}
}

func TestExecuteToolInterrupted(t *testing.T) {
	a, _, _, md := newTestAgent(t)
	md.ExecuteResultFunc = func(ctx context.Context, _ string, _ json.RawMessage) (game.Result, error) {
		<-ctx.Done()
		return game.Result{}, ctx.Err()
	}

	use := claude.ToolUse{ID: "t1", Name: "navigate_to", Input: json.RawMessage(`{}`)}

	go func() {
		time.Sleep(10 * time.Millisecond)
		a.flight.interrupt(CancelEmergencyStop, "player stop")
	}()

	result, isError := a.executeTool(context.Background(), use)
	if isError {
		t.Error("interrupted tools should not set isError")
	}
	if !strings.Contains(result.Text, "interrupted") {
		t.Errorf("text = %q, expected interrupted message", result.Text)
	}
}

func TestExecuteToolUses(t *testing.T) {
	a, _, _, md := newTestAgent(t)
	callCount := 0
	md.ExecuteResultFunc = func(_ context.Context, name string, _ json.RawMessage) (game.Result, error) {
		callCount++
		return game.Result{Text: fmt.Sprintf("%s done", name)}, nil
	}

	uses := []claude.ToolUse{
		{ID: "t1", Name: "move_to", Input: json.RawMessage(`{}`)},
		{ID: "t2", Name: "dig_block", Input: json.RawMessage(`{}`)},
	}

	blocks, toolMS := a.executeToolUses(context.Background(), uses)
	if callCount != 2 {
		t.Errorf("expected 2 calls, got %d", callCount)
	}
	if toolMS < 0 {
		t.Errorf("toolMS = %d", toolMS)
	}
	resultBlocks := 0
	for _, b := range blocks {
		if b.Type == claude.BlockToolResult {
			resultBlocks++
		}
	}
	if resultBlocks != 2 {
		t.Errorf("expected 2 result blocks, got %d", resultBlocks)
	}
}

func TestLogAndPublishTurn(t *testing.T) {
	a, _, _, _ := newTestAgent(t)
	resp := &claude.Response{
		Text:       "I'll move north",
		StopReason: "end_turn",
		Model:      "claude-sonnet-4-6",
		Usage:      claude.Usage{InputTokens: 100, OutputTokens: 50},
	}
	a.logAndPublishTurn(1, 0, resp, 200, 100)
}

// --- Perception tick tests ---

func TestPerceptionTickSkipsFlight(t *testing.T) {
	a, ms, _, _ := newTestAgent(t)
	called := false
	ms.GetVitalSignsFunc = func(_ context.Context) (*pb.GetVitalSignsResponse, error) {
		called = true
		return &pb.GetVitalSignsResponse{}, nil
	}

	a.flight.enter("move_to", func(error) {})
	defer a.flight.exit()

	a.perceptionTick(context.Background())
	if called {
		t.Error("sidecar should not be called when tool is in flight")
	}
}

func TestPerceptionTickThrottles(t *testing.T) {
	a, ms, ma, _ := newTestAgent(t)
	a.lastWakeAt.Store(time.Now().UnixNano())

	ms.GetVitalSignsFunc = func(_ context.Context) (*pb.GetVitalSignsResponse, error) {
		return &pb.GetVitalSignsResponse{Health: 20, Food: 20}, nil
	}
	ms.GetSurroundingsFunc = func(_ context.Context, _ int32, _ bool) (*pb.GetSurroundingsResponse, error) {
		return &pb.GetSurroundingsResponse{}, nil
	}

	a.appendEvent(&pb.GameEvent{
		Type:     pb.EventType_EVENT_WEATHER_CHANGE,
		Priority: pb.EventPriority_EVENT_PRIORITY_LOW,
	})

	gkCalled := false
	ma.GatekeeperCheckFunc = func(_ context.Context, _ claude.GatekeeperSnapshot) (*claude.GatekeeperDecision, error) {
		gkCalled = true
		return &claude.GatekeeperDecision{}, nil
	}

	a.perceptionTick(context.Background())
	if gkCalled {
		t.Error("gatekeeper should be throttled for routine events with healthy vitals")
	}
}

func TestPerceptionTickWakes(t *testing.T) {
	a, ms, ma, _ := newTestAgent(t)
	a.lastWakeAt.Store(time.Now().Add(-time.Minute).UnixNano())

	ms.GetVitalSignsFunc = func(_ context.Context) (*pb.GetVitalSignsResponse, error) {
		return &pb.GetVitalSignsResponse{Health: 20, Food: 20}, nil
	}
	ms.GetSurroundingsFunc = func(_ context.Context, _ int32, _ bool) (*pb.GetSurroundingsResponse, error) {
		return &pb.GetSurroundingsResponse{Biome: "plains"}, nil
	}

	ma.GatekeeperCheckFunc = func(_ context.Context, _ claude.GatekeeperSnapshot) (*claude.GatekeeperDecision, error) {
		return &claude.GatekeeperDecision{Wake: true, Reason: "health is low"}, nil
	}

	a.perceptionTick(context.Background())

	select {
	case sig := <-a.wakeChan:
		if sig.Reason != WakeGatekeeper {
			t.Errorf("reason = %v, want WakeGatekeeper", sig.Reason)
		}
		if sig.GatekeeperSnap == nil {
			t.Error("expected GatekeeperSnap")
		}
	default:
		t.Fatal("expected wake signal")
	}
}

func TestPerceptionTickIdle(t *testing.T) {
	a, ms, ma, _ := newTestAgent(t)
	a.lastWakeAt.Store(time.Now().Add(-time.Minute).UnixNano())

	ms.GetVitalSignsFunc = func(_ context.Context) (*pb.GetVitalSignsResponse, error) {
		return &pb.GetVitalSignsResponse{Health: 20, Food: 20}, nil
	}
	ms.GetSurroundingsFunc = func(_ context.Context, _ int32, _ bool) (*pb.GetSurroundingsResponse, error) {
		return &pb.GetSurroundingsResponse{}, nil
	}

	ma.GatekeeperCheckFunc = func(_ context.Context, _ claude.GatekeeperSnapshot) (*claude.GatekeeperDecision, error) {
		return &claude.GatekeeperDecision{Wake: false, Reason: "all quiet"}, nil
	}

	a.perceptionTick(context.Background())

	select {
	case <-a.wakeChan:
		t.Fatal("should not wake when gatekeeper says idle")
	default:
	}
}
