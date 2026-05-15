# AGENTS.md - internal/

Conventions for Go code under `internal/`. The root `AGENTS.md` has project-wide
rules; this file scopes Go-specific guidance.

## Code Composition

Top-level functions should scan as a DSL of named steps. They express WHAT
happens as a sequence of well-named calls. The HOW lives in the helpers.

When refactoring a function with high cyclomatic complexity, apply this
principle: extract blocks that have their own distinct purpose into named
helpers, so the parent reads as a series of named steps.

**Extract when** a block has a distinct purpose nameable in 2-3 words, operates
at a different abstraction level than its surroundings, and replacing 10-30
inline lines with a named call makes the parent more readable.

**Do NOT extract when** the code is purely sequential setup with no branching,
the helper would need 4+ parameters (the extraction boundary is wrong), the name
would just restate the code in camelCase, or the code is declarative data (tool
definitions, config structs).

**Naming:** Helpers describe what they yield or do -- `drainChat`,
`writeEntityBuckets`, `shortEntityList`. Never name by where it's called --
`thinkStep1`, `handlePart2`.

**Placement:** Helpers stay in the same file as their caller unless they serve
multiple files in the package. Never create a file just for one small helper.

**Anti-patterns:** Helper soup (extracting every 5-line block obscures a
readable 40-line flow). State-smuggling parameters (5+ params means the
extraction boundary is wrong). Naming after the caller. Extracting trivial
`if err != nil` handling.

**After refactoring:** Read the top-level function aloud -- does it scan as
named steps? Run `make cyclo` -- complexity must hold steady or improve.

## Conventions

**Error handling.** Game action handlers return `(resultText string, error)`.
Tool input errors and game-logic failures (missing recipe, unreachable block)
return the error as `resultText` with `nil` error -- Claude sees a readable
explanation. Only gRPC transport failures return non-nil `error` (which aborts
the loop). Do NOT use `return "", fmt.Errorf(...)` for tool/game errors.

**Guardrail-safe vocabulary.** Use neutral action names to avoidvAnthropic API
content filter false positives: `interact_with_entity(cow, harvest)` not
`kill(cow)`. Actions: harvest, attack, feed, trade, mount, shear.

**Config.** Viper with env prefix `GOLEM`. Key: `GOLEM_ANTHROPIC_API_KEY`. Model
overrides: `GOLEM_MODEL_PLAYER`, `_WRITER`, `_WORKHORSE`, `_DEEP`.

**Logging.** `internal/logging/` follows the mab-go pattern. DO NOT MODIFY this
package.

**Generated code.** `internal/grpc/pb/` is committed and never hand-edited. Run
`make proto` to regenerate after editing `proto/minecraft.proto`.

**mab-go patterns.** Study sibling projects (e.g. xmind-mcp, sheets-mcp) for
established CLI, config, error handling, and Makefile conventions.

## Scoped Commands

The Makefile targets (`test`, `lint`, `vet`, `cyclo`) all run the entire repo.
When working in a single package, prefer the scoped invocations to avoid running
the full suite:

```
go test -race ./internal/<pkg>/...
go vet ./internal/<pkg>/...
golangci-lint run ./internal/<pkg>/...
gocyclo -over 10 internal/<pkg>
```

The full `make fmt build test lint cyclo` block still runs before a change is
considered done -- the scoped commands are for iteration inside a package.

## Package READMEs

Every `internal/` package has a `README.md` with hand-written narrative plus
auto-generated sections (between HTML comment markers like
`<!-- BEGIN:generated:exported-api -->`). Run `make docs` after changes that
affect exported APIs, package imports, or tool definitions. `make docs:check`
is a dry-run that fails if any section is stale.

To add a new package README, copy `docs/templates/package-readme.md`, write the
narrative sections, then run `make docs`.
