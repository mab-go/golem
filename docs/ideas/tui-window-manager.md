# TUI Window Manager

| Field   | Value     |
|---------|-----------|
| Status  | sketch    |
| Created | 2026-04-27 |

## Motivation

The current golem-tui has a fixed 3-pane layout (Mind, Tabbed logs,
Chat). It works, but the operator can't adapt the workspace to the
task at hand. Debugging a pathfinding issue wants the sidecar log
and Mind side by side with a screenshot floating on top. Watching
the bot socialize wants chat maximized with logs minimized. Right
now you get one layout, and it's the same regardless of what you're
doing.

The proposal: make golem-tui feel like a tiny operating system.
Free-floating, mouse-draggable panes with z-order, snap-to-grid for
quick tiling, and the ability to layer transient overlays
(screenshots, deep-think output, help) on top of the working set.

The reference point is tuios -- it proves the interaction model
works in a terminal: drag to move, drag edges to resize, click to
focus, snap zones at the screen edges. We're not trying to clone
it; we're stealing the interaction patterns and applying them to a
purpose-built mission control surface.

## Goals

- Operator can move and resize panes with the mouse
- Panes can overlap; clicking raises the clicked pane
- Snap-to-grid for fast tiling without manual pixel-pushing
- Transient overlays (screenshot, help, deep-think) render above
  the workspace and dismiss cleanly
- The current 3-pane arrangement remains achievable as a saved
  layout (no functional regression)

## Non-Goals

- BSP tiling. The user explicitly does not want a tiling tree;
  free-floating with snap is the target.
- Workspaces / multiple desktops. One workspace is enough.
- Embedded shell sessions or PTYs. golem-tui is not a multiplexer.
- Replacing the existing pane content. We are changing the
  *layout system*, not what the panes display.

## Approach

Build it ourselves. tuios's actual window manager (drag, focus,
compositor) lives in `internal/app/` (~623 KB) and is fused with
their terminal emulator and PTY machinery -- not extractable. The
only cleanly-vendor-able piece (`internal/layout/`, BSP tiling
math) is exactly what we don't want. So no vendoring; just build
the pieces we need.

Four pieces, roughly:

### 1. Window primitive (~50 lines)

```
type Window struct {
    ID       string
    Pane     Pane    // existing Pane interface
    X, Y     int
    W, H     int
    Z        int     // z-order
    Focused  bool
    Snapped  SnapZone // none | left-half | right-half | top-left | ...
}
```

The existing `Pane` interface (`pane.go`) is already
position-agnostic; it just needs a position alongside its size.

### 2. Mouse + drag state machine (~150 lines)

Bubble Tea v2 emits `tea.MouseMsg` with X/Y and button state. The
state machine tracks:

- Idle -> mouse click -> hit-test -> raise focused window
- Drag on title bar -> move window with cursor
- Drag on edge/corner -> resize
- Release near snap zone -> snap to grid cell

Hit-testing walks windows in reverse z-order; first match wins.

### 3. Z-order compositor (~50 lines, thin wrapper)

Use `lipgloss/v2`'s built-in compositor:

```go
layers := make([]*lipgloss.Layer, 0, len(windows))
for _, w := range windows { // ascending z-order
    l := lipgloss.NewLayer(w.Pane.View()).
        X(w.X).Y(w.Y).Z(w.Z)
    layers = append(layers, l)
}
canvas := lipgloss.NewCanvas(width, height)
canvas.Compose(lipgloss.NewCompositor(layers...))
out := lipgloss.Sprint(canvas.Render())
// hand `out` to tea.View.SetContent
```

`lipgloss.Layer` already supports hit-testing, which the drag
state machine in piece 2 will use to decide which window is
under the cursor.

Optimization (later): cache each pane's parsed `*uv.Buffer` and
re-stamp on geometry changes only, instead of re-parsing the
ANSI string every frame. tuios does this for unfocused panes.

### 4. Snap targets (~50 lines)

Define snap zones as regions: screen edges (left half, right half,
top half, bottom half), corners (quarters), full-screen. While
dragging, if the cursor enters a snap zone, show a translucent
preview rectangle. On release, the window jumps to the snapped
geometry.

Total estimate: ~300 lines (compositor reduced from ~200 to ~50
since lipgloss/v2 does the heavy lifting). No new dependencies --
lipgloss/v2 is already a direct dep, and ultraviolet is already
indirect.

## Interaction with Existing Code

The existing `Pane` interface (`cmd/golem-tui/pane.go`) is well
abstracted -- it doesn't know about position, just size. The
refactor concentrates in:

- `cmd/golem-tui/layout.go` -- replace the fixed 3-slot system
  with a window manager
- `cmd/golem-tui/model.go` -- mouse message routing, replace
  lipgloss-join rendering with compositor output
- New file: `cmd/golem-tui/wm.go` (or a `wm/` subpackage) -- the
  window manager itself

The pane implementations themselves (mindpane.go, chatpane.go,
etc.) should not need to change. Adding `SetPosition(x, y int)` to
the `Pane` interface is the only contract change.

## Default Layout

On first launch, place windows in the same arrangement as today:
Mind on the left (45% width), Tabbed top-right (65% of right
column), Chat bottom-right. The user can then drag/snap from
there. Save the last layout to disk and restore on next launch.

## Open Questions

These need follow-up passes before this is a buildable plan:

1. ~~**Bubble Tea version.**~~ Resolved 2026-04-27: golem-tui is on
   `charm.land/bubbletea/v2 v2.0.6` with Lipgloss v2 and Bubbles v2.
   First-class mouse support is available -- no migration needed.

2. ~~**ultraviolet vs roll-our-own.**~~ Resolved 2026-04-27: use
   the layered compositor that ships in `charm.land/lipgloss/v2`
   (which we already depend on directly). Specifically:
   `lipgloss.Layer` (with `.X(int)`, `.Y(int)`, `.Z(int)` and
   hit-testing), `lipgloss.NewCompositor(layers...)`, and
   `lipgloss.Canvas`. This is the same API tuios uses for its
   window compositor.

   Why: lipgloss/v2's compositor is built on top of
   `github.com/charmbracelet/ultraviolet`, which is already in
   our `go.sum` as an indirect dependency. No new modules added.
   Rolling our own ANSI parser + cell buffer would be ~1000-1500
   LoC of correctness-critical code (SGR sequences, truecolor,
   wide-char East Asian width edge cases) for no benefit.

   If we ever need to drop below the lipgloss API (e.g. for
   per-cell hooks during focus highlighting), `ultraviolet`'s
   `Buffer` + `StyledString` are the next layer down -- still
   already in our module graph. Avoid `uv.Terminal` /
   `uv.TerminalScreen`; Bubble Tea owns the terminal.

   Caveat: ultraviolet has no version tags and warns about API
   churn. This is the same risk profile we already accept by
   running on Bubble Tea v2 / Lipgloss v2 (both pinned via
   pseudo-versions today).

3. ~~**Snap behavior.**~~ Resolved 2026-04-27: preview-then-commit.
   While dragging, the window itself follows the cursor. When the
   cursor enters a snap zone, a translucent preview rectangle
   shows where the window will land. On mouse release the window
   jumps to the snapped geometry. Magnet-style edge zones (left
   half, right half, top half, bottom half, four corners,
   full-screen). Preview rendering style (dim border vs box
   characters vs color overlay) is a follow-up implementation
   detail.

4. ~~**Keyboard parity.**~~ Resolved 2026-04-27: mouse-only for
   the initial implementation. The window manager API must not
   paint us into a corner -- keep window operations
   (move/resize/raise/snap) as named methods on the window manager
   so a future keyboard binding layer can call them without
   refactoring.

5. ~~**Modal overlays.**~~ Resolved 2026-04-27: three-tier
   rendering.
   - **Window layer** (the workspace): everything else --
     command input, screenshots, deep-think output, expanded log
     entries -- becomes a regular floating window with the same
     drag/snap/focus rules as content panes.
   - **Chrome layer** (always-on-top, anchored): the status bar
     stays where it is today. Not a window. Renders above the
     window layer.
   - **Modal layer** (blocks everything): the help screen is a
     true modal. While open, all input goes to the modal; any
     key or click dismisses it and returns control to the
     workspace. Renders above the chrome layer.
   Implication: the existing `cmdInput` migrates from "anchored
   above status bar" to "a small floating window." Default
   position TBD (likely centered or near the focused pane).

6. ~~**Migration path.**~~ Resolved 2026-04-27: develop on a
   feature branch and merge when it's ready. No experimental flag
   or parallel layout. The current 3-pane layout will be
   reproduced as the default startup arrangement (see "Default
   Layout" above), so users do not lose what they had.

7. ~~**Layout persistence.**~~ Resolved 2026-04-27:
   - Persist to `~/.config/golem/tui-layout.json` (XDG-compliant,
     matches the existing `GOLEM_*` config conventions). Per-user,
     not per-project.
   - On exit, save the current arrangement as "last".
   - On startup, restore "last" if present, else fall back to the
     default preset.
   - Ship named presets the user can cycle through with a
     hotkey: `default` (current 3-pane layout reproduced),
     `debug` (Mind + sidecar log + screenshot floating), `chat`
     (chat maximized, logs minimized). User-defined presets are
     a follow-up.

8. ~~**Resize behavior with content.**~~ Resolved 2026-04-27:
   preview-then-commit on resize, mirroring the snap behavior
   from Q3.

   **The cost.** Tracing the existing pane code:
   `SetSize(w, h)` is O(1) on every pane (just stores dimensions
   and sets `dirty = true`). The expensive work happens in
   `View()` when the dirty flag is checked: for logPane
   (10000-line ring buffer), chatPane (2000 lines), eventsPane /
   sidecarLogPane / serverLogPane (5000 lines each), and
   mindPane (200 entries with width-dependent separators), the
   pane re-joins the buffer and runs `ansi.Wrap` on every line.
   Wrapping is not cached -- each dirty `View()` rewraps from
   scratch.

   **The decision.**
   - **Moving a window**: live. Move only changes X/Y, never
     calls `SetSize`. Smooth always.
   - **Resizing a window**: preview-then-commit. While dragging
     an edge or corner, render a translucent outline at the
     in-progress geometry. The pane keeps its current size and
     content. Only on mouse release does the WM call `SetSize`
     on the pane, triggering exactly one rewrap.

   This mirrors what most desktop WMs do for slow-rendering
   content, never pays the rewrap cost more than once per drag
   gesture, and avoids the stuttering that 30-60 fps rewraps of
   a 10k-line buffer would cause.

   Future optimization (not required): cache wrapped output
   keyed by `(width, content_hash)` so that toggling between
   common sizes is free. Probably not worth doing until we
   measure pain.

## Next Steps

All eight open questions are resolved.

1. Promote this doc into a real implementation plan in
   `docs/plans/`. The plan should break the work into milestones
   (window primitive + compositor first, then mouse drag, then
   snap, then layout persistence, then preset cycling) so we can
   merge incrementally on the feature branch.
2. Cut the feature branch and start.
