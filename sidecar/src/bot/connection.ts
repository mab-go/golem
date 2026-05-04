import { EventEmitter } from "node:events"
import { createBot as createMineflayerBot, type Bot } from "mineflayer"
import pathfinderPkg from "mineflayer-pathfinder"
import toolPkg from "mineflayer-tool"
import collectBlockPkg from "mineflayer-collectblock"
import { loader as autoEat } from "mineflayer-auto-eat"
import { emitAutopilotEvent } from "./events.js"
import { packetDebugPlugin } from "./packet-debug.js"

const { pathfinder } = pathfinderPkg
const { plugin: toolPlugin } = toolPkg
const { plugin: collectBlock } = collectBlockPkg

// ---------------------------------------------------------------------------
// Configuration
// ---------------------------------------------------------------------------

const config = {
  host: process.env.MINECRAFT_HOST ?? "localhost",
  port: parseInt(process.env.MINECRAFT_PORT ?? "25565", 10),
  username: process.env.MINECRAFT_USERNAME ?? "claude",
  version: process.env.MINECRAFT_VERSION ?? "1.21.9",
  auth: (process.env.MINECRAFT_AUTH ?? "offline") as "offline" | "microsoft",
  profilesFolder: process.env.MINECRAFT_PROFILES_FOLDER || undefined,
}

// ---------------------------------------------------------------------------
// Module state
// ---------------------------------------------------------------------------

let bot: Bot | null = null
let pendingBot: Bot | null = null
let reconnecting = false
let intentionalQuit = false
let lastGoodPos: { x: number; y: number; z: number } | null = null
let posWatchdog: ReturnType<typeof setInterval> | null = null

const BASE_DELAY_MS = 2000
const MAX_DELAY_MS = 60000
const BACKOFF_MULTIPLIER = 2
const JITTER_FACTOR = 0.2
const MAX_INITIAL_ATTEMPTS = 30
const PER_ATTEMPT_TIMEOUT_MS = config.auth === "microsoft" ? 300_000 : 30_000

/** Lifecycle emitter. Fires "botReady" (Bot) and "botLost" (Bot). */
export const botEvents = new EventEmitter()

// ---------------------------------------------------------------------------
// Public API
// ---------------------------------------------------------------------------

/** Returns the current bot instance, or null if not connected. */
export function getBot(): Bot | null {
  return bot
}

/**
 * Creates and connects the mineflayer bot. Retries with exponential backoff
 * if the server is not ready yet. Resolves when the bot fires its first
 * "spawn" event and plugins are loaded.
 */
export async function connectBot(): Promise<Bot> {
  intentionalQuit = false

  for (let attempt = 0; attempt < MAX_INITIAL_ATTEMPTS; attempt++) {
    try {
      return await withTimeout(
        createAndWaitForSpawn(),
        PER_ATTEMPT_TIMEOUT_MS,
        `Connection attempt ${attempt + 1} timed out after ${PER_ATTEMPT_TIMEOUT_MS / 1000}s`,
      )
    } catch (err) {
      // Clean up the bot from the failed attempt.
      if (pendingBot && pendingBot !== bot) {
        try {
          pendingBot.quit("Retrying connection")
        } catch {
          // Ignore cleanup errors.
        }
        pendingBot = null
      }

      if (attempt + 1 >= MAX_INITIAL_ATTEMPTS) {
        throw new Error(
          `Failed to connect after ${MAX_INITIAL_ATTEMPTS} attempts: ${(err as Error).message}`,
        )
      }

      const baseDelay = Math.min(
        BASE_DELAY_MS * Math.pow(BACKOFF_MULTIPLIER, attempt),
        MAX_DELAY_MS,
      )
      const jitter = baseDelay * JITTER_FACTOR * (Math.random() * 2 - 1)
      const delay = Math.round(baseDelay + jitter)

      console.error(
        `[bot] Initial connection attempt ${attempt + 1}/${MAX_INITIAL_ATTEMPTS} failed:`,
        (err as Error).message,
      )
      console.log(`[bot] Retrying in ${(delay / 1000).toFixed(1)}s...`)
      await sleep(delay)
    }
  }

  throw new Error("Exhausted connection attempts")
}

/** Gracefully disconnects the bot. Called during sidecar shutdown. */
export function disconnectBot(): void {
  intentionalQuit = true
  if (bot) {
    bot.quit("Sidecar shutting down")
    bot = null
  }
}

// ---------------------------------------------------------------------------
// Internal
// ---------------------------------------------------------------------------

function createAndWaitForSpawn(): Promise<Bot> {
  return new Promise((resolve, reject) => {
    console.log(
      `[bot] Connecting to ${config.host}:${config.port} as "${config.username}" (MC ${config.version}, auth: ${config.auth})...`,
    )

    let newBot: Bot
    try {
      newBot = createMineflayerBot({
        host: config.host,
        port: config.port,
        username: config.username,
        version: config.version,
        auth: config.auth,
        ...(config.profilesFolder && { profilesFolder: config.profilesFolder }),
        ...(config.auth === "microsoft" && {
          onMsaCode: (data: { verification_uri: string; user_code: string }) => {
            console.log(
              `[bot] Microsoft auth: open ${data.verification_uri} and enter code ${data.user_code}`,
            )
          },
        }),
      })
    } catch (err) {
      reject(err)
      return
    }
    pendingBot = newBot

    newBot.once("spawn", () => {
      console.log("[bot] Spawned in world. Loading plugins...")
      loadPlugins(newBot)
      bot = newBot
      pendingBot = null
      reconnecting = false
      console.log("[bot] Ready.")
      botEvents.emit("botReady", newBot)
      wirePositionGuard(newBot)
      wireReconnect(newBot)
      resolve(newBot)
    })

    newBot.once("error", (err: Error) => {
      // Only reject if this is the initial connection attempt.
      if (!bot && !reconnecting) {
        reject(err)
      } else {
        console.error("[bot] Error:", err.message)
      }
    })
  })
}

function loadPlugins(b: Bot): void {
  // Order matters: pathfinder must be loaded before collectblock.
  b.loadPlugin(pathfinder)
  b.loadPlugin(toolPlugin)
  b.loadPlugin(collectBlock)
  b.loadPlugin(autoEat)
  b.loadPlugin(packetDebugPlugin)
  console.log("[bot] Plugins loaded: pathfinder, tool, collectblock, auto-eat")
}

function wirePositionGuard(b: Bot): void {
  // Save last-known-good position on every physics tick (20Hz).
  // physicsTick stops firing once position becomes NaN, so this captures
  // the most recent valid position before corruption.
  b.on("physicsTick", () => {
    const pos = b.entity.position
    if (Number.isFinite(pos.x) && Number.isFinite(pos.y) && Number.isFinite(pos.z)) {
      lastGoodPos = { x: pos.x, y: pos.y, z: pos.z }
    }
  })

  // Independent watchdog: mineflayer's physics engine skips ticks when
  // position is NaN (physics.js:79), so no events fire. A separate timer
  // is the only way to detect and recover from the dead state.
  posWatchdog = setInterval(() => {
    if (!b.entity) return
    const pos = b.entity.position
    if (Number.isFinite(pos.x) && Number.isFinite(pos.y) && Number.isFinite(pos.z)) return

    console.error(`[bot] CRITICAL: Position is invalid (${pos.x}, ${pos.y}, ${pos.z})`)
    try {
      b.pathfinder.stop()
    } catch {
      /* pathfinder may not be ready */
    }

    if (lastGoodPos) {
      b.entity.position.set(lastGoodPos.x, lastGoodPos.y, lastGoodPos.z)
      b.entity.velocity.set(0, 0, 0)
      console.log(
        `[bot] Restored position to (${lastGoodPos.x.toFixed(1)}, ${lastGoodPos.y.toFixed(1)}, ${lastGoodPos.z.toFixed(1)})`,
      )
      emitAutopilotEvent(
        `Position became NaN — restored to (${lastGoodPos.x.toFixed(0)}, ${lastGoodPos.y.toFixed(0)}, ${lastGoodPos.z.toFixed(0)})`,
      )
    } else {
      console.error("[bot] No last known good position — cannot auto-recover")
      emitAutopilotEvent("Position became NaN — no recovery position available")
    }
  }, 1000)
}

function wireReconnect(b: Bot): void {
  b.on("end", (reason: string) => {
    if (intentionalQuit) return
    console.log(`[bot] Disconnected: ${reason}`)
    handleDisconnect(b)
  })

  b.on("kicked", (reason: string) => {
    if (intentionalQuit) return
    console.log(`[bot] Kicked: ${reason}`)
    handleDisconnect(b)
  })
}

function handleDisconnect(oldBot: Bot): void {
  if (reconnecting) return
  reconnecting = true
  bot = null
  lastGoodPos = null
  if (posWatchdog) {
    clearInterval(posWatchdog)
    posWatchdog = null
  }
  botEvents.emit("botLost", oldBot)
  scheduleReconnect(0)
}

function sleep(ms: number): Promise<void> {
  return new Promise(resolve => setTimeout(resolve, ms))
}

function withTimeout<T>(promise: Promise<T>, ms: number, message: string): Promise<T> {
  return new Promise<T>((resolve, reject) => {
    const timer = setTimeout(() => reject(new Error(message)), ms)
    promise.then(
      val => {
        clearTimeout(timer)
        resolve(val)
      },
      err => {
        clearTimeout(timer)
        reject(err)
      },
    )
  })
}

function scheduleReconnect(attempt: number): void {
  if (intentionalQuit) return

  const baseDelay = Math.min(BASE_DELAY_MS * Math.pow(BACKOFF_MULTIPLIER, attempt), MAX_DELAY_MS)
  const jitter = baseDelay * JITTER_FACTOR * (Math.random() * 2 - 1)
  const delay = Math.round(baseDelay + jitter)

  console.log(`[bot] Reconnecting in ${(delay / 1000).toFixed(1)}s (attempt ${attempt + 1})...`)

  setTimeout(async () => {
    if (intentionalQuit) return
    try {
      await createAndWaitForSpawn()
      console.log("[bot] Reconnected successfully.")
    } catch (err) {
      console.error(`[bot] Reconnect attempt ${attempt + 1} failed:`, (err as Error).message)
      scheduleReconnect(attempt + 1)
    }
  }, delay)
}
