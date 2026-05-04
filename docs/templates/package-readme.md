# Package README Template

This is the canonical template for package-level READMEs in the golem project.
Copy this file into a package directory, rename it to `README.md`, and fill in
the hand-written sections. Run `make docs` to populate the generated sections.

---

## Template

```markdown
# <package-name>

<One-paragraph purpose statement. Expand on the doc.go comment -- what problem
does this package solve, and why does it exist as a separate package?>

## Overview

<Hand-written narrative. Cover: what the package does, key concepts and types,
design decisions worth knowing about. For simple packages a single sentence is
fine. For complex packages (agent, claude, game) this may be several paragraphs.>

## Exported API

<!-- BEGIN:generated:exported-api -->
<!-- END:generated:exported-api -->

## Dependencies

<!-- BEGIN:generated:dependencies -->
<!-- END:generated:dependencies -->

## Used By

<!-- BEGIN:generated:used-by -->
<!-- END:generated:used-by -->

## Key Files

| File | Purpose |
|------|---------|
| file.go | What this file is responsible for |

## See Also

- [related-package](../related-package/) -- how it relates
- [docs/relevant-doc.md](../../docs/relevant-doc.md) -- deeper context
```

---

## Section Rules

- **Overview**: Always present, always hand-written.
- **Exported API**: Generated. Shows `go doc` output as a fenced code block.
- **Dependencies**: Generated. Lists internal packages this package imports.
- **Used By**: Generated. Lists internal packages and binaries that
  import this package.
- **Key Files**: Hand-written. **Omit this section entirely** for
  packages with 3 or fewer files.
- **See Also**: Hand-written. **Omit this section entirely** if there
  is nothing meaningful to link.

## Variations

**For `cmd/` packages**: Replace the **Exported API** section with a
hand-written **CLI Usage** section showing subcommands and flags. Keep
the `dependencies` marker.

**For `internal/grpc/pb/`**: No README. This is generated protobuf code.

## Writing Conventions

- **80-character line wrap**: Wrap regular prose at 80 characters. Tables,
  code blocks, URLs, and CLI examples are exempt.
- **ASCII only**: Use only US-keyboard-typeable characters. Write `->` not
  the Unicode arrow, `--` not the em-dash, `x` not the multiplication sign.

## Generated Marker Reference

| Marker name | Data source |
|-------------|-------------|
| `exported-api` | `go doc ./internal/<pkg>` |
| `dependencies` | `go list -json` Imports field, filtered to module-internal paths |
| `used-by` | Inverted dependency graph from `go list` |
| `package-map` | Top-level README only: all packages with doc strings |
| `tool-catalog` | Top-level README only: tool constants from `internal/claude/tools.go` |
