# golem-tui

Bubble Tea mission control TUI for the golem agent. Provides a
real-time view of the agent's thoughts, tool execution, game events,
and chat -- plus optional Docker-managed Minecraft server lifecycle.

## Overview

The TUI renders a three-pane layout: the agent's mind (thoughts and
decisions) on the left, tabbed log panes (agent log, sidecar log,
events, and optionally server log) on the upper right, and Minecraft
chat on the lower right. An orchestrator manages subprocess lifecycle
-- it can start a Docker Minecraft server, launch the sidecar with
auto-restart (up to 5 attempts), and run the agent, all from a single
binary.

A demo mode (`--demo`) simulates the full startup sequence and agent
behavior without any external dependencies, useful for UI development
and screenshots.

## CLI Usage

```
golem-tui [flags]    Launch the TUI mission control
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--demo` | `false` | Run with simulated agent (no sidecar/server needed) |
| `--no-agent` | `false` | Start TUI without launching the agent subprocess |
| `--sidecar-dir` | auto-detected | Custom path to sidecar directory |
| `--log-file` | none | Write debug logs to file |
| `--manage-server` | `false` | Start/stop a Docker Minecraft server |
| `--server-image` | `itzg/minecraft-server` | Docker image for managed server |
| `--server-port` | `25565` | Minecraft server port |

All agent tuning flags from `golem serve` are also accepted
(perception, memory, model overrides, timeouts).

### Key Bindings

| Key | Action |
|-----|--------|
| `Tab` / `Shift+Tab` | Cycle through panes |
| `1`-`4` | Switch log tabs |
| `↑` / `↓` | Scroll active pane |
| `q` / `Ctrl+C` | Quit |

## Dependencies

<!-- BEGIN:generated:dependencies -->

- [`agent`](../../internal/agent/)
- [`claude`](../../internal/claude/)
- [`grpc`](../../internal/grpc/)
- [`logging`](../../internal/logging/)
- [`memory`](../../internal/memory/)
- [`perception`](../../internal/perception/)
- [`publisher`](../../internal/publisher/)
- [`version`](../../internal/version/)

<!-- END:generated:dependencies -->

## Key Files

| File | Purpose |
|------|---------|
| main.go | CLI entry point, config builders, demo/real mode dispatch |
| model.go | Top-level Bubble Tea model, resize/key handling, message routing |
| layout.go | Three-slot grid layout (mind 45%, tabbed logs 65%, chat 35%) |
| orchestrator.go | Subprocess management: Docker server, sidecar restart, agent launch |
| docker.go | Docker container lifecycle: inspect, create, stop, cleanup |
| mindpane.go | Agent thought stream and reasoning display |
| chatpane.go | Minecraft chat message display |
| tabbedpane.go | Tab switcher for the right-side log panes |
| bridge.go | gRPC message bridge from publisher events to TUI messages |
| demo.go | Simulated startup sequence and agent actions for demo mode |
| theme.go | Lipgloss color theme and styling constants |
| msgs.go | Bubble Tea message type definitions |

## See Also

- [golem](../golem/) -- headless agent binary (same agent, no UI)
- [publisher](../../internal/publisher/) -- event interface that feeds the TUI
