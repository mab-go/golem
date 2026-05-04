# grpc

The gRPC server that wires Mineflayer action handlers to the
MinecraftService proto contract.

## Overview

`server.ts` implements `MinecraftServiceServer` -- the full set of RPCs
defined in `proto/minecraft.proto`. Unary handlers are wrapped by
`asyncHandler()`, which injects the current bot and cancels pathfinding
if the client disconnects mid-call. Streaming handlers (Tier 2 tasks and
event subscription) manage their own lifecycles via `runTask()` and
subscriber tracking respectively.

`generated/` contains the TypeScript protobuf bindings. These files are
auto-generated -- never hand-edit them. Run `make proto` from the repo
root to regenerate after changing `proto/minecraft.proto`.

## Key Files

| File                     | Purpose                                                 |
| ------------------------ | ------------------------------------------------------- |
| `server.ts`              | MinecraftService implementation and gRPC server factory |
| `generated/minecraft.ts` | Auto-generated proto types -- never hand-edited         |

## See Also

- [../../../proto/minecraft.proto](../../../proto/minecraft.proto) -- source of
  truth for all RPCs and message types
- [../bot/actions/](../bot/actions/) -- tier action handlers wired by
  server.ts
