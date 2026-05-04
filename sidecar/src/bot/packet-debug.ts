import type { Bot } from "mineflayer"

const ENABLED = process.env.GOLEM_PACKET_DEBUG === "true"
const PREFIX = "[packet-debug]"

const OUTGOING_WATCH = new Set(["block_dig", "block_place", "window_click", "held_item_slot"])

const INCOMING_WATCH = [
  "acknowledge_player_digging",
  "block_change",
  "set_slot",
  "window_items",
  "game_state_change",
  "respawn",
]

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function truncateBuffers(_key: string, value: unknown): unknown {
  if (Buffer.isBuffer(value)) return `<Buffer ${value.length} bytes>`
  if (value instanceof Uint8Array) return `<Uint8Array ${value.length} bytes>`
  return value
}

function log(label: string, data?: unknown): void {
  const ts = new Date().toISOString().slice(11, 23)
  if (data !== undefined) {
    console.log(`${PREFIX} [${ts}] ${label}`, JSON.stringify(data, truncateBuffers))
  } else {
    console.log(`${PREFIX} [${ts}] ${label}`)
  }
}

function logHotbarState(bot: Bot): void {
  const slots: string[] = []
  const start = (bot as unknown as { QUICK_BAR_START: number }).QUICK_BAR_START ?? 36
  for (let i = 0; i < 9; i++) {
    const item = bot.inventory.slots[start + i]
    slots.push(item ? `${item.name}x${item.count}` : "empty")
  }
  log(`HOTBAR [selected=${bot.quickBarSlot}]: ${slots.join(" | ")}`)
  const held = bot.heldItem
  log(`HELD: ${held ? `${held.name}x${held.count} (slot ${held.slot})` : "empty"}`)
}

// ---------------------------------------------------------------------------
// Plugin
// ---------------------------------------------------------------------------

export function packetDebugPlugin(bot: Bot): void {
  if (!ENABLED) {
    console.log(`${PREFIX} Disabled (set GOLEM_PACKET_DEBUG=true to enable)`)
    return
  }

  console.log(`${PREFIX} Enabled -- logging block interaction packets`)

  // -- Initial state (spawn has already fired when loadPlugins runs) --------

  log("GAME_STATE", {
    gameMode: bot.game.gameMode,
    dimension: bot.game.dimension,
    hardcore: bot.game.hardcore,
    position: bot.entity.position,
  })
  logHotbarState(bot)

  // -- Outgoing packet interception ----------------------------------------

  const client = bot._client as {
    write: (name: string, params: unknown) => void
  }
  const originalWrite = client.write.bind(client)

  client.write = (name: string, params: unknown) => {
    if (OUTGOING_WATCH.has(name)) {
      log(`OUT ${name}`, params)
    }
    return originalWrite(name, params)
  }

  // -- Incoming packet listeners -------------------------------------------

  for (const name of INCOMING_WATCH) {
    if (name === "block_change") {
      bot._client.on(name, (data: { location?: { x: number; y: number; z: number } }) => {
        if (!data.location) return
        const pos = bot.entity.position
        const dx = data.location.x - pos.x
        const dy = data.location.y - pos.y
        const dz = data.location.z - pos.z
        if (dx * dx + dy * dy + dz * dz <= 256) {
          log(`IN ${name}`, data)
        }
      })
    } else {
      bot._client.on(name, (data: unknown) => {
        log(`IN ${name}`, data)
      })
    }
  }

  // -- Held item tracking --------------------------------------------------

  bot.on("heldItemChanged" as Parameters<Bot["on"]>[0], () => {
    const held = bot.heldItem
    log(`HELD_ITEM_CHANGED: ${held ? `${held.name}x${held.count} (slot ${held.slot})` : "empty"}`)
  })

  // -- Periodic hotbar snapshot --------------------------------------------

  const hotbarTimer = setInterval(() => {
    logHotbarState(bot)
  }, 10_000)
  hotbarTimer.unref()
}
