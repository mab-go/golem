# Setup

Complete walkthrough for getting golem running from a fresh clone.

------------------------------------------------------------------------

## Prerequisites

| Tool | Version | Purpose |
|------|---------|---------|
| Go | 1.26.1+ | Build the Go agent and TUI binaries |
| Node.js | Current LTS | Build and run the Mineflayer sidecar |
| protoc | 3.x+ | Only needed if modifying `proto/minecraft.proto` |
| Docker | Optional | Docker Compose deployment or TUI-managed Minecraft server |

You also need:

- A **Minecraft server** running version **1.21.9** with `online-mode=false`
  (offline/cracked auth). The bot authenticates with a username only.
- An **Anthropic API key** with access to Claude Sonnet, Haiku, and Opus models.

------------------------------------------------------------------------

## 1. Clone and Install Tools

```bash
git clone git@github.com:mab-go/golem.git
cd golem
make setup
```

`make setup` installs project-local Go tools into `./bin/` (golangci-lint,
goimports, gocyclo, protoc plugins) and runs `npm install` in the sidecar
directory.

------------------------------------------------------------------------

## 2. Configure the API Key

The agent needs an Anthropic API key. Set it as an environment variable:

```bash
export GOLEM_ANTHROPIC_API_KEY="sk-ant-..."
```

The agent also checks `ANTHROPIC_API_KEY` as a fallback. You can pass it as a
flag (`--anthropic-api-key`) but environment variables are recommended to avoid
leaking keys in shell history.

------------------------------------------------------------------------

## 3. Minecraft Server

You need a Minecraft 1.21.9 server with offline authentication. Three options:

### Option A: TUI-Managed Docker Server

The TUI can start and manage a Minecraft server container for you:

```bash
./bin/golem-tui --server
```

This pulls the `itzg/minecraft-server` image and creates a container with
sensible defaults. The server data persists in a Docker volume. Use
`--server-remove` to clean up the container on exit, and `--server-remove-data`
to also remove the world data volume.

### Option B: Docker Compose

```bash
docker compose up --build
```

This starts both the Go agent and sidecar. Edit `docker-compose.yml` to point
at your Minecraft server (the compose file does not include one).

### Option C: External Server

If you already have a Minecraft 1.21.9 server, configure the connection:

```bash
export GOLEM_MINECRAFT_HOST="your-server-host"
export GOLEM_MINECRAFT_PORT="25565"
```

Or pass `--minecraft-host` and `--minecraft-port` flags to the agent.

Ensure `online-mode=false` in the server's `server.properties`.

------------------------------------------------------------------------

## 4. Build

```bash
make build
```

This compiles three things:
- `./bin/golem` -- the headless agent binary
- `./bin/golem-tui` -- the TUI binary
- `./sidecar/dist/` -- the compiled TypeScript sidecar

------------------------------------------------------------------------

## 5. Verify the Setup

Run the verification suite to confirm everything compiles and passes.
`make fmt` runs goimports on Go code and Prettier on the sidecar (requires
`sidecar/node_modules` from `make setup`).

```bash
make fmt && make build && make test && make lint && make cyclo
```

All five targets must pass.

------------------------------------------------------------------------

## 6. First Run

See [getting-started.md](getting-started.md) for how to start the sidecar and
agent, interact with the bot, and understand its behavior.

------------------------------------------------------------------------

## Model Overrides

By default, golem uses specific Claude models for each cognitive tier. You can
override any of them:

```bash
export GOLEM_MODEL_PLAYER="claude-sonnet-4-6"             # Gameplay decisions
export GOLEM_MODEL_WRITER="claude-sonnet-4-6"             # Journal/knowledge prose
export GOLEM_MODEL_WORKHORSE="claude-haiku-4-5-20251001"  # Gatekeeper/classification
export GOLEM_MODEL_DEEP="claude-opus-4-7"                 # Strategic advisor
```

See the main [README](../README.md#configuration) for the full configuration
reference.
