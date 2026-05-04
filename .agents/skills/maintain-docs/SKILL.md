---
name: maintain-docs
description: >
  Update and maintain project documentation -- READMEs, docs/ pages,
  and auto-generated sections.
when_to_use: >
  When asked to update documentation, add a package README, refresh
  generated sections, or check for documentation drift after API changes.
disable-model-invocation: true
allowed-tools:
  - Read
  - Edit
  - Write
  - Bash(cd tools/docgen && go run . *)
  - Bash(go doc *)
  - Bash(go list *)
  - Bash(find * -name README.md *)
---

# Documentation Maintenance

This project uses a hybrid documentation system: hand-written narrative
sections plus auto-generated boilerplate, managed by a generator tool at
`tools/docgen/`.

## Mode 1: Update Generated Sections

Run the generator to refresh all auto-populated sections across all READMEs:

```bash
make docs
```

This populates content between `<!-- BEGIN:generated:SECTION -->` and
`<!-- END:generated:SECTION -->` markers. Hand-written content outside
these markers is never touched.

To check for drift without writing (useful in CI or before committing):

```bash
make docs:check
```

If `docs:check` reports stale sections, run `make docs` and commit the result.

## Mode 2: Add a New Package README

1. Read the template at `docs/templates/package-readme.md` for the
   canonical structure and section rules.
2. Read the package's source files to understand its purpose, key types,
   and design decisions.
3. Create `README.md` in the package directory following the template:
   - Write the purpose statement and Overview narrative by hand.
   - Include the generated section markers (`exported-api`,
     `dependencies`, `used-by`) with empty BEGIN/END pairs.
   - Add a Key Files table if the package has more than 3 `.go` files.
   - Add a See Also section only if there are meaningful links.
   - For `cmd/` packages: use a hand-written CLI Usage section instead
     of the exported-api marker.
   - Skip `internal/grpc/pb/` -- generated code gets no README.
4. Run `make docs` to populate the generated sections.
5. Review the generated output for accuracy.

Reference examples: `internal/agent/README.md`,
`internal/claude/README.md`, `internal/game/README.md`.

## Mode 3: Check for Documentation Drift

After API changes (new exports, renamed functions, added/removed
dependencies), generated sections may become stale.

```bash
make docs:check
```

If stale, run `make docs` to update. Also check whether hand-written
sections (Overview, Key Files) need updating to reflect the API changes.

To find packages that are missing READMEs entirely:

```bash
find internal cmd -type d -not -path '*/pb' | while read dir; do
  [ -f "$dir/README.md" ] || echo "missing: $dir/README.md"
done
```

## Generated Section Reference

| Marker | Data Source | Used In |
|--------|-----------|---------|
| `exported-api` | `go doc` output | Package READMEs |
| `dependencies` | `go list -json` imports (internal only) | Package + cmd READMEs |
| `used-by` | Inverted dependency graph | Package READMEs |
| `package-map` | All packages with doc strings + file counts | Top-level README |
| `tool-catalog` | Tool constants from `internal/claude/tools.go` | Top-level README |
