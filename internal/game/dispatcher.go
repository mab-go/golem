package game

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mab-go/golem/internal/claude"
	sidecar "github.com/mab-go/golem/internal/grpc"
	"github.com/mab-go/golem/internal/logging"
	"github.com/mab-go/golem/internal/memory"
	"github.com/mab-go/golem/internal/perception"
	"github.com/mab-go/golem/internal/publisher"
	"github.com/mab-go/golem/internal/task"
)

// DefaultToolTimeout is the per-tool context deadline.
const DefaultToolTimeout = 60 * time.Second

const (
	smeltBaseTimeout = 30 * time.Second
	smeltPerItem     = 15 * time.Second
	maxToolTimeout   = 10 * time.Minute
)

// longRunningTools is the set of Tier 2 background tasks. Used by
// IsLongRunning for classification (e.g. deciding whether a chat interrupt
// should preempt an in-flight tool).
var longRunningTools = map[string]struct{}{
	claude.ToolHarvestBlock:      {},
	claude.ToolGather:            {},
	claude.ToolBuildStructure:    {},
	claude.ToolProcessAll:        {},
	claude.ToolOrganizeInventory: {},
	claude.ToolClearArea:         {},
	claude.ToolFarm:              {},
}

// conflictingTools cannot run while a background Tier 2 task is active.
// Physical actions conflict with the bot's body; Tier 2 tools are included
// for a cleaner error than the TaskManager's internal rejection.
var conflictingTools = map[string]struct{}{
	claude.ToolMoveTo:                {},
	claude.ToolNavigateTo:            {},
	claude.ToolDigBlock:              {},
	claude.ToolPlaceBlock:            {},
	claude.ToolAttackEntity:          {},
	claude.ToolHarvestBlock:          {},
	claude.ToolInteractWithEntity:    {},
	claude.ToolWithdrawFromContainer: {},
	claude.ToolDepositToContainer:    {},
	claude.ToolGather:                {},
	claude.ToolBuildStructure:        {},
	claude.ToolProcessAll:            {},
	claude.ToolOrganizeInventory:     {},
	claude.ToolClearArea:             {},
	claude.ToolFarm:                  {},
}

// State is the subset of agent state that tool handlers can mutate. It is
// defined as an interface in this package so the dispatcher does not depend
// on the agent package, keeping the import graph acyclic.
type State interface {
	// SetVerbosity updates the agent's verbosity mode. Implementations should
	// propagate the change to the perception formatter and the pacing state.
	SetVerbosity(v perception.VerbosityMode)
}

// Dispatcher routes a tool_use block to a gRPC call or a local handler and
// returns a natural-language result for Claude to read as a tool_result.
//
// The standard pattern: only transport-level failures return a non-nil error.
// Game-logic failures (e.g. "no iron_ore within 16 blocks") are returned as
// the resultText so Claude can reason about what to do differently.
type Dispatcher struct {
	client      *sidecar.Client
	memory      *memory.Manager
	state       State
	taskMgr     *task.Manager
	pub         publisher.EventPublisher
	log         logging.Logger
	botUsername string

	handlers      map[string]handlerFunc
	imageHandlers map[string]imageHandlerFunc
}

// Result is the return type of Execute. Text carries the natural-language
// summary; ImagePNG is populated only by handlers that produce an image
// (currently just handleTakeScreenshot). Callers should forward ImagePNG to
// the Anthropic tool_result as an image content block.
type Result struct {
	Text     string
	ImagePNG []byte
}

type handlerFunc func(ctx context.Context, input json.RawMessage) (string, error)
type imageHandlerFunc func(ctx context.Context, input json.RawMessage) (Result, error)

// NewDispatcher constructs a Dispatcher wired to the sidecar client, memory
// manager, and agent state. The handler map is built during construction.
func NewDispatcher(botUsername string, client *sidecar.Client, mem *memory.Manager, state State, taskMgr *task.Manager, pub publisher.EventPublisher, log logging.Logger) *Dispatcher {
	if pub == nil {
		pub = publisher.Nop()
	}
	if log == nil {
		log = logging.NewDefaultLogger()
	}
	d := &Dispatcher{client: client, memory: mem, state: state, taskMgr: taskMgr, pub: pub, log: log, botUsername: botUsername}
	d.handlers = map[string]handlerFunc{
		// Tier 0
		claude.ToolMoveTo:       d.handleMoveTo,
		claude.ToolLookAt:       d.handleLookAt,
		claude.ToolPlaceBlock:   d.handlePlaceBlock,
		claude.ToolDigBlock:     d.handleDigBlock,
		claude.ToolEquipItem:    d.handleEquipItem,
		claude.ToolUseItem:      d.handleUseItem,
		claude.ToolAttackEntity: d.handleAttackEntity,
		claude.ToolJump:         d.handleJump,
		claude.ToolSetSneak:     d.handleSetSneak,
		claude.ToolSendChat:     d.handleSendChat,

		// Tier 1
		claude.ToolNavigateTo:            d.handleNavigateTo,
		claude.ToolInteractWithEntity:    d.handleInteractWithEntity,
		claude.ToolOpenContainer:         d.handleOpenContainer,
		claude.ToolWithdrawFromContainer: d.handleWithdrawFromContainer,
		claude.ToolDepositToContainer:    d.handleDepositToContainer,
		claude.ToolCraftItem:             d.handleCraftItem,
		claude.ToolSmeltItem:             d.handleSmeltItem,
		claude.ToolEat:                   d.handleEat,

		// Tier 2 (background tasks)
		claude.ToolHarvestBlock:      d.handleHarvestBlock,
		claude.ToolGather:            d.handleGather,
		claude.ToolBuildStructure:    d.handleBuildStructure,
		claude.ToolProcessAll:        d.handleProcessAll,
		claude.ToolOrganizeInventory: d.handleOrganizeInventory,
		claude.ToolClearArea:         d.handleClearArea,
		claude.ToolFarm:              d.handleFarm,
		claude.ToolCancelTask:        d.handleCancelTask,

		// Tier 3
		claude.ToolSurveyArea:    d.handleSurveyArea,
		claude.ToolFindNearest:   d.handleFindNearest,
		claude.ToolWhatCanICraft: d.handleWhatCanICraft,
		claude.ToolAssessThreat:  d.handleAssessThreat,
		claude.ToolPlanPath:      d.handlePlanPath,

		// Perception
		claude.ToolLookAround:     d.handleLookAround,
		claude.ToolCheckInventory: d.handleCheckInventory,
		// Perception -- take_screenshot is an image handler; see below.

		// Meta
		claude.ToolSetVerbosity:         d.handleSetVerbosity,
		claude.ToolWriteJournal:         d.handleWriteJournal,
		claude.ToolUpdateGoals:          d.handleUpdateGoals,
		claude.ToolUpdateWorldKnowledge: d.handleUpdateWorldKnowledge,
		claude.ToolUpdateInventoryNotes: d.handleUpdateInventoryNotes,
		claude.ToolUpdateSocial:         d.handleUpdateSocial,
	}
	d.imageHandlers = map[string]imageHandlerFunc{
		claude.ToolTakeScreenshot: d.handleTakeScreenshot,
	}
	return d
}

// Execute runs the handler for the named tool, unmarshaling the input and
// returning a natural-language result string. Deprecated for image-producing
// tools -- callers that may receive an image should use ExecuteResult instead.
func (d *Dispatcher) Execute(ctx context.Context, toolName string, input json.RawMessage) (string, error) {
	res, err := d.ExecuteResult(ctx, toolName, input)
	return res.Text, err
}

// ExecuteResult runs the handler for the named tool and returns a Result
// that may carry both a text summary and an image payload. Text-only tools
// return Result{Text: ...}; take_screenshot populates ImagePNG as well.
func (d *Dispatcher) ExecuteResult(ctx context.Context, toolName string, input json.RawMessage) (Result, error) {
	if input == nil {
		input = json.RawMessage(`{}`)
	}
	if d.taskMgr != nil && d.taskMgr.IsActive() {
		if _, conflict := conflictingTools[toolName]; conflict {
			return Result{Text: fmt.Sprintf("Cannot execute %s -- %s is currently running. Cancel the active task first (cancel_task) or wait for it to complete.", toolName, d.taskMgr.ActiveName())}, nil
		}
	}
	if h, ok := d.imageHandlers[toolName]; ok {
		return h(ctx, input)
	}
	if h, ok := d.handlers[toolName]; ok {
		text, err := h(ctx, input)
		return Result{Text: text}, err
	}
	return Result{}, fmt.Errorf("unknown tool: %s (available: %s)", toolName, strings.Join(d.known(), ", "))
}

// Timeout returns the per-tool context deadline the agent loop should apply.
// Tools whose duration scales with a count parameter (smelt_item) get an
// adaptive deadline; everything else gets the default.
func (d *Dispatcher) Timeout(toolName string, input json.RawMessage) time.Duration {
	if toolName == claude.ToolSmeltItem {
		var p struct {
			Count int32 `json:"count"`
		}
		if json.Unmarshal(input, &p) == nil && p.Count > 1 {
			return scaledTimeout(smeltBaseTimeout, smeltPerItem, p.Count)
		}
	}
	return DefaultToolTimeout
}

// IsLongRunning reports whether the named tool belongs to the Tier 2 streaming
// set. Used by the agent loop to decide whether an interrupt should preempt
// the in-flight tool: only Tier 2 tasks are worth cancelling -- shorter tools
// complete in seconds and preempting them risks corrupting in-progress actions.
func IsLongRunning(toolName string) bool {
	_, ok := longRunningTools[toolName]
	return ok
}

func (d *Dispatcher) known() []string {
	names := make([]string, 0, len(d.handlers)+len(d.imageHandlers))
	for n := range d.handlers {
		names = append(names, n)
	}
	for n := range d.imageHandlers {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// decode wraps json.Unmarshal with a consistent error message so all handlers
// report parse failures the same way.
func decode(input json.RawMessage, v any) error {
	if err := json.Unmarshal(input, v); err != nil {
		return fmt.Errorf("invalid tool input: %w", err)
	}
	return nil
}

func scaledTimeout(base, perUnit time.Duration, count int32) time.Duration {
	if t := base + time.Duration(count)*perUnit; t <= maxToolTimeout {
		return t
	}
	return maxToolTimeout
}

// formatFailure converts an ActionResult failure into a natural-language
// result string Claude can reason about.
func formatFailure(kind, detail string) string {
	if detail == "" {
		return fmt.Sprintf("%s failed.", kind)
	}
	return fmt.Sprintf("%s failed: %s", kind, detail)
}
