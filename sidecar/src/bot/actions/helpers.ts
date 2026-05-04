import type { Bot } from "mineflayer"
import type { Block as MfBlock } from "prismarine-block"
import type { IndexedData } from "minecraft-data"
import type { Entity as MfEntity } from "prismarine-entity"
import type { Item } from "prismarine-item"
import type { Window } from "prismarine-windows"
import { Vec3 } from "vec3"
import pathfinderPkg from "mineflayer-pathfinder"
import type { goals as goalsNs, Move } from "mineflayer-pathfinder"
const { Movements, goals } = pathfinderPkg
import type { ActionResult, InventoryItem as ProtoItem } from "../../grpc/generated/minecraft.js"

// ---------------------------------------------------------------------------
// ActionResult constructors
// ---------------------------------------------------------------------------

/** Construct a successful ActionResult with the given message. */
export function ok(message: string): ActionResult {
  return { success: true, error: "", message }
}

/** Construct a failed ActionResult from an error or string message. */
export function fail(err: unknown): ActionResult {
  const message = err instanceof Error ? err.message : String(err)
  return { success: false, error: message, message: "" }
}

// ---------------------------------------------------------------------------
// Event loop yielding
// ---------------------------------------------------------------------------

/** Yield control so pending I/O (keepalive responses, gRPC) can run. */
export function yieldToEventLoop(): Promise<void> {
  return new Promise(resolve => setTimeout(resolve, 0))
}

// ---------------------------------------------------------------------------
// Entity resolution
// ---------------------------------------------------------------------------

/** Return the entity with the given numeric ID, or undefined if not loaded. */
export function findEntityById(bot: Bot, id: number): MfEntity | undefined {
  return bot.entities[id]
}

/** Return the nearest entity matching name, username, or type, or undefined if none. */
export function findEntityByName(bot: Bot, name: string): MfEntity | undefined {
  const lower = name.toLowerCase()
  let best: MfEntity | undefined
  let bestDist = Infinity

  for (const entity of Object.values(bot.entities)) {
    if (entity === bot.entity) continue
    const eName = (entity.name ?? "").toLowerCase()
    const eUsername = (entity.username ?? "").toLowerCase()
    const eType = (entity.type ?? "").toLowerCase()

    if (eName === lower || eUsername === lower || eType === lower) {
      const dist = entity.position.distanceTo(bot.entity.position)
      if (dist < bestDist) {
        best = entity
        bestDist = dist
      }
    }
  }

  return best
}

// ---------------------------------------------------------------------------
// Inventory helpers
// ---------------------------------------------------------------------------

/** Return the first inventory item whose name matches (case-insensitive), or undefined. */
export function findItemInInventory(bot: Bot, name: string): Item | undefined {
  const lower = name.toLowerCase()
  return bot.inventory.items().find(i => i.name.toLowerCase() === lower)
}

/** Return the first item in the inventory portion of a window matching name, or undefined. */
export function findItemInWindow(window: Window, name: string): Item | undefined {
  const lower = name.toLowerCase()
  for (let i = window.inventoryStart; i < window.inventoryEnd; i++) {
    const item = window.slots[i]
    if (item && item.name.toLowerCase() === lower) return item
  }
  return undefined
}

/** Return a count-by-name snapshot of the main inventory for pre/post diffs. */
export function snapshotInventory(bot: Bot): Map<string, number> {
  const m = new Map<string, number>()
  for (const item of bot.inventory.items()) {
    m.set(item.name, (m.get(item.name) ?? 0) + item.count)
  }
  return m
}

/** Options for inventory-settle timing. All fields are optional with sensible defaults. */
export interface SettleOpts {
  firstEventTimeout?: number
  quietPeriod?: number
  maxWait?: number
}

/** Tracker returned by trackInventorySettle for waiting on or cancelling a settle window. */
export interface InventorySettleTracker {
  wait(): Promise<void>
  cancel(): void
}

/**
 * Pre-attach an updateSlot listener and return a tracker whose wait() resolves
 * once inventory updates quiesce. Attach before the triggering action so
 * synchronous events are captured.
 */
export function trackInventorySettle(bot: Bot, opts?: SettleOpts): InventorySettleTracker {
  const firstEventTimeout = opts?.firstEventTimeout ?? 150
  const quietPeriod = opts?.quietPeriod ?? 80
  const maxWait = opts?.maxWait ?? 2000

  let eventSeen = false
  let cancelled = false
  let debounceTimer: ReturnType<typeof setTimeout> | null = null
  let maxTimer: ReturnType<typeof setTimeout> | null = null
  let firstTimer: ReturnType<typeof setTimeout> | null = null
  let resolvePromise: (() => void) | null = null

  const cleanup = () => {
    if (debounceTimer) clearTimeout(debounceTimer)
    if (maxTimer) clearTimeout(maxTimer)
    if (firstTimer) clearTimeout(firstTimer)
    bot.inventory.removeListener("updateSlot", onSlotUpdate)
  }

  const settleAndResolve = () => {
    cleanup()
    if (resolvePromise) resolvePromise()
  }

  const onSlotUpdate = () => {
    eventSeen = true
    if (debounceTimer) clearTimeout(debounceTimer)
    debounceTimer = setTimeout(settleAndResolve, quietPeriod)
  }

  bot.inventory.on("updateSlot", onSlotUpdate)

  maxTimer = setTimeout(() => {
    cleanup()
    if (resolvePromise) resolvePromise()
  }, maxWait)

  return {
    wait(): Promise<void> {
      if (cancelled) return Promise.resolve()
      return new Promise<void>(resolve => {
        resolvePromise = resolve
        if (eventSeen) {
          if (debounceTimer) clearTimeout(debounceTimer)
          debounceTimer = setTimeout(settleAndResolve, quietPeriod)
        } else {
          firstTimer = setTimeout(() => {
            if (!eventSeen) settleAndResolve()
          }, firstEventTimeout)
        }
      })
    },
    cancel() {
      cancelled = true
      cleanup()
    },
  }
}

/** Wait for inventory updateSlot events to quiesce after an action. */
export function waitForInventorySettle(bot: Bot, opts?: SettleOpts): Promise<void> {
  return trackInventorySettle(bot, opts).wait()
}

/** Return the total count of items named `name` in the main inventory, zero if absent. */
export function countItem(bot: Bot, name: string): number {
  let total = 0
  for (const item of bot.inventory.items()) {
    if (item.name === name) total += item.count
  }
  return total
}

/** Return the total count of new items added to inventory since the snapshot. */
export function countGainSince(bot: Bot, before: Map<string, number>): number {
  let total = 0
  const current = snapshotInventory(bot)
  for (const [name, count] of current) {
    const prev = before.get(name) ?? 0
    if (count > prev) total += count - prev
  }
  return total
}

/** Return all inventory items whose count increased since the snapshot as a ProtoItem list. */
export function computeGains(bot: Bot, before: Map<string, number>): ProtoItem[] {
  const gains: ProtoItem[] = []
  const current = snapshotInventory(bot)
  const items = bot.inventory.items()
  for (const [name, count] of current) {
    const prev = before.get(name) ?? 0
    if (count <= prev) continue
    const rep = items.find(i => i.name === name)
    if (!rep) continue
    const mapped = mapItemToProto(rep)
    gains.push({ ...mapped, count: count - prev })
  }
  return gains
}

/**
 * Return a Movements config that blocks tunneling through solid terrain but
 * allows breaking trivial obstacles (tall grass, cobwebs, rails). Use for all
 * navigation that must not consume inventory blocks as scaffolding.
 */
export function safeMovements(bot: Bot): InstanceType<typeof Movements> {
  const mcData = bot.registry as unknown as IndexedData
  const m = new Movements(bot)
  m.allowSprinting = true
  for (const block of mcData.blocksArray) {
    if (block.boundingBox === "block") {
      m.blocksCantBreak.add(block.id)
    }
  }
  m.allow1by1towers = false
  m.scafoldingBlocks = []
  m.maxDropDown = 2
  m.infiniteLiquidDropdownDistance = false
  return m
}

/**
 * Like safeMovements but permits digging through breakable blocks. Still
 * suppresses scaffolding and 1x1 towers to avoid consuming inventory items.
 */
export function digMovements(bot: Bot): InstanceType<typeof Movements> {
  const m = new Movements(bot)
  m.allowSprinting = true
  m.allow1by1towers = false
  m.scafoldingBlocks = []
  m.maxDropDown = 2
  m.infiniteLiquidDropdownDistance = false
  return m
}

/** Convert a Mineflayer Item to the proto InventoryItem format. */
export function mapItemToProto(item: Item): ProtoItem {
  const maxDur = item.maxDurability ?? 0
  const currentDur = maxDur > 0 ? maxDur - (item.durabilityUsed ?? 0) : 0
  return {
    name: item.name,
    count: item.count,
    slot: item.slot,
    displayName: item.displayName,
    maxDurability: maxDur,
    currentDurability: currentDur,
  }
}

// ---------------------------------------------------------------------------
// Face vectors for PlaceBlock
// ---------------------------------------------------------------------------

/** Unit vectors keyed by face name for use with bot.placeBlock. */
export const FACE_VECTORS: Record<string, Vec3> = {
  top: new Vec3(0, 1, 0),
  bottom: new Vec3(0, -1, 0),
  north: new Vec3(0, 0, -1),
  south: new Vec3(0, 0, 1),
  east: new Vec3(1, 0, 0),
  west: new Vec3(-1, 0, 0),
}

// Items that produce orientation-specific block variants on placement.
const BLOCK_PLACEMENT_VARIANTS: Record<string, ReadonlySet<string>> = {
  torch: new Set(["torch", "wall_torch"]),
  soul_torch: new Set(["soul_torch", "soul_wall_torch"]),
  redstone_torch: new Set(["redstone_torch", "redstone_wall_torch"]),
}

/** Return true if blockName is the expected placed form of itemName, accounting for wall variants. */
export function matchesPlacedBlock(itemName: string, blockName: string): boolean {
  const variants = BLOCK_PLACEMENT_VARIANTS[itemName]
  return variants ? variants.has(blockName) : itemName === blockName
}

// ---------------------------------------------------------------------------
// Equipment destination mapping
// ---------------------------------------------------------------------------

/** Valid equipment destination names accepted by bot.equip. */
export const EQUIP_DESTINATIONS: Record<string, string> = {
  hand: "hand",
  "off-hand": "off-hand",
  head: "head",
  torso: "torso",
  legs: "legs",
  feet: "feet",
}

// ---------------------------------------------------------------------------
// Interactive blocks (open a UI on right-click -- unsafe as placeBlock anchors)
// ---------------------------------------------------------------------------

/** Block names that open a UI on right-click and must not be used as placeBlock anchors. */
export const INTERACTIVE_BLOCKS = new Set([
  "chest",
  "trapped_chest",
  "ender_chest",
  "barrel",
  "hopper",
  "dropper",
  "dispenser",
  "furnace",
  "blast_furnace",
  "smoker",
  "brewing_stand",
  "shulker_box",
  "white_shulker_box",
  "orange_shulker_box",
  "magenta_shulker_box",
  "light_blue_shulker_box",
  "yellow_shulker_box",
  "lime_shulker_box",
  "pink_shulker_box",
  "gray_shulker_box",
  "light_gray_shulker_box",
  "cyan_shulker_box",
  "purple_shulker_box",
  "blue_shulker_box",
  "brown_shulker_box",
  "green_shulker_box",
  "red_shulker_box",
  "black_shulker_box",
  "crafting_table",
  "enchanting_table",
  "anvil",
  "chipped_anvil",
  "damaged_anvil",
  "grindstone",
  "stonecutter",
  "cartography_table",
  "loom",
  "smithing_table",
  "beacon",
])

/** Return true if the block name opens a UI on right-click. */
export function isInteractiveBlock(name: string): boolean {
  return INTERACTIVE_BLOCKS.has(name)
}

// ---------------------------------------------------------------------------
// Pathfinder arrival verification
// ---------------------------------------------------------------------------

/** Return true if the bot's current position satisfies the pathfinder goal. */
export function checkGoalReached(bot: Bot, goal: goalsNs.Goal): boolean {
  const pos = bot.entity.position
  return goal.isEnd({
    x: Math.floor(pos.x),
    y: Math.floor(pos.y),
    z: Math.floor(pos.z),
  } as unknown as Move)
}

/** Return Euclidean distance from pos to (x, y, z). */
export function distanceTo(
  pos: { x: number; y: number; z: number },
  x: number,
  y: number,
  z: number,
): number {
  const dx = pos.x - x
  const dy = pos.y - y
  const dz = pos.z - z
  return Math.sqrt(dx * dx + dy * dy + dz * dz)
}

// ---------------------------------------------------------------------------
// Position validation
// ---------------------------------------------------------------------------

/** Throw if the bot's position contains NaN; return the valid position otherwise. */
export function validatePosition(bot: Bot): Vec3 {
  const pos = bot.entity.position
  if (isNaN(pos.x) || isNaN(pos.y) || isNaN(pos.z)) {
    throw new Error(`bot position is NaN (${pos.x}, ${pos.y}, ${pos.z})`)
  }
  return pos
}

// ---------------------------------------------------------------------------
// Surface-preferring block search
// ---------------------------------------------------------------------------

const NEIGHBOR_OFFSETS: Vec3[] = [
  new Vec3(0, 1, 0),
  new Vec3(0, -1, 0),
  new Vec3(1, 0, 0),
  new Vec3(-1, 0, 0),
  new Vec3(0, 0, 1),
  new Vec3(0, 0, -1),
]

/**
 * Find the nearest surface-exposed block of blockId within maxDistance. Falls
 * back to the closest block regardless of exposure when no surface candidates
 * exist.
 */
export function findSurfaceBlock(
  bot: Bot,
  blockId: number,
  maxDistance: number,
  maxY?: number,
): MfBlock | null {
  const all = bot.findBlocks({ matching: blockId, maxDistance, count: 32 })
  const positions = maxY !== undefined ? all.filter(p => p.y <= maxY) : all
  if (positions.length === 0) return null

  for (const pos of positions) {
    for (const offset of NEIGHBOR_OFFSETS) {
      const neighbor = bot.blockAt(pos.plus(offset))
      if (!neighbor || neighbor.name === "air") {
        return bot.blockAt(pos)
      }
    }
  }

  return bot.blockAt(positions[0])
}

const GOTO_TIMEOUT_MS = 45_000

/**
 * Wrap bot.pathfinder.goto with a hard timeout (default 45s). Calls
 * pathfinder.stop() on expiry, which causes goto() to reject.
 */
export function gotoWithTimeout(
  bot: Bot,
  goal: goalsNs.Goal,
  timeoutMs: number = GOTO_TIMEOUT_MS,
): Promise<void> {
  validatePosition(bot)
  return new Promise<void>((resolve, reject) => {
    const timer = setTimeout(() => {
      bot.pathfinder.stop()
      reject(new Error(`pathfinder timed out after ${(timeoutMs / 1000).toFixed(0)}s`))
    }, timeoutMs)

    bot.pathfinder.goto(goal).then(
      () => {
        clearTimeout(timer)
        resolve()
      },
      err => {
        clearTimeout(timer)
        reject(err)
      },
    )
  })
}

/** True when the pathfinder error is transient and a retry may succeed. */
export function isPathRetryable(err: unknown): boolean {
  if (!(err instanceof Error)) return false
  return err.name === "PathStopped" || err.name === "GoalChanged"
}

const GOTO_MAX_RETRIES = 2

/**
 * Wrap gotoWithTimeout with retry on transient pathfinder errors
 * (PathStopped, GoalChanged). Yields between retries so the
 * pathfinder's internal cleanup runs before re-entering setGoal.
 */
export async function gotoWithRetry(
  bot: Bot,
  goal: goalsNs.Goal,
  timeoutMs: number = GOTO_TIMEOUT_MS,
  maxRetries: number = GOTO_MAX_RETRIES,
): Promise<void> {
  for (let attempt = 0; ; attempt++) {
    try {
      await gotoWithTimeout(bot, goal, timeoutMs)
      return
    } catch (err) {
      if (attempt < maxRetries && isPathRetryable(err)) {
        await new Promise(r => setTimeout(r, 0))
        continue
      }
      throw err
    }
  }
}

// ---------------------------------------------------------------------------
// Post-dig drop collection
// ---------------------------------------------------------------------------

const DROP_SETTLE_MS = 150
const DROP_WALK_TIMEOUT_MS = 1500
const DROP_PICKUP_PAUSE_MS = 150

function horizDistance(a: Vec3, b: Vec3): number {
  const dx = a.x - b.x
  const dz = a.z - b.z
  return Math.sqrt(dx * dx + dz * dz)
}

/** Walk to the dig site to collect nearby drops via auto-pickup. */
export async function collectNearbyDrops(bot: Bot, referencePos: Vec3): Promise<void> {
  await new Promise(r => setTimeout(r, DROP_SETTLE_MS))

  const pos = bot.entity.position
  const hDist = horizDistance(pos, referencePos)
  if (hDist <= 1.5) {
    await new Promise(r => setTimeout(r, DROP_PICKUP_PAUSE_MS))
    return
  }

  const target = new Vec3(referencePos.x, pos.y, referencePos.z)

  if (hDist <= 5) {
    await bot.lookAt(target, true)
    bot.setControlState("forward", true)
    bot.setControlState("sprint", true)

    const start = Date.now()
    while (
      horizDistance(bot.entity.position, referencePos) > 1.5 &&
      Date.now() - start < DROP_WALK_TIMEOUT_MS
    ) {
      await new Promise(r => setTimeout(r, 50))
    }
    bot.setControlState("forward", false)
    bot.setControlState("sprint", false)

    await new Promise(r => setTimeout(r, DROP_PICKUP_PAUSE_MS))
    return
  }

  try {
    await gotoWithTimeout(bot, new goals.GoalNear(target.x, target.y, target.z, 1), 5_000)
  } catch {
    // Unreachable
  }
}
