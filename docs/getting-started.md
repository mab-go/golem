# Getting Started

This guide covers post-setup operation: starting the system, interacting with
the bot, and understanding its behavior. Complete [setup.md](setup.md) first.

------------------------------------------------------------------------

## Starting the System

Golem is a two-process system. Start the sidecar first, then the agent.

### 1. Start the Sidecar

The sidecar is a Node.js process that connects to the Minecraft server and
exposes a gRPC interface for the Go agent.

```bash
cd sidecar
npm start    # Production mode
# or
npm run dev  # Development mode with tsx (auto-reload)
```

The sidecar listens on port 50051 by default. It connects to the Minecraft
server and joins as the bot user. You should see the bot appear in the game.

### 2. Start the Agent

#### Headless Mode

```bash
./bin/golem serve
# or
make run ARGS="serve"
```

The agent connects to the sidecar, runs the agentic loop, and logs to stderr.

#### TUI Mode

```bash
./bin/golem-tui
```

The TUI manages the sidecar subprocess for you (auto-starts and auto-restarts
it), so you only need to start one process. It provides a multi-pane terminal
interface with:

- **Chat pane** -- live Minecraft chat, including the bot's messages
- **Log pane** -- agent and sidecar logs
- **Events pane** -- classified game events (combat, chat, environment)
- **Mind pane** -- memory state (journal, goals, world knowledge)
- **Status bar** -- connection status, model info, token usage

#### TUI with Managed Server

To have the TUI also manage a Docker Minecraft server:

```bash
./bin/golem-tui --server
```

#### Demo Mode

To explore the TUI without any backend services:

```bash
./bin/golem-tui --demo
```

------------------------------------------------------------------------

## Interacting with the Bot

### In-Game Chat

The bot reads all Minecraft chat messages. Talk to it naturally -- it will
respond in chat. The agent classifies incoming messages and decides whether to
wake up and respond based on relevance.

### Operator Commands

Operator commands are prefixed with `/golem` in Minecraft chat:

| Command | Effect |
|---------|--------|
| `/golem pause` | Pause the agentic loop |
| `/golem resume` | Resume the agentic loop |
| `/golem verbose` | Switch to verbose perception mode |
| `/golem terse` | Switch to terse perception mode |
| `/golem normal` | Switch to standard perception mode |
| `/golem stop` | Emergency stop -- immediately cancels the current action |

### Chatting with the Bot

Just type in Minecraft chat. The bot sees all messages and decides whether to
respond. Messages directed at the bot (mentioning its name or addressed to it)
are more likely to trigger a response. The gatekeeper -- a lightweight Haiku
model -- makes the wake/sleep decision on each perception tick.

------------------------------------------------------------------------

## Understanding Agent Behavior

### The Agentic Loop

The agent follows a continuous cycle:

1. **Perceive** -- Read vital signs, surroundings, inventory, and recent events
   from the sidecar
2. **Think** -- Send the perception context to Claude (Player tier) with the
   full tool catalog
3. **Act** -- Execute any tool calls Claude returns (may be multiple rounds)
4. **Remember** -- Update persistent memory (journal, goals, world knowledge)

Between cycles, a **gatekeeper** (Workhorse tier / Haiku) runs on a fast tick
(default 3s), classifying events and deciding whether the agent should wake up.
This keeps costs low during idle periods -- the expensive Player model only runs
when something interesting happens.

### Wake Signals

The agent wakes from sleep when:

- The gatekeeper detects a relevant event (chat, combat, environment change)
- A heartbeat fires (default 45s) for temporal awareness
- A background task completes or reports progress
- An operator command arrives

### Memory System

The agent maintains five persistent memory files on disk:

| File | Purpose |
|------|---------|
| `journal.md` | Running narrative of experiences and events |
| `goals.md` | Current objectives and priorities |
| `world_knowledge.md` | Learned facts about the world (base locations, resources) |
| `inventory_notes.json` | Structured inventory observations |
| `social.md` | Information about other players |

The agent reads all memory at the start of each agentic cycle and can update
any file via dedicated tools (`write_journal`, `update_goals`, etc.).

### Background Tasks

Tier 2 tools (`gather`, `build_structure`, `farm`, etc.) run as streaming
background tasks. Only one can be active at a time. The agent receives progress
updates and can cancel a running task with `cancel_task`. Tasks have a
configurable timeout (default 10 minutes).

------------------------------------------------------------------------

## Next Steps

- Read the [README](../README.md) for the full configuration reference
- Explore package-level READMEs for architecture details
- See [troubleshooting.md](troubleshooting.md) if you run into issues
