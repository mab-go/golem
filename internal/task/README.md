# task

Single-slot task manager for Tier 2 streaming operations. At most one
background task (gather, build, farm, etc.) runs at a time.

## Overview

The `Manager` controls the lifecycle of long-running gRPC streaming
tasks. It drains the task's event channel, emits synthetic game events
through the publisher, handles cancellation and timeout, and wakes the
think loop when a task reaches terminal status. The single-slot design
is intentional -- the agent focuses on one goal-oriented operation at
a time, matching how a human player would approach multi-step tasks.

## Exported API

<!-- BEGIN:generated:exported-api -->

```
package task // import "github.com/mab-go/golem/internal/task"

Package task manages the single active background-task slot for Tier 2 streaming
operations (gather, build, farm, etc.).

type Manager struct{ ... }
    func NewManager(parentCtx context.Context, eventSink func(*pb.GameEvent), wakeFn func(), ...) *Manager
```

<!-- END:generated:exported-api -->

## Dependencies

<!-- BEGIN:generated:dependencies -->

- [`grpc`](../grpc/)
- [`pb`](../grpc/pb/)
- [`logging`](../logging/)
- [`perception`](../perception/)

<!-- END:generated:dependencies -->

## Used By

<!-- BEGIN:generated:used-by -->

- [`agent`](../agent/)
- [`game`](../game/)

<!-- END:generated:used-by -->

## See Also

- [grpc](../grpc/) -- provides the streaming task channels
- [agent](../agent/) -- owns the Manager and routes task tool calls through it
