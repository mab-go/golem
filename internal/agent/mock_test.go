package agent

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/mab-go/golem/internal/claude"
	"github.com/mab-go/golem/internal/game"
	"github.com/mab-go/golem/internal/grpc/pb"
	"github.com/mab-go/golem/internal/logging"
	"github.com/mab-go/golem/internal/memory"
	"github.com/mab-go/golem/internal/perception"
	"github.com/mab-go/golem/internal/publisher"
	"github.com/mab-go/golem/internal/task"
)

var _ SidecarClient = (*mockSidecar)(nil)
var _ AIClient = (*mockAI)(nil)
var _ ToolDispatcher = (*mockDispatcher)(nil)

type mockSidecar struct {
	GetVitalSignsFunc   func(ctx context.Context) (*pb.GetVitalSignsResponse, error)
	GetSurroundingsFunc func(ctx context.Context, radius int32, fullUpdate bool) (*pb.GetSurroundingsResponse, error)
	GetInventoryFunc    func(ctx context.Context, includeCraftSuggestions bool) (*pb.GetInventoryResponse, error)
	SubscribeEventsFunc func(ctx context.Context, filterTypes ...pb.EventType) (pb.MinecraftService_SubscribeEventsClient, error)
	SendChatFunc        func(ctx context.Context, message string) error
}

func (m *mockSidecar) GetVitalSigns(ctx context.Context) (*pb.GetVitalSignsResponse, error) {
	return m.GetVitalSignsFunc(ctx)
}

func (m *mockSidecar) GetSurroundings(ctx context.Context, radius int32, fullUpdate bool) (*pb.GetSurroundingsResponse, error) {
	return m.GetSurroundingsFunc(ctx, radius, fullUpdate)
}

func (m *mockSidecar) GetInventory(ctx context.Context, includeCraftSuggestions bool) (*pb.GetInventoryResponse, error) {
	return m.GetInventoryFunc(ctx, includeCraftSuggestions)
}

func (m *mockSidecar) SubscribeEvents(ctx context.Context, filterTypes ...pb.EventType) (pb.MinecraftService_SubscribeEventsClient, error) {
	return m.SubscribeEventsFunc(ctx, filterTypes...)
}

func (m *mockSidecar) SendChat(ctx context.Context, message string) error {
	return m.SendChatFunc(ctx, message)
}

type mockAI struct {
	SendMessagePartsFunc func(ctx context.Context, model string, parts claude.CacheableSystemPrompt, messages []claude.Message, tools []claude.Tool, opts ...claude.CallOption) (*claude.Response, error)
	ClassifyEventsFunc   func(ctx context.Context, events []*pb.GameEvent) ([]perception.ClassifiedEvent, error)
	GatekeeperCheckFunc  func(ctx context.Context, snap claude.GatekeeperSnapshot) (*claude.GatekeeperDecision, error)
	ThinkDeeplyFunc      func(ctx context.Context, situation, contextSummary string) (*claude.Response, error)
}

func (m *mockAI) SendMessageParts(ctx context.Context, model string, parts claude.CacheableSystemPrompt, messages []claude.Message, tools []claude.Tool, opts ...claude.CallOption) (*claude.Response, error) {
	return m.SendMessagePartsFunc(ctx, model, parts, messages, tools, opts...)
}

func (m *mockAI) ClassifyEvents(ctx context.Context, events []*pb.GameEvent) ([]perception.ClassifiedEvent, error) {
	return m.ClassifyEventsFunc(ctx, events)
}

func (m *mockAI) GatekeeperCheck(ctx context.Context, snap claude.GatekeeperSnapshot) (*claude.GatekeeperDecision, error) {
	return m.GatekeeperCheckFunc(ctx, snap)
}

func (m *mockAI) ThinkDeeply(ctx context.Context, situation, contextSummary string) (*claude.Response, error) {
	return m.ThinkDeeplyFunc(ctx, situation, contextSummary)
}

type mockDispatcher struct {
	ExecuteResultFunc func(ctx context.Context, toolName string, input json.RawMessage) (game.Result, error)
	TimeoutFunc       func(toolName string, input json.RawMessage) time.Duration
}

func (m *mockDispatcher) ExecuteResult(ctx context.Context, toolName string, input json.RawMessage) (game.Result, error) {
	return m.ExecuteResultFunc(ctx, toolName, input)
}

func (m *mockDispatcher) Timeout(toolName string, input json.RawMessage) time.Duration {
	if m.TimeoutFunc != nil {
		return m.TimeoutFunc(toolName, input)
	}
	return 60 * time.Second
}

func newTestAgent(t *testing.T) (*Agent, *mockSidecar, *mockAI, *mockDispatcher) {
	t.Helper()
	mem, err := memory.New(t.TempDir())
	if err != nil {
		t.Fatalf("memory.New: %v", err)
	}
	ms := &mockSidecar{}
	ma := &mockAI{}
	md := &mockDispatcher{}
	formatter := perception.NewFormatter(perception.FormatProse, perception.VerbosityStandard)
	pacing := NewPacingState(perception.VerbosityStandard)
	history := claude.NewHistory(10)
	log := logging.NewDefaultLogger()
	ctx := context.Background()
	taskMgr := task.NewManager(ctx, func(*pb.GameEvent) {}, func() {}, 10*time.Minute, log)
	a := &Agent{
		cfg: Config{
			BotUsername:            "testbot",
			PerceptionRadius:       16,
			HistoryMessages:        10,
			MaxToolRounds:          3,
			TaskTimeout:            10 * time.Minute,
			PerceptionTickInterval: 3 * time.Second,
			HeartbeatInterval:      45 * time.Second,
			GatekeeperTimeout:      5 * time.Second,
		},
		client:     ms,
		ai:         ma,
		memory:     mem,
		formatter:  formatter,
		pacing:     pacing,
		history:    history,
		builder:    claude.NewContextBuilder(mem, formatter, history),
		tools:      claude.Tools(),
		gkHealth:   newGatekeeperHealth(5*time.Minute, 0.05, log),
		log:        log,
		pub:        publisher.Nop(),
		wakeChan:   make(chan WakeSignal, 1),
		flight:     &toolFlight{},
		dispatcher: md,
		taskMgr:    taskMgr,
	}
	a.chatHandler = &ChatInterceptor{
		client:      ms,
		pacing:      pacing,
		memory:      mem,
		history:     history,
		botUsername: "testbot",
		shutdown:    func() {},
		log:         log,
	}
	return a, ms, ma, md
}
