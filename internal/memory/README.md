# memory

On-disk persistence for the agent's long-term memory. Five files
capture different aspects of the agent's experience and knowledge,
surviving across restarts.

## Overview

The `Manager` serializes all access to a memory directory through a
single mutex. Four markdown files (journal, goals, world-knowledge,
social) and one JSON file (inventory-notes) are auto-seeded on first
run. All writes are atomic -- content goes to a temp file first, then
gets renamed into place to prevent corruption on partial writes. The
journal appends timestamped entries; all other files are full
replacements. Missing files return empty strings rather than errors.

## Exported API

<!-- BEGIN:generated:exported-api -->

```
package memory // import "github.com/mab-go/golem/internal/memory"

Package memory manages the agent's persistent memory files on disk.

const FileJournal = "journal.md" ...
type Manager struct{ ... }
    func New(dir string) (*Manager, error)
type State struct{ ... }
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
- [`claude`](../claude/)
- [`game`](../game/)

<!-- END:generated:used-by -->

## See Also

- [docs/getting-started.md](../../docs/getting-started.md) -- memory
  system overview for operators
