# AGENTS.md - proto/

`minecraft.proto` is the source of truth for all gRPC RPCs between the Go agent
and the Node.js sidecar. Any edit here requires regenerating both language
bindings:

```
make proto
```

This regenerates `internal/grpc/pb/` (Go) and `sidecar/src/grpc/generated/`
(TypeScript). Both directories are committed; never hand-edit them.

## Adding a New Tool

Every new tool touches 7 files across two languages, in this order:

1. `proto/minecraft.proto` -- Add request/response messages + RPC to
   `MinecraftService`
2. Run `make proto` -- Regenerates Go and TypeScript code
3. `sidecar/src/bot/actions/tierN.ts` -- Implement the Mineflayer logic
4. `sidecar/src/grpc/server.ts` -- Wire the handler to the RPC endpoint
5. `internal/grpc/client.go` -- Add Go wrapper method
6. `internal/claude/tools.go` -- Define the tool (name, description, JSON
   schema)
7. `internal/game/tierN.go` -- Write handler; register in `dispatcher.go`

Missing any file means the tool silently fails or doesn't appear in Claude's
tool catalog.

## Action Tiers

New tools go in the tier matching their abstraction level:
- **Tier 0** -- 1:1 Mineflayer wrappers (atomic, single bot API call)
- **Tier 1** -- Text-adventure verbs ("navigate to", "craft item",
  "harvest block")
- **Tier 2** -- Goal-oriented streaming tasks (multi-step, cancellable,
  progress-reporting)
- **Tier 3** -- Read-only planning queries (safe to call freely, no
  side effects)
