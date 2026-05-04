# Plan: TUI Window Manager

| Field   | Value      |
|---------|------------|
| Status  | ready      |
| Created | 2026-04-27 |
| Source  | [docs/ideas/tui-window-manager.md](../ideas/tui-window-manager.md) |

## Context

The current golem-tui has a fixed 3-pane layout (Mind, Tabbed
logs, Chat) hardcoded in `layout.go`. It works but cannot adapt
to the operator's task. We want to replace it with a window
manager: free-floating panes with mouse drag, snap-to-grid, and
overlapping windows with z-order. The reference point is tuios's
interaction model -- not a clone, just the patterns.

All eight design questions from the source idea doc are
resolved. This plan executes that design.

## Goals

- Operator can move and resize panes with the mouse
- Panes overlap; clicking raises the clicked pane to top z
- Snap-to-grid edge zones for fast tiling without pixel-pushing
- Three-tier rendering: window layer / chrome (status bar) /
  modal (help)
- Layout state persists across sessions; named presets cycle
  via hotkey
- Default startup layout reproduces today's 3-pane arrangement
  (no functional regression)
- WM API kept future-proof so a keyboard binding layer can be
  added later without refactoring

## Non-Goals

- BSP tiling
- Multiple workspaces
- Embedded shell sessions / PTYs
- Replacing what panes display (only changing the layout system)
- Keyboard window controls in this iteration (mouse-only;
  keyboard parity is a follow-up)

## Approach Summary

- Compositor: `lipgloss/v2`'s `Layer` + `NewCompositor` +
  `Canvas`. Already a direct dependency. Zero new modules.
- Window primitive: thin struct wrapping the existing `Pane`
  interface with X/Y/W/H/Z and focus state.
- Mouse: Bubble Tea v2 `tea.MouseMsg` for hit-test, drag, snap.
- Resize: preview-then-commit (translucent outline during drag,
  one `SetSize` call on release) to avoid rewrap thrashing.
- Move: live (X/Y only, no `SetSize` cost).
- Modal: help screen renders above everything and traps input.

Total scope: ~300 lines of new/modified code, mostly in
`cmd/golem-tui/`.

## Critical Files

- `cmd/golem-tui/pane.go` -- `Pane` interface gets a
  `SetPosition(x, y int)` method
- `cmd/golem-tui/layout.go` -- gutted; replaced by the WM
- `cmd/golem-tui/model.go` -- mouse routing, three-tier render
- `cmd/golem-tui/helpoverlay.go` -- modal input trap
- `cmd/golem-tui/cmdinput.go` -- migrates from anchored to
  floating window
- New: `cmd/golem-tui/wm.go` -- window manager
- New: `cmd/golem-tui/wm_persist.go` -- layout JSON load/save
- New: `cmd/golem-tui/wm_test.go` -- unit tests for hit-test,
  snap zones, drag math

## Milestones

Each milestone is a shippable step on the feature branch.
Every milestone must pass the project's verification gate
(`make fmt build test lint cyclo`) before the next one starts.

### M1: Window primitive + compositor skeleton

**Goal:** Replace the fixed 3-slot layout with a window manager
that produces visually identical output to today.

**Deliverables:**
- `Pane` interface gains `SetPosition(x, y int)` (trivial impl
  in `scrollableViewport`)
- New `wm.go` with:
  - `Window` struct (id, pane, x, y, w, h, z, focused)
  - `WindowManager` struct (windows slice, focused id)
  - `Render(width, height int) string` method that walks
    windows in z-order, wraps each `pane.View()` in a
    `lipgloss.Layer` at its position, and composes via
    `lipgloss.NewCompositor`
- `layout.go` gutted to a thin shim that delegates to the WM
- `model.go` calls `wm.Render(...)` instead of the old
  `lipgloss.JoinVertical` flow for the workspace area
- Default startup arrangement: Mind (left 45%, full height),
  Tabbed (right top, 65%), Chat (right bottom, 35%) -- the
  same proportions as today
- Status bar and cmdInput still anchored, help still as today
  (M6 changes those)

**Verification:**
- TUI launches, looks identical to current TUI
- Tab key still cycles focus among the three windows
- All existing pane shortcuts still work
- `make fmt build test lint cyclo` passes

**Out of scope:** mouse, drag, snap, modal layer. Just the new
internal model rendering today's static arrangement.

### M2: Mouse focus

**Goal:** Click a window to raise and focus it.

**Deliverables:**
- Bubble Tea v2 `WithMouseAllMotion()` enabled in `main.go`
  (or whatever the v2 equivalent is)
- `model.Update` routes `tea.MouseMsg` to the WM
- WM hit-test walks windows in reverse z-order; first match wins
- Click on a window: bumps that window's z above all others,
  sets focused flag, clears focus elsewhere
- Click on empty canvas: no change
- Tab key still works (now equivalent to clicking the next
  window)

**Verification:**
- Click between Mind / Tabbed / Chat -- focus border moves
- Click on Mind while Chat is focused -- Mind raises (matters
  when we get to overlapping windows in M3)
- Test cases in `wm_test.go` for hit-test edge cases (clicks
  on borders, on overlap regions)

### M3: Mouse move

**Goal:** Drag a window's title bar to move it.

**Deliverables:**
- WM tracks drag state: idle / dragging-move
- Mouse-down on title bar (top row of border) starts a move
  drag, captures the click offset within the title
- Mouse-motion updates the dragged window's X/Y to follow the
  cursor (live; no preview)
- Mouse-up ends the drag
- Constraints: clamp window position so at least the title bar
  stays on screen (don't let users hide windows off-edge)
- The dragged window is always rendered above unfocused
  windows during the drag (z bump)

**Verification:**
- Drag Mind from left side to center; it follows smoothly
- Drag a window partway off-screen; it clamps at the edge
- During a drag, the moved window draws on top
- Releasing a drag commits the new position to layout state

### M4: Mouse resize with preview

**Goal:** Drag a window edge or corner to resize, with preview
rectangle and commit-on-release.

**Deliverables:**
- Hit-test recognizes 8 resize regions (4 edges + 4 corners)
  on a window's outer border
- Mouse-down on a resize region starts a resize drag
- During drag: render a translucent rectangle at the
  in-progress geometry; do NOT call the pane's `SetSize`
- Mouse-up: call `SetSize(newW, newH)` and `SetPosition` once
  with final geometry; clear preview
- Constraints: enforce a minimum window size (e.g. 20x6) to
  prevent unusable windows
- Translucent preview style: dim border using box-drawing
  characters (subject to refinement; whatever looks decent
  on the user's terminal is fine for v1)

**Verification:**
- Drag the right edge of Mind outward; preview rectangle
  follows the cursor; pane content does not reflow during
  drag; on release, content rewraps once at the new width
- Try to resize below minimum -- preview clamps
- logPane resize is smooth (10k-line rewrap happens once)

### M5: Snap-to-grid

**Goal:** Magnet zones at screen edges and corners that snap
windows to half/quarter/full screen.

**Deliverables:**
- Snap zones defined as rectangles at the edges and corners of
  the canvas (e.g. a 2-row strip at each edge, 4x2 corners)
- During a move drag (M3), if the cursor enters a snap zone,
  show a translucent preview at the snapped target geometry
- During a resize drag (M4), no snap (resizing is explicit
  geometry-setting)
- Snap targets:
  - Left edge -> left half
  - Right edge -> right half
  - Top edge -> top half
  - Bottom edge -> bottom half
  - Top-left / top-right / bottom-left / bottom-right corners
    -> quarter-screen
  - Optional: drag-to-top-center -> full-screen (TBD by feel)
- Mouse-up while in a snap zone: commit snapped geometry, mark
  the window's `Snapped` field; mouse-up outside any zone:
  commit the cursor-position geometry
- Snapped windows tracked so we can later "un-snap" by drag

**Verification:**
- Drag Mind to the right edge -- preview shows right half --
  release -- Mind snaps to right half
- Drag to a corner -- preview shows quarter
- Drag a snapped window away from its zone -- it un-snaps
  back to free-floating mode
- Snap previews use the same visual treatment as resize
  previews from M4

### M6: Three-tier rendering (modal + chrome)

**Goal:** Migrate help screen to a true modal and cmdInput to a
floating window. Status bar stays anchored chrome.

**Deliverables:**
- Render pipeline becomes:
  1. WM canvas (windows in z-order)
  2. Anchored chrome (status bar) overlaid at the bottom
  3. Modal layer (help screen) overlaid on top, full-screen,
     when active
- Help screen: while open, ALL key/mouse input dismisses it
  and routes nothing to the WM
- cmdInput: becomes a floating window (small, ~60 chars wide,
  3 rows tall, default position centered or near focused
  window). Activated with `:` like today; dismissed by Esc
  or completing a command. Draggable like any other window.
- Status bar: unchanged behavior; rendered above WM canvas
  but below modal

**Verification:**
- `?` opens help, blocks input, any key/click dismisses it
- `:` opens command input as a floating window; it can be
  dragged; submitting or Esc dismisses it
- Status bar visible during workspace use, hidden behind
  modal during help

### M7: Layout persistence + presets

**Goal:** Save and restore layouts; ship named presets.

**Deliverables:**
- Layout state schema (JSON):
  ```
  {
    "version": 1,
    "windows": [
      { "id": "mind", "x": 0, "y": 0, "w": 80, "h": 30,
        "z": 0, "snapped": "" },
      ...
    ]
  }
  ```
- `wm_persist.go` with `Save(path string) error` and
  `Load(path string) (*State, error)`
- Save on graceful exit (and possibly periodically) to
  `~/.config/golem/tui-layout.json` (XDG-respecting; honor
  `XDG_CONFIG_HOME` if set)
- On startup: load `tui-layout.json` if present; otherwise
  apply the `default` preset (today's 3-pane arrangement)
- Hardcoded presets:
  - `default` -- Mind 45%, Tabbed 65%/35% right, Chat right
    bottom (same as today)
  - `debug` -- Mind left half, Sidecar log right top, Server
    log right bottom, screenshot floating top-right
  - `chat` -- Chat full screen, Mind small bottom-left
- Hotkey to cycle presets (suggest `F2` or similar -- check
  it doesn't collide with existing bindings)
- Resilience: if the saved layout's window IDs don't match
  the current pane set (e.g. server log disabled), fall back
  to default preset and log a warning

**Verification:**
- Move some windows, quit, restart -- windows are where you
  left them
- Press preset hotkey -- cycles through default / debug /
  chat
- Delete the layout file -- next startup uses default
- Corrupt the layout file -- next startup falls back to
  default with a logged warning

## Testing Strategy

Bubble Tea is hard to integration-test, but the WM internals
are pure logic and easy to unit-test. Target coverage:

- `wm.go` hit-test, snap-zone detection, geometry math:
  table-driven tests in `wm_test.go`
- Drag state machine: tests for transitions (idle ->
  dragging-move -> committed; idle -> dragging-resize ->
  committed; cancel mid-drag)
- `wm_persist.go` round-trip: serialize a state, deserialize,
  compare
- Compositor output: smoke test that calling `Render(w, h)`
  with N windows returns a string of the expected line count
  (don't try to assert exact ANSI output -- that's brittle)

Manual verification per milestone is captured in each
milestone's "Verification" subsection.

## Verification (project-wide gate)

Per CLAUDE.md, before any milestone is considered done:

```
make fmt
make build
make test
make lint
make cyclo
```

All must pass. New code must not add to the cyclo>10 list.

## Future Work (out of scope for this plan)

- Keyboard window controls (move with hjkl, snap with
  prefix+arrow, etc.) -- the WM API will support this but
  the binding layer is a separate project
- Per-pane caching of wrapped output (only rewrap on
  content-change, not just dimension-change)
- User-defined presets via config file
- Window decorations beyond the simple bordered title
  (custom buttons, badges, status indicators)
- Multiple workspaces / virtual desktops

## Open Risks

- **lipgloss/v2 compositor performance.** Untested at our
  scale. If parsing each pane's ANSI into a Layer every
  frame is too slow, drop down to `uv.Buffer` directly and
  cache the parsed buffer per pane (the optimization noted
  in the idea doc). Mitigation: profile during M1; build
  the cache layer if needed.
- **Mouse coordinate mapping with terminal borders.**
  Bubble Tea v2's mouse coords are 1-indexed terminal
  cells. The WM works in 0-indexed canvas cells. Off-by-one
  bugs are easy here. Mitigation: thorough hit-test tests
  in M2 with edge-case coordinates.
- **Terminal mouse support varies.** Some terminals don't
  report drag motion well, or only report on click/release.
  Verify in the user's primary terminal early; have a
  fallback (M2 click-to-focus must work even without
  motion events).
