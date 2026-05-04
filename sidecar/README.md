# sidecar

The Mineflayer bot sidecar that gives the Go agent a physical presence in
Minecraft. It exposes the full game interface over gRPC and has no knowledge
of Claude or the Anthropic API -- the boundary is absolute.

## Overview

The sidecar is a Node.js/TypeScript process that runs alongside the Go agent.
The Go agent speaks gRPC to port 50051; the sidecar translates those calls
into Mineflayer bot actions and streams game events back. It handles the
entire Minecraft session lifecycle: connecting, reconnecting on disconnection,
and maintaining a prismarine-viewer page for screenshot capture.

All Mineflayer plugin loading, pathfinding, auto-eat, and bot-lifecycle events
are managed here. The Go agent never touches the Minecraft protocol directly.

## Startup Sequence

`src/index.ts` starts the process in order:

1. Connect bot (`src/bot/connection.ts`) -- blocks until first spawn
2. Register event listeners (`src/bot/events.ts`)
3. Start autopilot (`src/bot/autopilot.ts`)
4. Start prismarine-viewer (`src/screenshot/viewer.ts`)
5. Start gRPC server on port 50051 (`src/grpc/server.ts`)

On bot disconnect, steps 1-4 repeat automatically via the reconnect loop in
`connection.ts`.

## Directory Map

| Directory          | Purpose                                              |
| ------------------ | ---------------------------------------------------- |
| `src/bot/`         | Bot lifecycle, events, perception, autopilot         |
| `src/bot/actions/` | Tiered action handlers (tier 0-3) wired to gRPC RPCs |
| `src/grpc/`        | gRPC server and generated protobuf types             |
| `src/screenshot/`  | Playwright screenshot capture and viewer management  |

## Development Commands

```bash
npm install          # Install dependencies (also runs patch-viewer.cjs)
npm run dev          # Run with tsx (no build step)
npm run build        # Compile TypeScript to dist/
npm start            # Run compiled output
npm run format       # Run Prettier
```

Generate proto bindings (run from repo root):

```bash
make proto           # Regenerates src/grpc/generated/ from proto/minecraft.proto
```

## See Also

- [src/bot/actions/](src/bot/actions/) -- tier architecture and action handlers
- [src/bot/](src/bot/) -- connection lifecycle, events, perception, autopilot
- [../proto/minecraft.proto](../proto/minecraft.proto) -- gRPC contract (source
  of truth)
