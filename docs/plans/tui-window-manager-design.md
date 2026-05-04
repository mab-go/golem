# Design: TUI Window Manager

| Field   | Value      |
|---------|------------|
| Status  | ready      |
| Created | 2026-04-27 |
| Source  | [tui-window-manager.md](tui-window-manager.md) |

## Purpose

Companion to the implementation plan. Where the plan describes
*what* and *when*, this doc nails down *exactly how* the API
shapes, types, key bindings, and exact numbers will look. The
goal is that whoever implements M1-M7 has zero ambiguous
decisions to make at code-writing time.

## 1. Window Manager API

The WM is a struct in `cmd/golem-tui/wm.go`. Public methods are
high-level operations that either the mouse handler or a future
keyboard binding layer can call without changing the WM.

```go
package main

import (
    "image"
    "charm.land/lipgloss/v2"
    tea "charm.land/bubbletea/v2"
)

// Window wraps a Pane with WM state.
type Window struct {
    ID       string   // stable identifier for persistence
    Pane     Pane     // existing Pane interface (with SetPosition added)
    Title    string   // shown in the title bar
    X, Y     int      // canvas position (top-left), 0-indexed
    W, H     int      // size in cells
    Z        int      // z-order; higher z renders on top
    Focused  bool     // exactly one window is focused at a time
    Snapped  SnapZone // SnapZoneNone if free-floating
    MinW, MinH int    // minimum size (default 20x6)
}

// SnapZone identifies where a snapped window is anchored.
type SnapZone int

const (
    SnapZoneNone SnapZone = iota
    SnapZoneLeftHalf
    SnapZoneRightHalf
    SnapZoneTopHalf
    SnapZoneBottomHalf
    SnapZoneTopLeft
    SnapZoneTopRight
    SnapZoneBottomLeft
    SnapZoneBottomRight
    SnapZoneFullscreen
)

// HitRegion identifies what part of a window the cursor is over.
type HitRegion int

const (
    HitNone HitRegion = iota
    HitTitle           // top border row -- starts a move drag
    HitEdgeTop
    HitEdgeBottom
    HitEdgeLeft
    HitEdgeRight
    HitCornerTopLeft
    HitCornerTopRight
    HitCornerBottomLeft
    HitCornerBottomRight
    HitBody            // interior of the window -- focus only
)

// DragKind tracks what the active drag is doing.
type DragKind int

const (
    DragNone DragKind = iota
    DragMove
    DragResize  // the corner/edge is captured in dragResizeRegion
)

// WindowManager owns all windows and the active drag state.
type WindowManager struct {
    windows  []*Window
    canvasW  int
    canvasH  int
    focused  string  // id; "" if no window has focus

    // drag state (zero-valued when idle)
    dragKind        DragKind
    dragWindow      string         // id of window being dragged
    dragAnchor      image.Point    // cursor offset within window at drag start
    dragResizeRegion HitRegion     // which edge/corner during a resize drag
    dragPreview     image.Rectangle // in-progress geometry; rendered as overlay
    dragSnapZone    SnapZone       // active snap target during a move drag
}

// --- Lifecycle ---

func NewWindowManager() *WindowManager
func (wm *WindowManager) SetCanvasSize(w, h int)
func (wm *WindowManager) AddWindow(w *Window)
func (wm *WindowManager) RemoveWindow(id string)
func (wm *WindowManager) Window(id string) *Window
func (wm *WindowManager) Windows() []*Window // ordered by z ascending

// --- High-level operations (callable by mouse OR keyboard) ---

func (wm *WindowManager) Move(id string, x, y int)
func (wm *WindowManager) Resize(id string, w, h int)
func (wm *WindowManager) Raise(id string)               // bump z to top
func (wm *WindowManager) Focus(id string)               // also raises
func (wm *WindowManager) Snap(id string, zone SnapZone) // computes geometry from canvas size
func (wm *WindowManager) Unsnap(id string)              // clears Snapped flag

// --- Mouse-driven low-level state machine ---

func (wm *WindowManager) HitTest(x, y int) (id string, region HitRegion)
func (wm *WindowManager) HandleMouse(msg tea.MouseMsg) tea.Cmd

// HandleMouse internally manages BeginDrag / UpdateDrag / EndDrag /
// CancelDrag. Those are unexported because only mouse drives them.
// Keyboard parity uses Move/Resize/Snap directly.

// --- Rendering ---

// Render produces the composited workspace string sized to the canvas.
// Called by model.View() for the workspace area only (chrome and modal
// are rendered separately).
func (wm *WindowManager) Render() string

// RenderWithPreview is Render plus the drag-preview overlay (translucent
// rectangle at dragPreview, if a drag is in progress). Used during M4+.
func (wm *WindowManager) RenderWithPreview() string
```

**Rationale.**

- All geometry uses 0-indexed canvas cells. No coordinate
  systems mixed.
- `Move/Resize/Snap` are the keyboard-future-proof surface.
  When we add keyboard window controls later, they call
  these directly. No refactor needed.
- `HitTest` returns both window id and hit region; the mouse
  handler decides what to do (focus only, start move drag,
  start resize drag).
- `HandleMouse` swallows all mouse logic so `model.Update`
  has one clean dispatch point.
- The compositor uses `lipgloss.NewLayer(pane.View()).X(x).Y(y).Z(z)`
  per window and feeds them all to `lipgloss.NewCompositor`.

## 2. Bubble Tea v2 Mouse Event Surface

Verified against `charm.land/bubbletea/v2 v2.0.6` source.

**Message types** (all embed `tea.Mouse{X, Y int; Button MouseButton; Mod KeyMod}`):

```go
type MouseClickMsg   tea.Mouse  // button down
type MouseReleaseMsg tea.Mouse  // button up
type MouseMotionMsg  tea.Mouse  // cursor moved (with or without button)
type MouseWheelMsg   tea.Mouse  // wheel up/down/left/right
```

All implement the `tea.MouseMsg` interface. We can dispatch on
either the concrete type or the interface.

**Coordinates:** 0-indexed terminal cells. (0,0) is the top-left.

**Buttons we care about:**
- `tea.MouseLeft` -- primary drag button
- `tea.MouseWheelUp` / `tea.MouseWheelDown` -- viewport scroll
- (Right and middle are unused for now but the type system
  supports them for free.)

**Enabling motion reporting in v2** (different from v1!):

```go
v := tea.NewView(content)
v.MouseMode = tea.MouseModeCellMotion
return v
```

We use `MouseModeCellMotion` rather than `MouseModeAllMotion`.
Cell motion only reports motion *while a button is held* (i.e.
during a drag), which is all we need. AllMotion would emit a
message on every cursor move and add noise.

**Note:** v2 deprecated the `tea.WithMouseCellMotion()`
ProgramOption; the configuration moved to the `View` struct.
Don't reach for the old API.

**Drag state machine in our handler:**

```
idle
  + MouseClickMsg(left, in HitTitle)     -> dragMove(window, anchor=cursor-window)
  + MouseClickMsg(left, in HitEdge/Corner)-> dragResize(window, region)
  + MouseClickMsg(left, in HitBody)      -> Focus(window); idle
  + MouseClickMsg(left, in HitNone)      -> idle (no-op)
  + MouseWheelMsg                        -> route to focused pane
dragMove
  + MouseMotionMsg                       -> update window X/Y; check snap zones
  + MouseReleaseMsg                      -> commit (snap if in zone, else free); idle
dragResize
  + MouseMotionMsg                       -> update dragPreview rectangle
  + MouseReleaseMsg                      -> SetSize on pane; idle
```

The state-machine table belongs in `wm_test.go` once we
implement it.

## 3. Pane Interface Change

```go
// pane.go
type Pane interface {
    Init() tea.Cmd
    Update(msg tea.Msg) (Pane, tea.Cmd)
    View() string
    Title() string
    Focused() bool
    SetFocused(bool)
    SetSize(width, height int)
    SetPosition(x, y int)  // NEW
}
```

**Semantics of `SetPosition`:**

- Coordinates are canvas cells, 0-indexed.
- Panes do NOT use the position to render -- the WM positions
  the pane's `View()` output via `lipgloss.Layer.X(x).Y(y)`.
  `SetPosition` exists for panes that need to know where they
  are on screen (e.g. for a future tooltip, or for absolute
  cursor positioning).
- Default implementation is a no-op store on `scrollableViewport`:

```go
// pane.go (added to scrollableViewport)
func (sv *scrollableViewport) SetPosition(x, y int) {
    sv.x = x
    sv.y = y
}
```

This adds two `int` fields to `scrollableViewport` and one method
to each pane that embeds it. Trivial; no behavior change.

## 4. Default Arrangement (Exact Numbers)

From `cmd/golem-tui/layout.go` lines 70-89:

| Region | Width | Height |
|--------|-------|--------|
| Mind   | `canvasW * 45 / 100` | `canvasH` |
| Right  | `canvasW - mindW`    | `canvasH` |
| Tabbed | `rightW`             | `rightH * 65 / 100` |
| Chat   | `rightW`             | `rightH - tabbedH` |

**Exact percentages: 45/55 for Mind/right column, 65/35 for
Tabbed/Chat within the right column.**

No minimum-size constraints exist today. The WM will introduce
`MinW=20, MinH=6` as defaults (configurable per window) so users
cannot resize a window into uselessness.

**Default preset (M7 ships this):**

```
Window  ID         X                   Y                   W                   H
mind    "mind"     0                   0                   canvasW*45/100      canvasH
tabbed  "tabbed"   canvasW*45/100      0                   canvasW-mindW       canvasH*65/100
chat    "chat"     canvasW*45/100      canvasH*65/100      canvasW-mindW       canvasH-tabbedH
```

**Fallback behavior preservation** (from existing
`distributeSpace` semantics in layout.go:51-74): if a pane is
hidden (closed window in WM terms), the remaining panes do NOT
auto-redistribute. The user moves/resizes them manually. This is
a behavior change worth flagging in M7 -- if it's surprising in
practice, we can add a "fill empty space" command later.

## 5. Hotkey Assignments

**Existing bindings (do not collide with these):**

| Key            | Action                       |
|----------------|------------------------------|
| `q`, `ctrl+c`  | Quit                         |
| `1`, `2`, `3`  | Toggle Mind / Tabbed / Chat  |
| `tab`          | Focus next window            |
| `[`, `shift+tab` | Previous tab               |
| `]`            | Next tab                     |
| `g`, `home`    | Scroll top (focused pane)    |
| `G`, `end`     | Scroll bottom (focused pane) |
| arrows, pgup/dn | Scroll                      |
| `?`            | Help overlay                 |
| `:`            | Server command input         |
| `enter` (cmd input) | Submit command          |
| `esc` (cmd input)   | Cancel                  |
| `up`/`down` (cmd input) | History                |

**All F-keys (F1-F12) are unbound.** All of `<`, `>`, `,`, `.`
are unbound.

**New bindings introduced by the WM:**

| Key | Action | Milestone |
|-----|--------|-----------|
| `<` | Cycle to previous preset | M7 |
| `>` | Cycle to next preset     | M7 |
| `F2` | Save current layout as "last" (manual save; auto-save also runs on exit) | M7 |

**Why `<` and `>`:** they read as backward/forward chevrons,
echo vim/tmux conventions for navigation, are easy to type, and
won't collide with any existing pane-content navigation.

**Reserved for future keyboard window controls (don't bind in
this iteration):**

- `ctrl+arrow` -- move focused window
- `shift+arrow` -- resize focused window
- `ctrl+shift+arrow` -- snap focused window in that direction
- `F3` / `F4` -- raise / lower focused window in z-order

These are listed so the WM API design above stays consistent
with them. The actual bindings ship in a separate keyboard-parity
follow-up.

## 6. Layout JSON Schema (preview, finalized in M7)

```go
// wm_persist.go
type LayoutState struct {
    Version int                  `json:"version"` // currently 1
    Windows []WindowState        `json:"windows"`
}

type WindowState struct {
    ID      string `json:"id"`      // matches Window.ID
    X       int    `json:"x"`
    Y       int    `json:"y"`
    W       int    `json:"w"`
    H       int    `json:"h"`
    Z       int    `json:"z"`
    Snapped string `json:"snapped"` // SnapZone name; "" for none
    Hidden  bool   `json:"hidden"`  // window present but not rendered
}
```

Schema versioning is explicit so future changes can migrate
gracefully.

Path: `$XDG_CONFIG_HOME/golem/tui-layout.json`, default
`~/.config/golem/tui-layout.json`.

## Open Items Deferred to Implementation

These are intentionally not pinned here because they are
better decided by feel during M4-M5:

- Translucent preview rendering style (which box-drawing
  characters, dim attribute vs color overlay)
- Exact snap-zone cell ranges (e.g. is the left-edge zone 1
  column or 3 columns wide?)
- Cursor offset behavior when a snap preview commits (does
  the window jump immediately, or animate?)
- Resize region cell ranges (specifically: are corners 1x1
  or larger? does the title bar overlap with the top edge
  for resize, or only with the corners?)
