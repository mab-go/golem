# AGENTS.md

## Project

`golem` is an autonomous AI agent (powered by Claude) that plays
Minecraft as a genuine co-op survival partner. A Go agent drives the
Perceive -> Think -> Act -> Remember loop, speaks gRPC to a Node.js
Mineflayer sidecar, and persists memory to disk.

**Module:** `github.com/mab-go/golem`
**Architecture:** Go agent calls the Anthropic API directly; this
project is not a plugin or tool-server.

Two binaries: `cmd/golem/` (headless agent) and `cmd/golem-tui/`
(Bubble Tea mission control TUI). Minecraft target: 1.21.9, offline
auth. Phase 7 (Multiplayer, Polish, Memory Management) is current --
see `docs/plans/`.

## Verification

Before ANY change is considered done, run all five targets. All must
pass. No exceptions.

```
make fmt
make build
make test
make lint
make cyclo
```

- `make build` compiles both Go binaries (`golem`, `golem-tui`) and
  sidecar TypeScript.
- `make test` runs with `-race` by default.
- `make cyclo` reports functions over complexity 10. New code must not
  add to the list.

## Code Composition

Top-level functions should scan as a DSL of named steps. They express
WHAT happens as a sequence of well-named calls. The HOW lives in the
helpers.

When refactoring a function with high cyclomatic complexity, apply this
principle: extract blocks that have their own distinct purpose into
named helpers, so the parent reads as a series of named steps.

**Extract when** a block has a distinct purpose nameable in 2-3 words,
operates at a different abstraction level than its surroundings, and
replacing 10-30 inline lines with a named call makes the parent more
readable.

**Do NOT extract when** the code is purely sequential setup with no
branching, the helper would need 4+ parameters (the extraction boundary
is wrong), the name would just restate the code in camelCase, or the
code is declarative data (tool definitions, config structs).

**Naming:** Helpers describe what they yield or do -- `drainChat`,
`writeEntityBuckets`, `shortEntityList`. Never name by where it's
called -- `thinkStep1`, `handlePart2`.

**Placement:** Helpers stay in the same file as their caller unless
they serve multiple files in the package. Never create a file just for
one small helper.

**Anti-patterns:** Helper soup (extracting every 5-line block obscures
a readable 40-line flow). State-smuggling parameters (5+ params means
the extraction boundary is wrong). Naming after the caller. Extracting
trivial `if err != nil` handling.

**After refactoring:** Read the top-level function aloud -- does it
scan as named steps? Run `make cyclo` -- complexity must hold steady
or improve.

## Conventions

**mab-go patterns.** Study sibling projects for established CLI,
config, error handling, and Makefile conventions:
- xmind-mcp: `/home/matt/Projects/mcp/xmind-mcp`
- sheets-mcp: `/home/matt/Projects/mcp/sheets-mcp`

**Logging.** `internal/logging/` follows the mab-go pattern. DO NOT MODIFY this package.

**Config.** Viper with env prefix `GOLEM`. Key:
`GOLEM_ANTHROPIC_API_KEY`. Model overrides: `GOLEM_MODEL_PLAYER`,
`_WRITER`, `_WORKHORSE`, `_DEEP`.

**Error handling.** Game action handlers return
`(resultText string, error)`. Tool input errors and game-logic failures
(missing recipe, unreachable block) return the error as `resultText`
with `nil` error -- Claude sees a readable explanation. Only gRPC
transport failures return non-nil `error` (which aborts the loop).
Do NOT use `return "", fmt.Errorf(...)` for tool/game errors.

**Generated code.** `internal/grpc/pb/` and
`sidecar/src/grpc/generated/` are committed. Never hand-edit -- run
`make proto` to regenerate.

**Guardrail-safe vocabulary.** Use neutral action names to avoid
Anthropic API content filter false positives:
`interact_with_entity(cow, harvest)` not `kill(cow)`. Actions:
harvest, attack, feed, trade, mount, shear.

**Sidecar boundary.** The sidecar (`sidecar/src/`) has NO knowledge of
Claude or the Anthropic API. It is a pure Mineflayer game interface.
Keep it that way.

**ASCII only.** Use only characters typeable on a US English keyboard
in Markdown files and Go/TypeScript source comments. No Unicode arrows
(use `->`), no em-dashes (use `--` or rephrase), no multiplication signs
(use `x`), no special math symbols (use `>=` not the glyph). TUI box-drawing
characters for terminal rendering are exempt.

**Markdown line wrapping.** Wrap regular prose in Markdown files at 80
characters. Exceptions: code blocks, inline code spans, CLI usage
examples, Markdown tables, URLs, and cases where wrapping would reduce
readability.

## Architecture

### Go Agent (`internal/`)

| Package | Purpose |
|---|---|
| `agent/` | Agentic loop, pacing state machine, chat interceptor, task manager, wake signals |
| `claude/` | Anthropic SDK wrapper, tool catalog, context assembly, system prompt, gatekeeper, metrics |
| `game/` | Dispatcher routing `tool_use` blocks to gRPC calls; handlers organized by tier |
| `grpc/` | Client wrapper for sidecar RPCs. `pb/` has generated protobuf code. |
| `perception/` | Formats gRPC perception data as prose or structured text |
| `memory/` | On-disk markdown/JSON files: journal, goals, world-knowledge, inventory-notes, social |
| `logging/` | Logrus wrapper (mab-go pattern -- DO NOT MODIFY) |
| `publisher/` | Fire-and-forget event bridge to TUI |
| `version/` | ldflags injection (Version, Commit, Date) |

### Sidecar

TypeScript ESM in `sidecar/src/`. Pure game interface -- no Claude
knowledge. gRPC server on port 50051. Proto contract:
`proto/minecraft.proto`; run `make proto` to regenerate both languages.

### Cognitive Model

Four model tiers (see `internal/claude/models.go` for current defaults;
all overridable via `GOLEM_MODEL_*`):
- **Player** -- conscious mind: moment-to-moment gameplay (agentic loop)
- **Writer** -- journal/knowledge prose synthesis
- **Workhorse** -- reflexes: gatekeeper wake/sleep decisions, event classification
- **Deep** -- strategic advisor: on-demand escalation via `think_deeply`

### Action Tiers

New tools go in the tier matching their abstraction level:
- **Tier 0** -- 1:1 Mineflayer wrappers (atomic, single bot API call)
- **Tier 1** -- Text-adventure verbs ("navigate to", "craft item", "harvest block")
- **Tier 2** -- Goal-oriented streaming tasks (multi-step, cancellable,
  progress-reporting)
- **Tier 3** -- Read-only planning queries (safe to call freely, no side effects)

## Adding a New Tool

Every new tool touches 7 files across two languages, in this order:

1. `proto/minecraft.proto` -- Add request/response messages + RPC to
   `MinecraftService`
2. Run `make proto` -- Regenerates Go and TypeScript code
3. `sidecar/src/bot/actions/tierN.ts` -- Implement the Mineflayer logic
4. `sidecar/src/grpc/server.ts` -- Wire the handler to the RPC endpoint
5. `internal/grpc/client.go` -- Add Go wrapper method
6. `internal/claude/tools.go` -- Define the tool (name, description, JSON schema)
7. `internal/game/tierN.go` -- Write handler; register in `dispatcher.go`

Missing any file means the tool silently fails or doesn't appear in
Claude's tool catalog.

## Sidecar Constraints

Deliberate mitigations for Mineflayer bugs. These are NOT tech debt to
clean up:

- **Crafting:** `bot.craft(recipe, N)` silently loses items when
  N > 1. The one-at-a-time loop in `tier1.ts craftItem` is intentional.
- **Pathfinding:** Always use `safeMovements()` or `digMovements()`
  from `helpers.ts`. Never construct raw `new Movements(bot)` -- the
  safe variants prevent tunneling into lava and wasting inventory on
  scaffolding.
- **Position NaN:** `connection.ts` has a watchdog that recovers from
  Mineflayer's position-NaN bug. The bot's physics stop firing once
  position is NaN, so only external polling can detect it. Do not
  remove.
- **Viewer patching:** `scripts/patch-viewer.cjs` patches
  prismarine-viewer for the target MC version at install time. Re-run
  `npm install` in `sidecar/` when changing `MINECRAFT_VERSION`.

## Development Commands

```bash
make setup             # First-time: install Go tools + sidecar npm deps
make proto             # Regenerate Go + TypeScript from proto/minecraft.proto
make run ARGS="serve"  # Run the headless agent
make help              # Full target list
```

Sidecar: `cd sidecar && npm install`, then `npm start`
(or `npm run dev` for tsx).
Docker: `docker compose up --build` brings up both services.

## Documentation

Every `internal/` and `cmd/` package gets a `README.md` with
hand-written narrative plus auto-generated sections. The template is
at `docs/templates/package-readme.md`.

**Generated sections** live between HTML comment markers:

```html
<!-- BEGIN:generated:exported-api -->
(auto-populated content)
<!-- END:generated:exported-api -->
```

Available markers: `exported-api`, `dependencies`, `used-by`,
`package-map`, `tool-catalog`.

**Keeping docs current:**

```bash
make docs          # Regenerate all auto-populated sections
make docs:check    # Dry-run: exit 1 if any section is stale
```

Run `make docs` after changes that affect exported APIs, package
imports, or tool definitions. The generator (`tools/docgen/`) reads
`go list`, `go doc`, and `internal/claude/tools.go` to populate
markers. It never creates files or touches hand-written content.

**Adding a new package README:** Copy the template from
`docs/templates/package-readme.md`, write the narrative sections,
then run `make docs`.

## Key Files

| File | Purpose |
|---|---|
| `proto/minecraft.proto` | gRPC contract (source of truth for all RPCs) |
| `internal/claude/tools.go` | Tool catalog (source of truth for all tools) |
| `internal/claude/models.go` | Model tier definitions and defaults |
| `internal/claude/system_prompt.go` | Persona and runtime prompt |
| `internal/agent/loop.go` | Agentic loop entry point |
| `internal/game/dispatcher.go` | `tool_use` routing and handler registry |
| `cmd/golem/main.go` | Headless agent CLI |
| `cmd/golem-tui/main.go` | TUI mission control |
