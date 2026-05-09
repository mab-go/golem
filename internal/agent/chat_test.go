package agent

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/mab-go/golem/internal/claude"
	"github.com/mab-go/golem/internal/grpc/pb"
	"github.com/mab-go/golem/internal/logging"
	"github.com/mab-go/golem/internal/memory"
	"github.com/mab-go/golem/internal/perception"
)

func newTestInterceptor(t *testing.T) *ChatInterceptor {
	t.Helper()
	mem, err := memory.New(t.TempDir())
	if err != nil {
		t.Fatalf("memory.New: %v", err)
	}
	return &ChatInterceptor{
		pacing:      NewPacingState(perception.VerbosityStandard),
		memory:      mem,
		history:     claude.NewHistory(10),
		botUsername: "claude",
		shutdown:    func() {},
		log:         logging.NewDefaultLogger(),
	}
}

func TestHandleCommandStop(t *testing.T) {
	c := newTestInterceptor(t)
	reply, isStop := c.handleCommand(context.Background(), "stop")
	if !isStop {
		t.Fatal("expected isStop=true for stop command")
	}
	if reply != "stopping current action" {
		t.Errorf("reply = %q, want %q", reply, "stopping current action")
	}
}

func TestHandleCommandsAreNotStop(t *testing.T) {
	c := newTestInterceptor(t)
	cmds := []string{"pause", "resume", "help", "", "verbose", "standard", "terse", "status", "shutdown"}
	for _, cmd := range cmds {
		_, isStop := c.handleCommand(context.Background(), cmd)
		if isStop {
			t.Errorf("command %q returned isStop=true, want false", cmd)
		}
	}
}

func TestHandleCommandPauseResume(t *testing.T) {
	c := newTestInterceptor(t)
	if c.pacing.Paused() {
		t.Fatal("expected not paused initially")
	}

	c.handleCommand(context.Background(), "pause")
	if !c.pacing.Paused() {
		t.Fatal("expected paused after pause command")
	}

	c.handleCommand(context.Background(), "resume")
	if c.pacing.Paused() {
		t.Fatal("expected unpaused after resume command")
	}
}

func TestHandleCommandVerbosity(t *testing.T) {
	c := newTestInterceptor(t)
	if c.pacing.Verbosity() != perception.VerbosityStandard {
		t.Fatalf("expected standard verbosity initially, got %v", c.pacing.Verbosity())
	}

	c.handleCommand(context.Background(), "verbose")
	if c.pacing.Verbosity() != perception.VerbosityVerbose {
		t.Errorf("expected verbose after command, got %v", c.pacing.Verbosity())
	}

	c.handleCommand(context.Background(), "terse")
	if c.pacing.Verbosity() != perception.VerbosityTerse {
		t.Errorf("expected terse after command, got %v", c.pacing.Verbosity())
	}

	c.handleCommand(context.Background(), "standard")
	if c.pacing.Verbosity() != perception.VerbosityStandard {
		t.Errorf("expected standard after command, got %v", c.pacing.Verbosity())
	}
}

func TestHandleCommandUnknown(t *testing.T) {
	c := newTestInterceptor(t)
	reply, isStop := c.handleCommand(context.Background(), "foobar")
	if isStop {
		t.Fatal("unknown command should not be stop")
	}
	if !strings.Contains(reply, "unknown command") {
		t.Errorf("expected unknown command message, got %q", reply)
	}
}

func TestHandleRoutingRegularChat(t *testing.T) {
	c := newTestInterceptor(t)
	evt := &pb.GameEvent{
		Type:        pb.EventType_EVENT_CHAT_MESSAGE,
		PlayerName:  "alice",
		ChatMessage: "come help me",
	}
	res := c.Handle(context.Background(), evt)
	if res.Handled {
		t.Fatal("regular chat should not be handled")
	}
	if res.HumanMessage != "come help me" {
		t.Errorf("HumanMessage = %q, want %q", res.HumanMessage, "come help me")
	}
	if res.Sender != "alice" {
		t.Errorf("Sender = %q, want %q", res.Sender, "alice")
	}
	if res.EmergencyStop {
		t.Fatal("regular chat should not be emergency stop")
	}
}

func TestHandleRoutingSelfChat(t *testing.T) {
	c := newTestInterceptor(t)
	evt := &pb.GameEvent{
		Type:        pb.EventType_EVENT_CHAT_MESSAGE,
		PlayerName:  "claude",
		ChatMessage: "hello world",
	}
	res := c.Handle(context.Background(), evt)
	if !res.Handled {
		t.Fatal("self-chat should be handled (filtered)")
	}
	if res.HumanMessage != "" {
		t.Errorf("self-chat should have empty HumanMessage, got %q", res.HumanMessage)
	}
}

func TestHandleRoutingEmptyMessage(t *testing.T) {
	c := newTestInterceptor(t)
	evt := &pb.GameEvent{
		Type:        pb.EventType_EVENT_CHAT_MESSAGE,
		PlayerName:  "alice",
		ChatMessage: "   ",
	}
	res := c.Handle(context.Background(), evt)
	if !res.Handled {
		t.Fatal("empty message should be handled (filtered)")
	}
}

func newTestInterceptorWithClient(t *testing.T, ms *mockSidecar) *ChatInterceptor {
	t.Helper()
	mem, err := memory.New(t.TempDir())
	if err != nil {
		t.Fatalf("memory.New: %v", err)
	}
	return &ChatInterceptor{
		client:      ms,
		pacing:      NewPacingState(perception.VerbosityStandard),
		memory:      mem,
		history:     claude.NewHistory(10),
		botUsername: "claude",
		shutdown:    func() {},
		log:         logging.NewDefaultLogger(),
	}
}

func TestHandleCommandSendsChatReply(t *testing.T) {
	var sent string
	ms := &mockSidecar{
		SendChatFunc: func(_ context.Context, msg string) error {
			sent = msg
			return nil
		},
	}
	c := newTestInterceptorWithClient(t, ms)
	evt := &pb.GameEvent{
		Type:        pb.EventType_EVENT_CHAT_MESSAGE,
		PlayerName:  "alice",
		ChatMessage: "/golem pause",
	}
	res := c.Handle(context.Background(), evt)
	if !res.Handled {
		t.Fatal("command should be handled")
	}
	if sent == "" {
		t.Fatal("expected SendChat to be called with reply")
	}
	if !c.pacing.Paused() {
		t.Error("expected paused after /golem pause")
	}
}

func TestHandleCommandSendChatError(t *testing.T) {
	ms := &mockSidecar{
		SendChatFunc: func(_ context.Context, _ string) error {
			return errors.New("connection lost")
		},
	}
	c := newTestInterceptorWithClient(t, ms)
	evt := &pb.GameEvent{
		Type:        pb.EventType_EVENT_CHAT_MESSAGE,
		PlayerName:  "alice",
		ChatMessage: "/golem help",
	}
	res := c.Handle(context.Background(), evt)
	if !res.Handled {
		t.Fatal("command should still be handled even if SendChat fails")
	}
}

func TestHandleStopSendsChatReply(t *testing.T) {
	var sent string
	ms := &mockSidecar{
		SendChatFunc: func(_ context.Context, msg string) error {
			sent = msg
			return nil
		},
	}
	c := newTestInterceptorWithClient(t, ms)
	evt := &pb.GameEvent{
		Type:        pb.EventType_EVENT_CHAT_MESSAGE,
		PlayerName:  "bob",
		ChatMessage: "/golem stop",
	}
	res := c.Handle(context.Background(), evt)
	if !res.Handled {
		t.Fatal("stop command should be handled")
	}
	if !res.EmergencyStop {
		t.Fatal("expected EmergencyStop=true")
	}
	if res.Sender != "bob" {
		t.Errorf("Sender = %q, want %q", res.Sender, "bob")
	}
	if sent == "" {
		t.Fatal("expected SendChat to be called with reply")
	}
}

func TestHandleNilEvent(t *testing.T) {
	c := newTestInterceptor(t)
	res := c.Handle(context.Background(), nil)
	if !res.Handled {
		t.Fatal("nil event should be handled")
	}
}

func TestHandleNonChatEvent(t *testing.T) {
	c := newTestInterceptor(t)
	evt := &pb.GameEvent{
		Type:        pb.EventType_EVENT_WEATHER_CHANGE,
		Description: "it started raining",
	}
	res := c.Handle(context.Background(), evt)
	if !res.Handled {
		t.Fatal("non-chat event should be handled (filtered)")
	}
}
