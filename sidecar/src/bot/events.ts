import type { Bot } from "mineflayer"
import type { Entity as MfEntity } from "prismarine-entity"
import type { Block as MfBlock } from "prismarine-block"
import type { ServerWritableStream } from "@grpc/grpc-js"
import {
  EventType,
  EventPriority,
  type SubscribeEventsRequest,
  type GameEvent,
  type Entity as ProtoEntity,
} from "../grpc/generated/minecraft.js"

// ---------------------------------------------------------------------------
// Subscriber management
// ---------------------------------------------------------------------------

/** Handle returned by addSubscriber for cancelling a gRPC event stream. */
export interface Subscriber {
  readonly id: number
  unsubscribe(): void
}

interface SubscriberEntry {
  id: number
  call: ServerWritableStream<SubscribeEventsRequest, GameEvent>
  filterSet: Set<EventType>
}

let nextId = 1
const subscribers = new Map<number, SubscriberEntry>()

/** Adds a gRPC stream subscriber. Returns a handle for cleanup. */
export function addSubscriber(
  call: ServerWritableStream<SubscribeEventsRequest, GameEvent>,
  filterTypes: EventType[],
): Subscriber {
  const id = nextId++
  const filterSet = new Set(filterTypes)
  subscribers.set(id, { id, call, filterSet })
  console.log(
    `[events] Subscriber ${id} added (filter: ${filterSet.size === 0 ? "all" : [...filterSet].join(", ")})`,
  )
  return {
    id,
    unsubscribe() {
      subscribers.delete(id)
      console.log(`[events] Subscriber ${id} removed`)
    },
  }
}

/** Send a GameEvent to all active subscribers, skipping any whose stream has closed. */
export function broadcast(event: GameEvent): void {
  for (const [id, entry] of subscribers) {
    // Apply filter: empty filterSet = all events pass.
    if (entry.filterSet.size > 0 && !entry.filterSet.has(event.type)) continue

    try {
      entry.call.write(event)
    } catch {
      // Client gone -- remove subscriber.
      subscribers.delete(id)
      console.log(`[events] Subscriber ${id} removed (write failed)`)
    }
  }
}

// ---------------------------------------------------------------------------
// Mineflayer listener registration
// ---------------------------------------------------------------------------

// Store bound listeners so we can remove them on disconnect.
const boundListeners = new WeakMap<Bot, Map<string, (...args: unknown[]) => void>>()

/** Attach Mineflayer event listeners that broadcast game events to all subscribers. */
export function registerBotListeners(bot: Bot): void {
  const listeners = new Map<string, (...args: unknown[]) => void>()
  boundListeners.set(bot, listeners)

  let lastTimePhase = ""

  function on<K extends string>(event: K, fn: (...args: unknown[]) => void) {
    listeners.set(event, fn)
    bot.on(event as "spawn", fn as () => void)
  }

  // 1. entitySpawn
  on("entitySpawn", (entity: unknown) => {
    const e = entity as MfEntity
    if (e.id === bot.entity.id) return
    broadcast(
      makeEvent(EventType.EVENT_ENTITY_SPAWN, EventPriority.EVENT_PRIORITY_LOW, {
        description: `${e.name ?? e.type ?? "unknown"} spawned`,
        entity: entityToProto(e, bot),
        position: vecToProto(e.position),
      }),
    )
  })

  // 2. entityGone
  on("entityGone", (entity: unknown) => {
    const e = entity as MfEntity
    if (e.id === bot.entity.id) return
    broadcast(
      makeEvent(EventType.EVENT_ENTITY_DESPAWN, EventPriority.EVENT_PRIORITY_LOW, {
        description: `${e.name ?? e.type ?? "unknown"} despawned`,
        entity: entityToProto(e, bot),
      }),
    )
  })

  // 3. entityHurt
  on("entityHurt", (entity: unknown) => {
    const e = entity as MfEntity
    broadcast(
      makeEvent(EventType.EVENT_ENTITY_HURT, EventPriority.EVENT_PRIORITY_HIGH, {
        description: `${e.name ?? e.type ?? "unknown"} was hurt`,
        entity: entityToProto(e, bot),
        position: vecToProto(e.position),
      }),
    )
  })

  // 4. entityDead
  on("entityDead", (entity: unknown) => {
    const e = entity as MfEntity
    broadcast(
      makeEvent(EventType.EVENT_ENTITY_DEATH, EventPriority.EVENT_PRIORITY_NORMAL, {
        description: `${e.name ?? e.type ?? "unknown"} died`,
        entity: entityToProto(e, bot),
        position: vecToProto(e.position),
      }),
    )
  })

  // 5. health (bot health/food changed)
  on("health", () => {
    broadcast(
      makeEvent(EventType.EVENT_HEALTH_CHANGE, EventPriority.EVENT_PRIORITY_HIGH, {
        description: `Health: ${bot.health}/20, Food: ${bot.food}/20`,
        position: vecToProto(bot.entity.position),
      }),
    )
  })

  // 6. death (bot died)
  on("death", () => {
    broadcast(
      makeEvent(EventType.EVENT_BOT_DEATH, EventPriority.EVENT_PRIORITY_CRITICAL, {
        description: "Bot died!",
        position: vecToProto(bot.entity.position),
      }),
    )
  })

  // 7. chat
  on("chat", (username: unknown, message: unknown) => {
    const u = username as string
    const m = message as string
    if (u === bot.username) return
    broadcast(
      makeEvent(EventType.EVENT_CHAT_MESSAGE, EventPriority.EVENT_PRIORITY_HIGH, {
        description: `<${u}> ${m}`,
        playerName: u,
        chatMessage: m,
      }),
    )
  })

  // 8. whisper
  on("whisper", (username: unknown, message: unknown) => {
    const u = username as string
    const m = message as string
    if (u === bot.username) return
    broadcast(
      makeEvent(EventType.EVENT_CHAT_MESSAGE, EventPriority.EVENT_PRIORITY_HIGH, {
        description: `[whisper] <${u}> ${m}`,
        playerName: u,
        chatMessage: m,
      }),
    )
  })

  // 9. time -- only emit on phase transitions
  on("time", () => {
    const phase = ticksToPhase(bot.time.timeOfDay)
    if (phase !== lastTimePhase) {
      lastTimePhase = phase
      broadcast(
        makeEvent(EventType.EVENT_DAY_NIGHT, EventPriority.EVENT_PRIORITY_LOW, {
          description: `Time changed to ${phase}`,
        }),
      )
    }
  })

  // 10. rain
  on("rain", () => {
    const weather = describeWeather(bot)
    broadcast(
      makeEvent(EventType.EVENT_WEATHER_CHANGE, EventPriority.EVENT_PRIORITY_LOW, {
        description: `Weather: ${weather}`,
      }),
    )
  })

  // 11. playerJoined
  on("playerJoined", (player: unknown) => {
    const p = player as { username: string }
    broadcast(
      makeEvent(EventType.EVENT_PLAYER_JOINED, EventPriority.EVENT_PRIORITY_NORMAL, {
        description: `${p.username} joined the game`,
        playerName: p.username,
      }),
    )
  })

  // 12. playerLeft
  on("playerLeft", (player: unknown) => {
    const p = player as { username: string }
    broadcast(
      makeEvent(EventType.EVENT_PLAYER_LEFT, EventPriority.EVENT_PRIORITY_NORMAL, {
        description: `${p.username} left the game`,
        playerName: p.username,
      }),
    )
  })

  // 13. blockUpdate -- only nearby blocks
  on("blockUpdate", (_oldBlock: unknown, newBlock: unknown) => {
    const nb = newBlock as MfBlock
    if (!nb?.position) return
    const dist = nb.position.distanceTo(bot.entity.position)
    if (dist > 16) return
    broadcast(
      makeEvent(EventType.EVENT_BLOCK_UPDATE, EventPriority.EVENT_PRIORITY_LOW, {
        description: `Block at (${nb.position.x}, ${nb.position.y}, ${nb.position.z}) changed to ${nb.name}`,
        blockType: nb.name,
        position: vecToProto(nb.position),
      }),
    )
  })

  console.log("[events] Bot listeners registered")
}

/** Remove the listeners attached by registerBotListeners from the given bot. */
export function unregisterBotListeners(bot: Bot): void {
  const listeners = boundListeners.get(bot)
  if (!listeners) return

  for (const [event, fn] of listeners) {
    ;(
      bot as unknown as { removeListener(e: string, fn: (...args: unknown[]) => void): void }
    ).removeListener(event, fn)
  }
  boundListeners.delete(bot)
  console.log("[events] Bot listeners unregistered")
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

/** Construct a GameEvent with sensible zero-value defaults for unused fields. */
export function makeEvent(
  type: EventType,
  priority: EventPriority,
  fields: Partial<GameEvent>,
): GameEvent {
  return {
    type,
    priority,
    timestamp: Date.now(),
    description: "",
    entity: undefined,
    playerName: "",
    chatMessage: "",
    position: undefined,
    blockType: "",
    ...fields,
  }
}

function entityToProto(entity: MfEntity, bot: Bot): ProtoEntity {
  return {
    id: entity.id,
    type: entity.name ?? entity.type ?? "unknown",
    name: entity.username ?? "",
    position: vecToProto(entity.position),
    distance: Math.round(entity.position.distanceTo(bot.entity.position) * 10) / 10,
    health: entity.health ?? 0,
  }
}

function vecToProto(v: { x: number; y: number; z: number }) {
  return { x: v.x, y: v.y, z: v.z }
}

function ticksToPhase(ticks: number): string {
  if (ticks < 1000) return "dawn"
  if (ticks < 6000) return "morning"
  if (ticks < 12000) return "noon"
  if (ticks < 13000) return "afternoon"
  if (ticks < 13800) return "dusk"
  if (ticks < 22300) return "night"
  if (ticks < 23000) return "midnight"
  return "dawn"
}

function describeWeather(bot: Bot): string {
  if (bot.thunderState > 0) return "thunderstorm"
  if (bot.isRaining) return "raining"
  return "clear"
}

// ---------------------------------------------------------------------------
// Autopilot event helper
// ---------------------------------------------------------------------------

/** Emits an autopilot action event to all subscribers. */
export function emitAutopilotEvent(description: string): void {
  broadcast(
    makeEvent(EventType.EVENT_AUTOPILOT_ACTION, EventPriority.EVENT_PRIORITY_NORMAL, {
      description,
    }),
  )
}
