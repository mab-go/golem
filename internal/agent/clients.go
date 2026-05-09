package agent

import (
	"context"
	"encoding/json"
	"time"

	"github.com/mab-go/golem/internal/claude"
	"github.com/mab-go/golem/internal/game"
	"github.com/mab-go/golem/internal/grpc/pb"
	"github.com/mab-go/golem/internal/perception"
)

// SidecarClient is the subset of sidecar.Client that the agent calls
// directly for perception, event subscription, and chat.
type SidecarClient interface {
	GetVitalSigns(ctx context.Context) (*pb.GetVitalSignsResponse, error)
	GetSurroundings(ctx context.Context, radius int32, fullUpdate bool) (*pb.GetSurroundingsResponse, error)
	GetInventory(ctx context.Context, includeCraftSuggestions bool) (*pb.GetInventoryResponse, error)
	SubscribeEvents(ctx context.Context, filterTypes ...pb.EventType) (pb.MinecraftService_SubscribeEventsClient, error)
	SendChat(ctx context.Context, message string) error
}

// AIClient is the subset of claude.Client that the agent calls for
// API requests, event classification, gatekeeper checks, and deep
// thinking.
type AIClient interface {
	SendMessageParts(ctx context.Context, model string, parts claude.CacheableSystemPrompt, messages []claude.Message, tools []claude.Tool, opts ...claude.CallOption) (*claude.Response, error)
	ClassifyEvents(ctx context.Context, events []*pb.GameEvent) ([]perception.ClassifiedEvent, error)
	GatekeeperCheck(ctx context.Context, snap claude.GatekeeperSnapshot) (*claude.GatekeeperDecision, error)
	ThinkDeeply(ctx context.Context, situation, contextSummary string) (*claude.Response, error)
}

// ToolDispatcher is the subset of game.Dispatcher that the agent loop
// calls to execute tool_use blocks and determine per-tool timeouts.
type ToolDispatcher interface {
	ExecuteResult(ctx context.Context, toolName string, input json.RawMessage) (game.Result, error)
	Timeout(toolName string, input json.RawMessage) time.Duration
}
