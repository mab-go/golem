# bot/actions

Tiered action handlers for the sidecar's gRPC service. Actions are organized
into four tiers by abstraction level; all tiers share the utilities in
`helpers.ts` and the task lifecycle wrapper in `task-runner.ts`.

## Overview

### Tier Architecture

**Tier 0** -- Atomic, 1:1 Mineflayer wrappers. Each function makes a single
bot API call and returns immediately. No pathfinding, no multi-step logic.
Examples: `moveTo`, `digBlock`, `placeBlock`.

**Tier 1** -- Composite "text-adventure verbs". Each function combines
navigation, inventory management, and interaction into a single coherent
action. Examples: `navigateTo`, `craftItem`, `harvestBlock`, `smeltItem`.

**Tier 2** -- Goal-oriented streaming tasks. Long-running loops that report
progress over a server-streaming RPC. Task bodies receive a `TaskContext`
(from `task-runner.ts`) for progress reporting and cancellation checks.
Cancellation calls `pathfinder.stop()` so navigation unwinds cleanly.
Examples: `gather`, `farm`, `clearArea`.

**Tier 3** -- Read-only planning queries. No side effects; safe to call
at any time without disrupting ongoing work. Examples: `surveyArea`,
`assessThreat`, `planPath`.

### Guardrail-Safe Vocabulary

Interaction verbs use neutral names to avoid Anthropic API content filter
false positives: `interact_with_entity(cow, harvest)` rather than `kill`.
Supported interaction types: harvest, attack, feed, trade, mount, shear.

### Crafting Mitigation

`craftItem` (tier 1) crafts one execution at a time in a loop rather than
calling `bot.craft(recipe, N)`. This is intentional: Mineflayer silently
loses items when N > 1, consuming ingredients without producing output.
Do not "fix" this loop.

### Pathfinding Profiles

All navigation uses `safeMovements()` or `digMovements()` from `helpers.ts`.
Never construct `new Movements(bot)` directly in action handlers -- the safe
variants prevent tunneling into lava and forbid consuming inventory items as
scaffolding. The one exception is `SurvivalAutopilot.threatFlee()`, which
uses raw `Movements` to allow sprinting without scaffolding restrictions.

## Key Files

| File             | Purpose                                                                       |
| ---------------- | ----------------------------------------------------------------------------- |
| `tier0.ts`       | Atomic actions: move, look, place, dig, equip, use, attack, jump, sneak       |
| `tier1.ts`       | Composite actions: navigate, interact, harvest, containers, craft, smelt, eat |
| `tier2.ts`       | Streaming tasks: gather, build, process, organize, clear, farm                |
| `tier3.ts`       | Read-only queries: survey, find, craft-check, threat, path-plan               |
| `helpers.ts`     | Shared utilities: inventory, pathfinding, entity/item lookup, drops           |
| `task-runner.ts` | Tier 2 task lifecycle: STARTED/IN_PROGRESS/COMPLETED/FAILED/CANCELLED         |

## See Also

- [../](../) -- connection, events, perception, autopilot
- [../../grpc/](../../grpc/) -- gRPC server that wires handlers to RPCs
