# grpc

gRPC client wrapper for the Mineflayer sidecar. Every bot action and
perception query flows through this package as either a unary RPC or a
server-streaming task channel.

## Overview

The `Client` type wraps the generated protobuf service client with a
Go-idiomatic API organized by action tier. Unary RPCs (Tier 0 atomics,
Tier 1 verbs, Tier 3 queries) return results directly. Tier 2 streaming
RPCs (gather, build, farm) return a `chan TaskEvent` that the task
manager drains for progress updates -- a goroutine pumps the server
stream and closes the channel on EOF or error.

The connection is plaintext and insecure by design -- the sidecar runs
locally on the same machine.

## Exported API

<!-- BEGIN:generated:exported-api -->

```
package grpc // import "github.com/mab-go/golem/internal/grpc"

Package grpc implements the gRPC client for the Mineflayer sidecar.

type Client struct{ ... }
    func NewClient(address string) (*Client, error)
type TaskEvent struct{ ... }
```

<!-- END:generated:exported-api -->

## Dependencies

<!-- BEGIN:generated:dependencies -->

- [`pb`](pb/)

<!-- END:generated:dependencies -->

## Used By

<!-- BEGIN:generated:used-by -->

- [`golem`](../../cmd/golem/)
- [`golem-tui`](../../cmd/golem-tui/)
- [`agent`](../agent/)
- [`game`](../game/)
- [`task`](../task/)

<!-- END:generated:used-by -->

## See Also

- [proto/minecraft.proto](../../proto/minecraft.proto) -- gRPC contract
  (source of truth for all RPCs)
- [claude](../claude/) -- tool definitions that map to these RPCs
- [game](../game/) -- dispatcher that calls these RPCs from tool_use blocks
