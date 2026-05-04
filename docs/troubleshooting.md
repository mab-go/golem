# Troubleshooting

Common issues and their solutions. This document will grow as we encounter
and resolve problems.

------------------------------------------------------------------------

## Sidecar Connection Failures

### Problem

The agent fails to start with an error like:

```
sidecar handshake (GetVitalSigns): rpc error: code = Unavailable
```

### Solution

- Ensure the sidecar is running (`cd sidecar && npm start`)
- Check that the sidecar port matches `GOLEM_SIDECAR_ADDRESS` (default:
  `localhost:50051`)
- If using Docker Compose, ensure the `mineflayer` service is healthy before
  the `golem` service starts

------------------------------------------------------------------------

## Minecraft Server Connectivity

### Problem

The sidecar starts but the bot does not appear in the Minecraft game.

### Solution

- Verify the server is running Minecraft **1.21.9** (version mismatch causes
  silent connection failures)
- Confirm `online-mode=false` in `server.properties` -- the bot uses offline
  authentication
- Check that `MINECRAFT_HOST` and `MINECRAFT_PORT` environment variables match
  the server's actual address (these are read by the sidecar, not the Go agent)
- Look at sidecar logs for connection error details

------------------------------------------------------------------------

## API Key / Authentication Errors

### Problem

The agent starts but fails on the first Claude API call with an authentication
error.

### Solution

- Set `GOLEM_ANTHROPIC_API_KEY` or `ANTHROPIC_API_KEY` in the environment
- Verify the key has access to the required models (Sonnet, Haiku, and Opus)
- Check for typos -- the key should start with `sk-ant-`

------------------------------------------------------------------------

## Position NaN (Mineflayer Bug)

### Problem

The bot becomes unresponsive. Logs may show NaN position values or physics
errors.

### Solution

This is a known Mineflayer bug where the bot's position becomes NaN and physics
stop firing. The sidecar includes a watchdog (`sidecar/src/bot/connection.ts`)
that detects this condition and recovers automatically by forcing a respawn.

If the watchdog is not recovering:
- Restart the sidecar
- Check that the watchdog polling interval has not been modified

------------------------------------------------------------------------

## Content Filter False Positives

### Problem

Claude refuses to execute certain game actions, citing content policy concerns.

### Solution

Golem uses guardrail-safe vocabulary to avoid Anthropic API content filter false
positives. If you encounter a new false positive:

- Use neutral action names: `interact_with_entity(cow, harvest)` not
  `kill(cow)`
- Supported action verbs: `harvest`, `attack`, `feed`, `trade`, `mount`, `shear`
- Check `internal/claude/system_prompt.go` for the persona framing that helps
  avoid filter triggers

------------------------------------------------------------------------

## Pathfinding Failures

### Problem

The bot gets stuck, digs into lava, or wastes inventory blocks on scaffolding
during navigation.

### Solution

All pathfinding must use `safeMovements()` or `digMovements()` from
`sidecar/src/bot/helpers.ts`. These helpers prevent:

- Tunneling into lava
- Using inventory items as scaffolding
- Pathfinding through dangerous terrain

If you see these symptoms, check that no code is constructing raw
`new Movements(bot)` -- always use the safe helpers.
