# publisher

Fire-and-forget event bridge connecting the agent loop to external
consumers. Defines the `EventPublisher` interface without implementing
a concrete subscriber -- consumers (like the TUI) implement the
interface and inject it at startup.

## Overview

The `EventPublisher` interface declares methods for every lifecycle
event the agent produces: think cycle progress, text deltas, tool
execution, component health, memory updates, game events, chat
messages, gatekeeper decisions, and log entries. All methods are
non-blocking and goroutine-safe. A `Nop()` publisher provides
zero-overhead silence for headless mode.

## Exported API

<!-- BEGIN:generated:exported-api -->

```
package publisher // import "github.com/mab-go/golem/internal/publisher"

Package publisher defines the EventPublisher interface used to bridge the agent
loop to external consumers (e.g. a TUI). All methods are fire-and-forget:
no return values, no blocking.

type EventPublisher interface{ ... }
    func Nop() EventPublisher
type Status int
    const StatusOK Status = iota ...
type TurnStats struct{ ... }
```

<!-- END:generated:exported-api -->

## Dependencies

<!-- BEGIN:generated:dependencies -->

_No internal dependencies._

<!-- END:generated:dependencies -->

## Used By

<!-- BEGIN:generated:used-by -->

- [`golem`](../../cmd/golem/)
- [`golem-tui`](../../cmd/golem-tui/)
- [`agent`](../agent/)
- [`game`](../game/)

<!-- END:generated:used-by -->
