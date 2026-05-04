# bot

Core bot modules: connection lifecycle, event broadcasting, perception
reading, and the survival autopilot. These modules collectively manage
the bot's session from first spawn through all reconnects.

## Overview

`connection.ts` owns the bot's lifecycle. It creates the Mineflayer bot,
loads plugins in the correct order (pathfinder before collectblock), and
fires `botReady` / `botLost` events on the shared `botEvents` emitter.
A position-NaN watchdog runs on a separate 1s timer -- Mineflayer's physics
engine skips ticks when position is NaN, so only an external timer can
detect and recover from the dead state. Do not remove this watchdog.

`events.ts` translates Mineflayer events (entitySpawn, health, chat, etc.)
into proto `GameEvent` messages and broadcasts them to all gRPC subscribers.
Subscribers registered via `addSubscriber()` are cleaned up automatically
when their stream closes or errors.

`perception.ts` reads a snapshot of the bot's current state -- vital signs,
surroundings, and inventory -- and maps Mineflayer types to proto response
types. It is called on every agent think cycle.

`autopilot.ts` runs a 2-second tick that equips better armor when found and
flees hostile entities when health is critically low. It also configures the
`mineflayer-auto-eat` plugin. Actions are emitted as autopilot events so
Claude knows what happened between think cycles.

## Key Files

| File            | Purpose                                                                            |
| --------------- | ---------------------------------------------------------------------------------- |
| `connection.ts` | Bot creation, plugin loading, exponential-backoff reconnect, position-NaN watchdog |
| `events.ts`     | Mineflayer -> proto event translation and gRPC subscriber management               |
| `perception.ts` | Snapshot reads: vital signs, surroundings, inventory, light level                  |
| `autopilot.ts`  | 2s survival tick: auto-armor, threat-flee, auto-eat configuration                  |

## See Also

- [actions/](actions/) -- tiered action handlers that operate on the bot
- [../grpc/](../grpc/) -- gRPC server that dispatches to these modules
