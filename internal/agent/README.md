# agent

The agentic loop that drives golem's Perceive -> Think -> Act -> Remember cycle.
This package owns the runtime lifecycle of the bot: event ingestion, gatekeeper
filtering, Claude API interaction, tool execution with interruption safety, and
operator command handling.

## Overview

The agent runs four concurrent goroutines:

1. **Main loop** -- waits on a wake channel, then runs a full think cycle
   (perception assembly -> Claude API call -> tool execution, up to 12 rounds)
2. **Event loop** -- subscribes to the sidecar's gRPC event stream, buffers
   events, and routes critical events or chat to fast-path handlers
3. **Perception loop** -- ticks every 3s, drains buffered events, and consults
   the gatekeeper (Haiku) to decide if the agent should wake
4. **Heartbeat loop** -- fires every 45s to ensure temporal awareness during
   quiet periods

The gatekeeper is the key cost-control mechanism: the expensive Player model
(Sonnet) only runs when the gatekeeper or a bypass condition says something
interesting happened. Five wake reasons exist:

| Reason | Source | Skips Gatekeeper? |
|--------|--------|-------------------|
| `WakeGatekeeper` | Haiku said "wake up" | No (this *is* the gatekeeper) |
| `WakeBypassChat` | Player sent a chat message | Yes |
| `WakeBypassCritical` | Critical-priority game event | Yes |
| `WakeBypassTask` | Background task completed/failed | Yes |
| `WakeHeartbeat` | Periodic timer | Yes |

Before consulting the gatekeeper, the perception loop filters out routine noise
(entity despawns, passive mob spawns, weather changes) when the bot is healthy
and was recently awake. This prevents unnecessary Haiku calls during calm
periods.

Tool execution is tracked by a flight recorder (`tool_flight.go`) that enables
safe interruption. When an emergency stop command or critical event arrives
mid-tool, the in-flight tool's context is cancelled with a structured reason so
Claude gets a clear explanation rather than a generic timeout.

The `ChatInterceptor` handles `/golem` operator commands locally (pause, resume,
verbosity, stop, shutdown, status) without invoking Claude, and surfaces
human chat messages to the main loop for the agent to respond to.

## Exported API

<!-- BEGIN:generated:exported-api -->

```
package agent // import "github.com/mab-go/golem/internal/agent"

Package agent implements the Perceive-Think-Act-Remember agentic loop.

const OperatorCommandPrefix = "/golem"
type Agent struct{ ... }
    func New(ctx context.Context, cfg Config, client *sidecar.Client, ai *claude.Client, ...) *Agent
type CancelReason int
    const CancelEmergencyStop CancelReason = iota + 1 ...
type ChatInterceptor struct{ ... }
    func NewChatInterceptor(client *sidecar.Client, pacing *PacingState, mem *memory.Manager, ...) *ChatInterceptor
type Config struct{ ... }
type GatekeeperResult struct{ ... }
type HandleResult struct{ ... }
type PacingState struct{ ... }
    func NewPacingState(v perception.VerbosityMode) *PacingState
type WakeReason int
    const WakeGatekeeper WakeReason = iota ...
type WakeSignal struct{ ... }
```

<!-- END:generated:exported-api -->

## Dependencies

<!-- BEGIN:generated:dependencies -->

- [`claude`](../claude/)
- [`game`](../game/)
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

- [`golem`](../../cmd/golem/)
- [`golem-tui`](../../cmd/golem-tui/)

<!-- END:generated:used-by -->

## Key Files

| File | Purpose |
|------|---------|
| agent.go | Core `Agent` struct, config, perception fetching, event/chat buffers |
| loop.go | Main runloop: `Run()`, `think()`, tool execution, background goroutines |
| pacing.go | Pause/resume and verbosity state (thread-safe) |
| chat.go | `ChatInterceptor` -- operator commands and human chat classification |
| wake.go | `WakeSignal` and `WakeReason` definitions, gatekeeper result carrying |
| events.go | Event filtering: bypass detection, routine-only checks, task completion |
| event_policy.go | Additional event classification policy |
| gatekeeper_health.go | Monitors gatekeeper (Haiku) failure rate in a sliding window |
| tool_flight.go | Tracks in-flight tools, enables context-based interruption |

## See Also

- [claude](../claude/) -- API client, tool catalog, context assembly,
  gatekeeper logic
- [game](../game/) -- tool dispatch and handler implementations
- [perception](../perception/) -- formats game data into text for the API
- [task](../task/) -- background task manager for Tier 2 streaming operations
