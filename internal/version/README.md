# version

Build metadata injected via ldflags at compile time. Provides version,
commit SHA, and build date for `--version` output.

## Overview

Three package-level variables (`Version`, `Commit`, `Date`) are set by
the linker during `go build` via `-ldflags -X`. Helper functions format
these for display.

## Exported API

<!-- BEGIN:generated:exported-api -->

```
package version // import "github.com/mab-go/golem/internal/version"

Package version holds build metadata injected via ldflags.

Example:

    go build -ldflags "-X github.com/mab-go/golem/internal/version.Version=1.0.0 \
      -X github.com/mab-go/golem/internal/version.Commit=$(git rev-parse HEAD) \
      -X github.com/mab-go/golem/internal/version.Date=$(date -u +%Y-%m-%d)" ./cmd/golem

var Commit = "0000000000000000000000000000000000000000"
var Date = "0000-00-00"
var Version = "0.0.0"
func Full() string
func ShortCommit() string
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

<!-- END:generated:used-by -->
