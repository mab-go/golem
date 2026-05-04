# golem

Headless agent binary. Connects to a Mineflayer sidecar and runs the
Perceive -> Think -> Act -> Remember loop powered by Claude.

## Overview

The CLI uses Cobra with two subcommands: `serve` for the main agent
loop and `test-actions` for exercising sidecar RPCs in an integration
test sequence. All configuration flows through Viper with the `GOLEM_`
environment variable prefix.

## CLI Usage

```
golem serve [flags]        # Run the autonomous agent loop
golem test-actions [flags] # Exercise Tier 0/1 action RPCs against the sidecar
```

### Connection Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--sidecar-address` | `localhost:50051` | gRPC address of the Mineflayer sidecar |
| `--minecraft-host` | `localhost` | Minecraft server hostname |
| `--minecraft-port` | `25565` | Minecraft server port |
| `--minecraft-username` | `claude` | Bot username for offline auth |
| `--minecraft-version` | `1.21.9` | Minecraft protocol version |

### Agent Tuning Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--memory-dir` | `./memory` | On-disk memory directory |
| `--perception-format` | `prose` | Perception output format (`prose` or `structured`) |
| `--perception-radius` | `16` | Block radius for surroundings queries |
| `--history-messages` | `80` | Conversation history depth |
| `--perception-tick` | `3s` | Interval between perception snapshots |
| `--heartbeat` | `45s` | Max idle time before forced wake |
| `--gatekeeper-timeout` | `5s` | Timeout for gatekeeper classification |
| `--task-timeout` | `10m` | Max duration for Tier 2 streaming tasks |

### Model Flags

| Flag | Env Override | Description |
|------|-------------|-------------|
| `--model` | `GOLEM_MODEL_PLAYER` | Player tier model (conscious mind) |
| `--model-writer` | `GOLEM_MODEL_WRITER` | Writer tier model (journal/knowledge) |
| `--model-workhorse` | `GOLEM_MODEL_WORKHORSE` | Workhorse tier model (gatekeeper/reflexes) |
| `--model-deep` | `GOLEM_MODEL_DEEP` | Deep tier model (strategic advisor) |
| `--anthropic-api-key` | `GOLEM_ANTHROPIC_API_KEY` | Anthropic API key |

## Dependencies

<!-- BEGIN:generated:dependencies -->

- [`agent`](../../internal/agent/)
- [`claude`](../../internal/claude/)
- [`grpc`](../../internal/grpc/)
- [`pb`](../../internal/grpc/pb/)
- [`logging`](../../internal/logging/)
- [`memory`](../../internal/memory/)
- [`perception`](../../internal/perception/)
- [`publisher`](../../internal/publisher/)
- [`version`](../../internal/version/)

<!-- END:generated:dependencies -->
