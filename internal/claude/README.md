# claude

Wraps the Anthropic SDK and implements golem's four-tier cognitive architecture.
This package handles all communication with the Claude API: streaming requests,
prompt caching, tool definitions, context assembly, and specialized model calls
for gatekeeper filtering, journal compaction, knowledge synthesis, event
classification, and strategic escalation.

## Overview

### Four Model Tiers

The package defines four cognitive tiers, each using a different Claude model
optimized for its role:

| Tier | Role | Default Model | Used For |
|------|------|---------------|----------|
| Player | Conscious mind | Sonnet | Moment-to-moment gameplay (agentic loop) |
| Writer | Prose synthesis | Sonnet | Journal compaction, knowledge summaries |
| Workhorse | Reflexes | Haiku | Gatekeeper wake/sleep, event classification |
| Deep | Strategic advisor | Opus | On-demand escalation via `think_deeply` |

All defaults are overridable via `GOLEM_MODEL_*` environment variables.

### Prompt Caching

The system prompt is split into a **stable** section (persona, tool reference,
gameplay guidance -- constant across turns) and a **dynamic** section (runtime
settings like verbosity and bot name -- changes per session). The stable section
gets an ephemeral cache breakpoint so subsequent turns cache-hit the persona and
tool catalog. The last block of each conversation message is also marked for
caching, so the conversation prefix grows incrementally without retransmission.

### Context Assembly

Each turn, the `ContextBuilder` assembles a user message from:
- **Memory** -- goals, journal (last 80 lines), world knowledge,
  inventory notes, social observations (all read from disk)
- **Perception** -- vital signs, surroundings, inventory (from sidecar gRPC)
- **Events** -- classified game events since last turn
- **Active task** -- background task status and progress
- **Player chat** -- buffered chat messages with sender attribution

History management trims on conversation-start boundaries (user messages without
tool results) to keep the sliding window coherent.

### Tool Catalog

The package defines 41 tools organized across four tiers plus perception, meta,
and escalation categories. Tool definitions include JSON Schema 2020-12 input
schemas. The `Tools()` function returns the canonical ordered list;
`ToolsByName()` returns a map for dispatcher lookups.

## Exported API

<!-- BEGIN:generated:exported-api -->

```
package claude // import "github.com/mab-go/golem/internal/claude"

Package claude wraps the Anthropic SDK for Claude API communication.

const DefaultModelPlayer = "claude-sonnet-4-6" ...
const ViperKeyModelPlayer = "model_player" ...
const ToolMoveTo = "move_to" ...
const DefaultMaxTokens int64 = 4096
var ModelPlayer string ...
func InitModels()
func SystemPrompt(cfg SystemPromptConfig) string
func ToolsByName() map[string]Tool
type Block struct{ ... }
type BlockType string
    const BlockText BlockType = "text" ...
type CacheableSystemPrompt struct{ ... }
    func SystemPromptParts(cfg SystemPromptConfig) CacheableSystemPrompt
type CallOption func(*callOpts)
    func Silent() CallOption
    func WithTextDelta(fn func(string)) CallOption
type ChatMessage struct{ ... }
type Client struct{ ... }
    func NewClient(apiKey string, maxTokens int64, metrics *Metrics, log logging.Logger) *Client
type ContextBuilder struct{ ... }
    func NewContextBuilder(mem *memory.Manager, fmtr *perception.Formatter, hist *History) *ContextBuilder
type GatekeeperDecision struct{ ... }
type GatekeeperSnapshot struct{ ... }
type History struct{ ... }
    func NewHistory(maxMessages int) *History
type Message struct{ ... }
type Metrics struct{ ... }
    func NewMetrics() *Metrics
type ModelTotals struct{ ... }
type PerceptionSnapshot struct{ ... }
type Response struct{ ... }
type Role string
    const RoleUser Role = "user" ...
type SystemPromptConfig struct{ ... }
type Tool struct{ ... }
    func Tools() []Tool
type ToolUse struct{ ... }
type Usage struct{ ... }
```

<!-- END:generated:exported-api -->

## Dependencies

<!-- BEGIN:generated:dependencies -->

- [`pb`](../grpc/pb/)
- [`logging`](../logging/)
- [`memory`](../memory/)
- [`perception`](../perception/)

<!-- END:generated:dependencies -->

## Used By

<!-- BEGIN:generated:used-by -->

- [`golem`](../../cmd/golem/)
- [`golem-tui`](../../cmd/golem-tui/)
- [`agent`](../agent/)
- [`game`](../game/)

<!-- END:generated:used-by -->

## Key Files

| File | Purpose |
|------|---------|
| client.go | Streaming API client, SDK type conversion, cache breakpoint placement |
| context.go | `ContextBuilder` and `History` -- per-turn message assembly from memory + perception |
| tools.go | Tool name constants and `Tool` definitions with JSON Schema input schemas |
| system_prompt.go | Persona prompt split into cacheable stable + dynamic sections |
| models.go | Model tier constants, Viper-driven overrides, `InitModels()` |
| gatekeeper.go | Haiku-powered wake/sleep filter with entity aggregation and compact snapshots |
| classification.go | Haiku-powered event priority re-ranking |
| escalation.go | `ThinkDeeply` -- Opus escalation for strategic reasoning |
| journal.go | `CompactJournal` -- Sonnet-powered journal entry summarization |
| knowledge.go | `SynthesizeKnowledge` -- Sonnet-powered world knowledge consolidation |
| metrics.go | Thread-safe per-model token usage tracking |

## See Also

- [agent](../agent/) -- owns the agentic loop that calls into this package
- [game](../game/) -- dispatcher that routes tool calls to gRPC handlers
- [perception](../perception/) -- formatter that this package uses for
  context assembly
- [memory](../memory/) -- on-disk state that `ContextBuilder` reads each turn
