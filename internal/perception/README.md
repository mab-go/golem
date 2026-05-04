# perception

Formats raw gRPC perception payloads into text that Claude can reason
about. Supports two output formats and three verbosity levels, enabling
A/B testing of which representation the model handles best.

## Overview

The `Formatter` renders surroundings, vital signs, inventory, and
events into either prose (text-adventure style) or structured (labeled
key-value) format. Three verbosity modes control detail density: terse
produces one-liners, standard gives concise specifics, and verbose
generates flowing atmospheric descriptions.

Entity classification maps Minecraft entity types into four categories
(hostile, neutral, passive, player) using hardcoded lookup tables.
Unknown entities default to passive. This classification drives how
entities appear in perception text and how the agent prioritizes
threats.

## Exported API

<!-- BEGIN:generated:exported-api -->

```
package perception // import "github.com/mab-go/golem/internal/perception"

Package perception formats raw game data into text-adventure descriptions.

type ClassifiedEvent struct{ ... }
type EntityCategory int
    const CategoryPlayer EntityCategory = iota ...
    func ClassifyEntity(entityType string) EntityCategory
type Format int
    const FormatProse Format = iota ...
    func ParseFormat(s string) (Format, error)
type Formatter struct{ ... }
    func NewFormatter(f Format, v VerbosityMode) *Formatter
type TaskStatus struct{ ... }
type VerbosityMode int
    const VerbosityStandard VerbosityMode = iota ...
    func ParseVerbosity(s string) (VerbosityMode, error)
```

<!-- END:generated:exported-api -->

## Dependencies

<!-- BEGIN:generated:dependencies -->

- [`pb`](../grpc/pb/)

<!-- END:generated:dependencies -->

## Used By

<!-- BEGIN:generated:used-by -->

- [`golem`](../../cmd/golem/)
- [`golem-tui`](../../cmd/golem-tui/)
- [`agent`](../agent/)
- [`claude`](../claude/)
- [`game`](../game/)
- [`task`](../task/)

<!-- END:generated:used-by -->

## See Also

- [grpc](../grpc/) -- provides the raw perception payloads
- [agent](../agent/) -- consumes formatted perception in the agentic loop
