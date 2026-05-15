# AGENTS.md - sidecar/

Conventions for the Node.js / TypeScript sidecar. The root `AGENTS.md` has
project-wide rules; this file scopes sidecar-specific guidance.

## Sidecar Boundary

The sidecar (`sidecar/src/`) has NO knowledge of Claude or the Anthropic API.
It is a pure Mineflayer game interface that exposes gRPC RPCs (port 50051). The
Go agent is the only Claude-aware component. Keep it that way -- do not import
the Anthropic SDK or any Claude-related types into sidecar code.

## Sidecar Constraints

Deliberate mitigations for Mineflayer bugs. These are NOT tech debt to clean up:

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

## Generated Code

`sidecar/src/grpc/generated/` is committed and never hand-edited. Run
`make proto` (from the repo root) to regenerate after editing
`proto/minecraft.proto`.

## Scoped Commands

```
cd sidecar && npm test           # or, from repo root: make test:sidecar
cd sidecar && npm run build
cd sidecar && npm run dev        # tsx watch mode
cd sidecar && npm start
```

The full `make fmt build test lint cyclo` block still runs before a change is
considered done -- the scoped commands are for iteration inside the sidecar.
