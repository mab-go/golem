# AGENTS.md

## Project

`golem` is an autonomous AI agent (powered by Claude) that plays Minecraft as a
genuine co-op survival partner. A Go agent drives the Perceive -> Think -> Act
-> Remember loop, speaks gRPC to a Node.js Mineflayer sidecar, and persists
memory to disk.

**Module:** `github.com/mab-go/golem`
**Architecture:** Go agent calls the Anthropic API directly; this
project is not a plugin or tool-server.

Two binaries: `cmd/golem/` (headless agent) and `cmd/golem-tui/` (Bubble Tea
mission control TUI). Minecraft target: 1.21.9, offline auth.

## Where To Look Next

Area-specific conventions live in subdirectory `AGENTS.md` files (with
`CLAUDE.md` symlinks alongside), loaded additively as Claude walks into each
tree:

- `internal/AGENTS.md` -- Go code: composition rules, error handling, config,
  guardrail vocabulary, scoped commands.
- `sidecar/AGENTS.md` -- Node.js / TypeScript sidecar: boundary rule, the four
  Mineflayer mitigations, scoped commands.
- `proto/AGENTS.md` -- gRPC contract: the 7-file checklist for adding a new
  tool.

Every `internal/` and `cmd/` package also has a `README.md` with hand-written
narrative plus auto-generated sections. Run `make docs` after changes that
affect exported APIs, package imports, or tool definitions `make docs:check` is
a dry-run that fails if stale.

## Verification

Before ANY change is considered done, run all five targets. All must pass. No
exceptions.

```
make fmt
make build
make test
make lint
make cyclo
```

- `make build` compiles both Go binaries (`golem`, `golem-tui`) and sidecar
  TypeScript.
- `make test` runs with `-race` by default.
- `make cyclo` reports functions over complexity 10. New code must not add to
  the list.

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

TypeScript ESM in `sidecar/src/`. Pure game interface -- no Claude knowledge.
gRPC server on port 50051. Proto contract: `proto/minecraft.proto`; run
`make proto` to regenerate both languages.

### Cognitive Model

Four model tiers (see `internal/claude/models.go` for current defaults; all\
overridable via `GOLEM_MODEL_*`):
- **Player** -- conscious mind: moment-to-moment gameplay (agentic loop)
- **Writer** -- journal/knowledge prose synthesis
- **Workhorse** -- reflexes: gatekeeper wake/sleep decisions, event
  classification
- **Deep** -- strategic advisor: on-demand escalation via `think_deeply`

### Action Tiers

- **Tier 0** -- 1:1 Mineflayer wrappers (atomic, single bot API call)
- **Tier 1** -- Text-adventure verbs ("navigate to", "craft item",
  "harvest block")
- **Tier 2** -- Goal-oriented streaming tasks (multi-step, cancellable,
  progress-reporting)
- **Tier 3** -- Read-only planning queries (safe to call freely, no
  side effects)

## Project-Wide Rules

**ASCII only.** Use only characters typeable on a US English keyboard in
Markdown files and Go/TypeScript source comments. No Unicode arrows (use `->`),
no em-dashes (use `--` or rephrase), no multiplication signs (use `x`), no
special math symbols (use `>=` not the glyph). TUI box-drawing characters for
terminal rendering are exempt.

**Markdown line wrapping.** Wrap regular prose in Markdown files at 80
characters. Exceptions: code blocks, inline code spans, CLI usage examples,
Markdown tables, URLs, and cases where wrapping would reduce readability.

## Development Commands

```bash
make setup             # First-time: install Go tools + sidecar npm deps
make proto             # Regenerate Go + TypeScript from proto/minecraft.proto
make run ARGS="serve"  # Run the headless agent
make help              # Full target list
```

Docker: `docker compose up --build` brings up both services.

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
