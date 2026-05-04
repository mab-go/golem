# game

Routes Claude's `tool_use` blocks to gRPC calls or local handlers. The
dispatcher is the bridge between the AI's decisions and the Minecraft world:
it parses tool inputs, calls the sidecar, and returns natural-language results
that Claude can reason about.

## Overview

### Dispatch Pattern

The `Dispatcher` registers two handler maps at construction time:

- `handlers` -- text-returning handlers (`func(ctx, input) (string, error)`)
- `imageHandlers` -- handlers that may return images (for `take_screenshot`)

When `Execute` is called with a tool name and JSON input, it:

1. Checks for Tier 2 task conflicts (can't `dig_block` while `gather`
   is running)
2. Looks up the handler by name
3. Calls the handler, which parses input, calls gRPC, and formats the result
4. Returns the result text (and optional image) to the agent loop

### Error Handling Convention

This is the most important pattern in the package. Game action handlers return
`(resultText string, error)` with a strict split:

| Scenario | Return | Why |
|----------|--------|-----|
| JSON decode failure | `(err.Error(), nil)` | Claude sees the parse error and can fix its input |
| Missing required field | `("tool requires field X", nil)` | Claude can reason about what's missing |
| gRPC transport failure | `("", err)` | Unrecoverable -- aborts the agentic loop turn |
| Game logic failure (block not found, can't craft) | `(formatFailure(...), nil)` | Claude tries an alternative strategy |
| Success | `("detailed result message", nil)` | Always nil error on success |

The rule: **only gRPC transport failures return non-nil error.** Everything else
is a readable message that Claude can act on.

### Tier Organization

| Tier | File | Pattern | Count |
|------|------|---------|-------|
| 0 | tier0.go | Single gRPC call, check `Result.Success` | 10 |
| 1 | tier1.go | Complex input validation, derived output metrics | 9 |
| 2 | tier2.go | Launch streaming task via `launchTask()`, return immediately | 6 |
| 3 | tier3.go | Synchronous query, return structured analysis | 5 |
| Perception | perception.go | Read-only observation, image support | 3 |
| Meta | meta.go | Memory writes, verbosity, task cancellation | 7 |

Tier 2 tools launch background streaming tasks through the task manager and
return immediately with a "task started" message. Progress appears in the
agent's perception stream on subsequent turns.

### Adaptive Timeouts

Most tools get a 60-second deadline. Two tools scale based on input:

- `harvest_block`: 30s base + 30s per block (e.g., 5 blocks = 180s)
- `smelt_item`: 30s base + 15s per item (e.g., 10 items = 180s)

Both are capped at 10 minutes.

### Task Conflict Management

When a Tier 2 background task is active, physical actions that would conflict
(movement, digging, placing, combat) are blocked with a descriptive message.
The agent must `cancel_task` or wait for completion before issuing conflicting
tools.

## Exported API

<!-- BEGIN:generated:exported-api -->

```
package game // import "github.com/mab-go/golem/internal/game"

Package game provides action handlers bridging Claude tool calls to gRPC.

const DefaultToolTimeout = 60 * time.Second
func IsLongRunning(toolName string) bool
type Dispatcher struct{ ... }
    func NewDispatcher(botUsername string, client *sidecar.Client, mem *memory.Manager, state State, ...) *Dispatcher
type Result struct{ ... }
type State interface{ ... }
```

<!-- END:generated:exported-api -->

## Dependencies

<!-- BEGIN:generated:dependencies -->

- [`claude`](../claude/)
- [`grpc`](../grpc/)
- [`pb`](../grpc/pb/)
- [`logging`](../logging/)
- [`memory`](../memory/)
- [`perception`](../perception/)
- [`publisher`](../publisher/)
- [`task`](../task/)

<!-- END:generated:dependencies -->

## Used By

<!-- BEGIN:generated:used-by -->

- [`agent`](../agent/)

<!-- END:generated:used-by -->

## Key Files

| File | Purpose |
|------|---------|
| dispatcher.go | `Dispatcher` struct, handler registration, `Execute`, timeout logic |
| tier0.go | Atomic Mineflayer wrappers (move, look, dig, place, equip, attack, jump, sneak, chat) |
| tier1.go | Text-adventure verbs (navigate, harvest, craft, smelt, eat, containers, entity interaction) |
| tier2.go | Goal-oriented streaming tasks (gather, build, farm, clear, process, organize) |
| tier3.go | Read-only planning queries (survey, find, craft check, threat assessment, pathfinding) |
| perception.go | Look around, check inventory, take screenshot |
| meta.go | Set verbosity, memory writes (journal, goals, knowledge, notes, social), cancel task |

## See Also

- [claude](../claude/) -- defines the tool catalog (`tools.go`) that
  this package dispatches
- [grpc](../grpc/) -- the client this package calls for all sidecar
  communication
- [task](../task/) -- manages the single active background-task slot
  for Tier 2 ops
- [memory](../memory/) -- written to by meta tool handlers
