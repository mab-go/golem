import type { ServerWritableStream } from "@grpc/grpc-js"
import { status as grpcStatus } from "@grpc/grpc-js"
import type { Bot } from "mineflayer"
import type { Block as MfBlock } from "prismarine-block"
import type { IndexedData } from "minecraft-data"
import pathfinderPkg from "mineflayer-pathfinder"
const { goals } = pathfinderPkg
import { Vec3 } from "vec3"
import { getBot } from "../connection.js"
import { runTask, type TaskContext } from "./task-runner.js"
import {
  collectNearbyDrops,
  computeGains,
  countGainSince,
  countItem,
  findItemInInventory,
  findItemInWindow,
  findSurfaceBlock,
  gotoWithRetry,
  gotoWithTimeout,
  isInteractiveBlock,
  mapItemToProto,
  safeMovements,
  snapshotInventory,
  trackInventorySettle,
  validatePosition,
} from "./helpers.js"
import type {
  GatherRequest,
  HarvestBlockRequest,
  BuildStructureRequest,
  ProcessAllRequest,
  OrganizeInventoryRequest,
  ClearAreaRequest,
  FarmRequest,
  TaskProgress,
  TaskResult,
  InventoryItem as ProtoItem,
} from "../../grpc/generated/minecraft.js"

// ---------------------------------------------------------------------------
// Gather -- loop findBlock -> navigate -> equip -> dig -> accumulate drops.
// ---------------------------------------------------------------------------

const GATHER_DEFAULT_RADIUS = 32

function requireBotOrThrow(): Bot {
  const bot = getBot()
  if (!bot) {
    throw {
      code: grpcStatus.UNAVAILABLE,
      details: "Bot not connected to Minecraft server",
    }
  }
  return bot
}

/** Stream a gather task: repeatedly find, navigate to, and mine the requested resource. */
export function gather(call: ServerWritableStream<GatherRequest, TaskProgress>): void {
  const bot = requireBotOrThrow()
  runTask(call, "gather", async tc => runGather(tc, call.request, bot), bot)
}

async function runGather(tc: TaskContext, req: GatherRequest, bot: Bot): Promise<TaskResult> {
  const mcData = bot.registry as unknown as IndexedData
  const blockInfo = mcData.blocksByName[req.resource]
  if (!blockInfo) {
    return {
      success: false,
      summary: `Unknown block type "${req.resource}"`,
      itemsGained: [],
      itemsUsed: [],
    }
  }

  const target = req.quantity > 0 ? req.quantity : 1
  const radius = req.radius > 0 ? req.radius : GATHER_DEFAULT_RADIUS

  bot.pathfinder.setMovements(safeMovements(bot))

  const before = snapshotInventory(bot)
  let collected = 0
  let blocksMined = 0

  tc.progress(`Looking for ${req.resource}...`, 0, target)

  while (collected < target) {
    if (tc.cancelled()) break
    validatePosition(bot)

    const block = findSurfaceBlock(bot, blockInfo.id, radius)
    if (!block) {
      break
    }

    try {
      const bp = block.position
      await gotoWithTimeout(bot, new goals.GoalNear(bp.x, bp.y, bp.z, 4))
    } catch (err) {
      // Pathfinding failed -- skip this block, try the next one (may still be reachable).
      tc.progress(
        `Path failed to ${req.resource} at (${block.position.x}, ${block.position.y}, ${block.position.z})`,
        collected,
        target,
      )
      continue
    }

    if (tc.cancelled()) break

    const liveBlock = bot.blockAt(block.position)
    if (!liveBlock || liveBlock.name === "air") continue

    // Guard against a findBlock / chunk-state mismatch. Mining the wrong
    // block would produce misleading drops against the gather target.
    if (liveBlock.name !== req.resource) {
      tc.progress(
        `Expected ${req.resource} at (${block.position.x}, ${block.position.y}, ${block.position.z}), got ${liveBlock.name} — skipping`,
        collected,
        target,
      )
      console.warn(
        `[gather] expected ${req.resource} at (${block.position.x}, ${block.position.y}, ${block.position.z}), ` +
          `got ${liveBlock.name} — skipping.`,
      )
      continue
    }

    const settle = trackInventorySettle(bot, {
      firstEventTimeout: 2000,
      quietPeriod: 150,
      maxWait: 5000,
    })
    try {
      await bot.tool.equipForBlock(liveBlock)
      await bot.dig(liveBlock)
      blocksMined++
    } catch (err) {
      settle.cancel()
      tc.progress(
        `Dig failed at (${liveBlock.position.x}, ${liveBlock.position.y}, ${liveBlock.position.z}): ${err instanceof Error ? err.message : String(err)}`,
        collected,
        target,
      )
      continue
    }

    await collectNearbyDrops(bot, liveBlock.position)

    await settle.wait()

    collected = countGainSince(bot, before)
    tc.progress(
      `Collected ${collected}/${target} ${req.resource} (mined ${blocksMined})`,
      collected,
      target,
    )
  }

  const gained = computeGains(bot, before)
  const success = collected >= target
  const summary = success
    ? `Gathered ${collected} ${req.resource} (mined ${blocksMined} blocks)`
    : `Gathered ${collected}/${target} ${req.resource} before running out (mined ${blocksMined} blocks)`

  return {
    success,
    summary,
    itemsGained: gained,
    itemsUsed: [],
  }
}

// ---------------------------------------------------------------------------
// HarvestBlock -- find and mine N blocks of a given type, streaming progress.
// ---------------------------------------------------------------------------

const HARVEST_DEFAULT_RADIUS = 32

export function harvestBlock(call: ServerWritableStream<HarvestBlockRequest, TaskProgress>): void {
  const bot = requireBotOrThrow()
  runTask(call, "harvest_block", async tc => runHarvestBlock(tc, call.request, bot), bot)
}

async function runHarvestBlock(
  tc: TaskContext,
  req: HarvestBlockRequest,
  bot: Bot,
): Promise<TaskResult> {
  const mcData = bot.registry as unknown as IndexedData
  const blockInfo = mcData.blocksByName[req.blockType]
  if (!blockInfo) {
    return {
      success: false,
      summary: `Unknown block type "${req.blockType}"`,
      itemsGained: [],
      itemsUsed: [],
    }
  }

  const count = req.count || 1
  const maxDistance = req.maxDistance || HARVEST_DEFAULT_RADIUS

  bot.pathfinder.setMovements(safeMovements(bot))

  const before = snapshotInventory(bot)
  let blocksMined = 0

  tc.progress(`Looking for ${req.blockType}...`, 0, count)

  for (let i = 0; i < count; i++) {
    if (tc.cancelled()) break
    validatePosition(bot)

    const maxY = Math.floor(bot.entity.position.y) + 4
    const block = findSurfaceBlock(bot, blockInfo.id, maxDistance, maxY)
    if (!block) break

    const bp = block.position

    try {
      await gotoWithRetry(bot, new goals.GoalNear(bp.x, bp.y, bp.z, 3))
    } catch {
      tc.progress(
        `Path failed to ${req.blockType} at (${bp.x}, ${bp.y}, ${bp.z})`,
        blocksMined,
        count,
      )
      continue
    }

    if (tc.cancelled()) break

    const target = bot.blockAt(bp)
    if (!target || target.name === "air") continue

    if (target.name !== req.blockType) {
      tc.progress(
        `Expected ${req.blockType} at (${bp.x}, ${bp.y}, ${bp.z}), got ${target.name} -- skipping`,
        blocksMined,
        count,
      )
      continue
    }

    const settle = trackInventorySettle(bot, {
      firstEventTimeout: 2000,
      quietPeriod: 150,
      maxWait: 5000,
    })
    try {
      await bot.tool.equipForBlock(target)
      await bot.dig(target)
      blocksMined++
    } catch (err) {
      settle.cancel()
      tc.progress(
        `Dig failed at (${bp.x}, ${bp.y}, ${bp.z}): ${err instanceof Error ? err.message : String(err)}`,
        blocksMined,
        count,
      )
      continue
    }

    await collectNearbyDrops(bot, bp)
    await settle.wait()

    tc.progress(`Harvested ${blocksMined}/${count} ${req.blockType}`, blocksMined, count)
  }

  const gained = computeGains(bot, before)
  const success = blocksMined >= count
  const summary = success
    ? `Harvested ${blocksMined} ${req.blockType}`
    : `Harvested ${blocksMined}/${count} ${req.blockType}`

  return {
    success,
    summary,
    itemsGained: gained,
    itemsUsed: [],
  }
}

function sleep(ms: number): Promise<void> {
  return new Promise(resolve => setTimeout(resolve, ms))
}

// ---------------------------------------------------------------------------
// BuildStructure -- minimal shelter: 4 walls + roof at the given location.
// ---------------------------------------------------------------------------

/** Stream a shelter-building task: place walls and roof at the given location. */
export function buildStructure(
  call: ServerWritableStream<BuildStructureRequest, TaskProgress>,
): void {
  const bot = requireBotOrThrow()
  runTask(call, "build_structure", async tc => runBuildStructure(tc, call.request, bot), bot)
}

async function runBuildStructure(
  tc: TaskContext,
  req: BuildStructureRequest,
  bot: Bot,
): Promise<TaskResult> {
  if (!req.location) {
    return fail("location required")
  }
  if (!req.material) {
    return fail("material required")
  }

  const width = Math.max(3, Math.floor(req.dimensions?.x ?? 5))
  const height = Math.max(3, Math.floor(req.dimensions?.y ?? 3))
  const depth = Math.max(3, Math.floor(req.dimensions?.z ?? 5))

  const origin = new Vec3(
    Math.floor(req.location.x),
    Math.floor(req.location.y),
    Math.floor(req.location.z),
  )

  const plan = planShelter(origin, width, height, depth)
  const total = plan.length

  const before = snapshotInventory(bot)
  let placed = 0
  let skipped = 0
  let used = 0

  bot.pathfinder.setMovements(safeMovements(bot))

  tc.progress(`Building ${width}×${height}×${depth} ${req.material} shelter`, 0, total)

  for (const pos of plan) {
    if (tc.cancelled()) break

    const existing = bot.blockAt(pos)
    if (existing && existing.name === req.material) {
      skipped++
      placed++
      tc.progress(`Placed ${placed}/${total} (${skipped} already present)`, placed, total)
      continue
    }
    if (existing && existing.name !== "air") {
      // Non-matching block in the way; skip rather than dig.
      skipped++
      placed++
      continue
    }

    let item = findItemInInventory(bot, req.material)
    if (!item) {
      return {
        success: false,
        summary: `Out of ${req.material} after placing ${placed - skipped}/${total - skipped}`,
        itemsGained: [],
        itemsUsed: inventoryDelta(bot, before),
      }
    }

    const anchor = findAnchor(bot, pos)
    if (!anchor) {
      skipped++
      placed++
      continue
    }

    try {
      await gotoWithTimeout(bot, new goals.GoalNear(pos.x, pos.y, pos.z, 3))
    } catch {
      // Pathing failure -- skip this block.
      skipped++
      placed++
      continue
    }
    if (tc.cancelled()) break

    item = findItemInInventory(bot, req.material)
    if (!item) {
      return {
        success: false,
        summary: `Out of ${req.material} after placing ${placed - skipped}`,
        itemsGained: [],
        itemsUsed: inventoryDelta(bot, before),
      }
    }

    try {
      await bot.equip(item, "hand")
      await new Promise(r => setTimeout(r, 50))

      let consumed = false
      for (let attempt = 0; attempt < 2 && !consumed; attempt++) {
        if (attempt > 0) {
          const check = bot.blockAt(pos)
          if (check && check.name !== "air") {
            consumed = true
            break
          }
        }

        const currentAnchor = attempt === 0 ? anchor : findAnchor(bot, pos)
        if (!currentAnchor) break

        const anchorBlock = bot.blockAt(currentAnchor.position)
        if (!anchorBlock) break

        const faceCenter = anchorBlock.position.offset(
          0.5 + currentAnchor.face.x * 0.5,
          0.5 + currentAnchor.face.y * 0.5,
          0.5 + currentAnchor.face.z * 0.5,
        )
        await bot.lookAt(faceCenter, true)

        const materialBefore = countItem(bot, req.material)
        try {
          await bot.placeBlock(anchorBlock, currentAnchor.face)
        } catch {
          // placeBlock may throw even on success (Mineflayer race)
        }
        const materialAfter = countItem(bot, req.material)
        if (materialAfter < materialBefore) {
          consumed = true
        }
      }

      if (consumed) {
        used++
      } else {
        console.warn(
          `[build_structure] placement at (${pos.x},${pos.y},${pos.z}) ` +
            `did not consume ${req.material}`,
        )
        skipped++
      }
      placed++
    } catch {
      skipped++
      placed++
      continue
    }

    tc.progress(`Placed ${placed}/${total}`, placed, total)
  }

  // Navigate outside through the door so the bot isn't trapped.
  const maxZ = origin.z + depth - 1
  const doorX = origin.x + Math.floor(width / 2)
  try {
    tc.progress("Exiting shelter", placed, total)
    await gotoWithTimeout(bot, new goals.GoalNear(doorX, origin.y, maxZ + 2, 1))
  } catch {
    console.warn("[build_structure] failed to navigate outside shelter after build")
  }

  const success = placed - skipped >= Math.floor(total * 0.8)
  const summary = success
    ? `Built shelter: placed ${placed - skipped} blocks (${skipped} pre-existing), used ${used} ${req.material}`
    : `Partial build: placed ${placed - skipped}/${total - skipped} blocks of ${req.material}`

  return {
    success,
    summary,
    itemsGained: [],
    itemsUsed: inventoryDelta(bot, before),
  }
}

function planShelter(origin: Vec3, w: number, h: number, d: number): Vec3[] {
  const positions: Vec3[] = []
  const maxX = origin.x + w - 1
  const maxZ = origin.z + d - 1

  // Door opening: 1x2 gap in south wall (z = maxZ) at center x.
  const doorX = origin.x + Math.floor(w / 2)

  // Walls: x = origin.x or maxX for each z in range, and z = origin.z or maxZ for each x.
  for (let y = origin.y; y < origin.y + h; y++) {
    for (let x = origin.x; x <= maxX; x++) {
      positions.push(new Vec3(x, y, origin.z))
      if (!(x === doorX && y < origin.y + 2)) {
        positions.push(new Vec3(x, y, maxZ))
      }
    }
    for (let z = origin.z + 1; z < maxZ; z++) {
      positions.push(new Vec3(origin.x, y, z))
      positions.push(new Vec3(maxX, y, z))
    }
  }
  // Roof: full rectangle at y = origin.y + h - 1? Walls already cover perimeter at top.
  // Add a solid roof layer one block above the top-of-walls.
  const roofY = origin.y + h
  for (let x = origin.x; x <= maxX; x++) {
    for (let z = origin.z; z <= maxZ; z++) {
      positions.push(new Vec3(x, roofY, z))
    }
  }
  return positions
}

interface AnchorTarget {
  position: Vec3
  face: Vec3
}

const FACE_OFFSETS: Array<{ offset: Vec3; face: Vec3 }> = [
  { offset: new Vec3(0, -1, 0), face: new Vec3(0, 1, 0) },
  { offset: new Vec3(0, 1, 0), face: new Vec3(0, -1, 0) },
  { offset: new Vec3(1, 0, 0), face: new Vec3(-1, 0, 0) },
  { offset: new Vec3(-1, 0, 0), face: new Vec3(1, 0, 0) },
  { offset: new Vec3(0, 0, 1), face: new Vec3(0, 0, -1) },
  { offset: new Vec3(0, 0, -1), face: new Vec3(0, 0, 1) },
]

function findAnchor(bot: Bot, target: Vec3): AnchorTarget | null {
  for (const { offset, face } of FACE_OFFSETS) {
    const neighborPos = target.offset(offset.x, offset.y, offset.z)
    const block = bot.blockAt(neighborPos)
    if (block && block.name !== "air" && !isLiquid(block.name) && !isInteractiveBlock(block.name)) {
      return { position: neighborPos, face }
    }
  }
  return null
}

function isLiquid(name: string): boolean {
  return name === "water" || name === "lava" || name === "flowing_water" || name === "flowing_lava"
}

function inventoryDelta(bot: Bot, before: Map<string, number>): ProtoItem[] {
  const out: ProtoItem[] = []
  const current = snapshotInventory(bot)
  const items = bot.inventory.items()
  for (const [name, prev] of before) {
    const now = current.get(name) ?? 0
    if (prev > now) {
      const rep = items.find(i => i.name === name)
      if (rep) {
        const mapped = mapItemToProto(rep)
        out.push({ ...mapped, count: prev - now })
      }
    }
  }
  return out
}

function fail(msg: string): TaskResult {
  return { success: false, summary: msg, itemsGained: [], itemsUsed: [] }
}

// ---------------------------------------------------------------------------
// ProcessAll -- smelt every unit of an item in a nearby furnace.
// ---------------------------------------------------------------------------

const PROCESS_FUEL_ORDER = [
  "coal",
  "charcoal",
  "oak_planks",
  "spruce_planks",
  "birch_planks",
  "jungle_planks",
  "acacia_planks",
  "dark_oak_planks",
  "stick",
]

/** Stream a smelt-all task: load all of an item into a nearby furnace and collect output. */
export function processAll(call: ServerWritableStream<ProcessAllRequest, TaskProgress>): void {
  const bot = requireBotOrThrow()
  runTask(call, "process_all", async tc => runProcessAll(tc, call.request, bot), bot)
}

async function runProcessAll(
  tc: TaskContext,
  req: ProcessAllRequest,
  bot: Bot,
): Promise<TaskResult> {
  if (!req.item) return fail("item required")

  const item = findItemInInventory(bot, req.item)
  if (!item) {
    return fail(`No ${req.item} in inventory`)
  }
  const totalItems = item.count

  const mcData = bot.registry as unknown as IndexedData
  const furnaceIds: number[] = []
  for (const name of ["furnace", "blast_furnace", "smoker"]) {
    const info = mcData.blocksByName[name]
    if (info) furnaceIds.push(info.id)
  }
  const furnacePos = bot.findBlock({ matching: furnaceIds, maxDistance: 32 })
  if (!furnacePos) {
    return fail("No furnace within 32 blocks")
  }

  bot.pathfinder.setMovements(safeMovements(bot))
  try {
    await gotoWithTimeout(
      bot,
      new goals.GoalNear(furnacePos.position.x, furnacePos.position.y, furnacePos.position.z, 3),
    )
  } catch (err) {
    return fail(`Path to furnace failed: ${err instanceof Error ? err.message : String(err)}`)
  }
  if (tc.cancelled()) return fail("cancelled")

  const furnaceBlock = bot.blockAt(furnacePos.position)
  if (!furnaceBlock) return fail("Furnace disappeared")

  const furnace = await (bot as any).openFurnace(furnaceBlock)

  // Pick fuel from the furnace window's inventory slots -- the same slot
  // range that putFuel/putInput search, avoiding bot.inventory mismatch.
  let fuelItem
  for (const fuelName of PROCESS_FUEL_ORDER) {
    fuelItem = findItemInWindow(furnace, fuelName)
    if (fuelItem) break
  }
  if (!fuelItem) {
    furnace.close()
    return fail("No fuel (coal/charcoal/planks/etc.) in inventory")
  }

  const fuelNeeded = Math.max(1, Math.ceil(totalItems / 8))
  try {
    await furnace.putFuel(fuelItem.type, fuelItem.metadata, Math.min(fuelItem.count, fuelNeeded))
    const inputItem = findItemInWindow(furnace, req.item)
    if (!inputItem) {
      furnace.close()
      return fail(`${req.item} not found in inventory`)
    }
    await furnace.putInput(inputItem.type, inputItem.metadata, totalItems)
  } catch (err) {
    furnace.close()
    return fail(`Furnace load failed: ${err instanceof Error ? err.message : String(err)}`)
  }

  tc.progress(`Smelting ${totalItems} ${req.item}...`, 0, totalItems)

  const before = snapshotInventory(bot)
  const deadline = Date.now() + totalItems * 12000 + 15000
  let lastProgress = 0

  while (Date.now() < deadline) {
    if (tc.cancelled()) break
    const out = furnace.outputItem()
    if (out && out.count > 0) {
      try {
        await furnace.takeOutput()
      } catch {
        // Empty snap -- keep polling.
      }
      const current = countGainSince(bot, before)
      if (current !== lastProgress) {
        lastProgress = current
        tc.progress(`Smelted ${current}/${totalItems}`, current, totalItems)
      }
      if (current >= totalItems) break
    }
    await sleep(500)
  }

  furnace.close()

  const gained = computeGains(bot, before)
  const used = inventoryDelta(bot, before)

  return {
    success: lastProgress >= totalItems,
    summary:
      lastProgress >= totalItems
        ? `Smelted ${totalItems} ${req.item}`
        : `Smelted ${lastProgress}/${totalItems} ${req.item} before timing out`,
    itemsGained: gained,
    itemsUsed: used,
  }
}

// ---------------------------------------------------------------------------
// OrganizeInventory -- hotbar placement + optional chest stash of junk items.
// ---------------------------------------------------------------------------

const JUNK_NAMES = new Set([
  "dirt",
  "gravel",
  "cobblestone",
  "andesite",
  "diorite",
  "granite",
  "rotten_flesh",
  "poisonous_potato",
  "string",
  "bone",
  "feather",
  "spider_eye",
])

const PRIORITY_HOTBAR = [
  "diamond_sword",
  "iron_sword",
  "stone_sword",
  "wooden_sword",
  "diamond_pickaxe",
  "iron_pickaxe",
  "stone_pickaxe",
  "wooden_pickaxe",
  "diamond_axe",
  "iron_axe",
  "stone_axe",
  "wooden_axe",
  "shield",
  "bow",
  "crossbow",
]

/** Stream an inventory-organize task: arrange priority tools in the hotbar and optionally stash junk. */
export function organizeInventory(
  call: ServerWritableStream<OrganizeInventoryRequest, TaskProgress>,
): void {
  const bot = requireBotOrThrow()
  runTask(call, "organize_inventory", async tc => runOrganize(tc, call.request, bot), bot)
}

async function runOrganize(
  tc: TaskContext,
  req: OrganizeInventoryRequest,
  bot: Bot,
): Promise<TaskResult> {
  let movedToHotbar = 0
  let stashed = 0

  // Build ordered list of preferred hotbar items we currently own.
  const inventory = bot.inventory.items()
  const pickPreferred: typeof inventory = []
  for (const name of PRIORITY_HOTBAR) {
    const match = inventory.find(i => i.name === name)
    if (match) pickPreferred.push(match)
    if (pickPreferred.length >= 9) break
  }

  // Hotbar slots in mineflayer are 36-44.
  const HOTBAR_BASE = 36
  for (let i = 0; i < pickPreferred.length; i++) {
    if (tc.cancelled()) break
    const targetSlot = HOTBAR_BASE + i
    const item = pickPreferred[i]
    if (item.slot === targetSlot) continue
    try {
      await bot.moveSlotItem(item.slot, targetSlot)
      movedToHotbar++
      tc.progress(`Moved ${item.name} to hotbar slot ${i + 1}`, movedToHotbar, pickPreferred.length)
    } catch {
      // ignore conflicts; best-effort
    }
  }

  if (req.stashJunk) {
    const mcData = bot.registry as unknown as IndexedData
    const chestInfo = mcData.blocksByName["chest"]
    const barrelInfo = mcData.blocksByName["barrel"]
    const containerIds = [chestInfo?.id, barrelInfo?.id].filter(
      (x): x is number => typeof x === "number",
    )

    const containerPos =
      containerIds.length > 0 ? bot.findBlock({ matching: containerIds, maxDistance: 24 }) : null

    if (containerPos) {
      bot.pathfinder.setMovements(safeMovements(bot))
      try {
        await gotoWithTimeout(
          bot,
          new goals.GoalNear(
            containerPos.position.x,
            containerPos.position.y,
            containerPos.position.z,
            3,
          ),
        )
      } catch {
        // Couldn't path -- fall through.
      }
      if (!tc.cancelled()) {
        const chestBlock = bot.blockAt(containerPos.position)
        if (chestBlock) {
          const chest = await bot.openContainer(chestBlock)
          try {
            for (const item of bot.inventory.items()) {
              if (tc.cancelled()) break
              if (!JUNK_NAMES.has(item.name)) continue
              try {
                await chest.deposit(item.type, null, item.count)
                stashed += item.count
                tc.progress(`Stashed ${item.count}× ${item.name}`, stashed, stashed)
              } catch {
                // Chest full or conflict; move on.
              }
            }
          } finally {
            chest.close()
          }
        }
      }
    }
  }

  const summary = req.stashJunk
    ? `Organized: moved ${movedToHotbar} items to hotbar, stashed ${stashed} junk items`
    : `Organized: moved ${movedToHotbar} items to hotbar`

  return {
    success: true,
    summary,
    itemsGained: [],
    itemsUsed: [],
  }
}

// ---------------------------------------------------------------------------
// ClearArea -- spiral outward, dig every non-air block at bot Y..Y+2.
// ---------------------------------------------------------------------------

/** Stream an area-clearing task: dig all non-air blocks within radius at the bot's y-level. */
export function clearArea(call: ServerWritableStream<ClearAreaRequest, TaskProgress>): void {
  const bot = requireBotOrThrow()
  runTask(call, "clear_area", async tc => runClearArea(tc, call.request, bot), bot)
}

async function runClearArea(tc: TaskContext, req: ClearAreaRequest, bot: Bot): Promise<TaskResult> {
  if (!req.center) return fail("center required")
  const radius = req.radius > 0 ? req.radius : 4

  const center = new Vec3(
    Math.floor(req.center.x),
    Math.floor(req.center.y),
    Math.floor(req.center.z),
  )
  const spiral = generateSpiral(center, radius)
  const yRange = [0, 1, 2]
  const estimated = spiral.length * yRange.length

  let cleared = 0
  let checked = 0

  bot.pathfinder.setMovements(safeMovements(bot))

  for (const xy of spiral) {
    if (tc.cancelled()) break
    for (const yOff of yRange) {
      if (tc.cancelled()) break
      checked++
      const pos = new Vec3(xy.x, center.y + yOff, xy.z)
      const block = bot.blockAt(pos)
      if (!block || block.name === "air" || isLiquid(block.name)) continue

      try {
        await gotoWithTimeout(bot, new goals.GoalNear(pos.x, pos.y, pos.z, 4))
      } catch {
        continue
      }
      if (tc.cancelled()) break

      const liveBlock = bot.blockAt(pos)
      if (!liveBlock || liveBlock.name === "air") continue

      try {
        await bot.tool.equipForBlock(liveBlock)
        await bot.dig(liveBlock)
        cleared++
        tc.progress(
          `Cleared ${cleared} blocks (${checked}/${estimated} checked)`,
          cleared,
          estimated,
        )
      } catch {
        // Tool/dig failure -- move on.
        continue
      }
    }
  }

  return {
    success: !tc.cancelled(),
    summary: `Cleared ${cleared} blocks within radius ${radius}`,
    itemsGained: [],
    itemsUsed: [],
  }
}

function generateSpiral(center: Vec3, radius: number): Array<{ x: number; z: number }> {
  const points: Array<{ x: number; z: number }> = [{ x: center.x, z: center.z }]
  let x = 0
  let z = 0
  let dx = 0
  let dz = -1
  const max = (radius * 2 + 1) * (radius * 2 + 1)
  for (let i = 0; i < max; i++) {
    if (-radius <= x && x <= radius && -radius <= z && z <= radius) {
      points.push({ x: center.x + x, z: center.z + z })
    }
    if (x === z || (x < 0 && x === -z) || (x > 0 && x === 1 - z)) {
      const tmp = dx
      dx = -dz
      dz = tmp
    }
    x += dx
    z += dz
  }
  // Dedupe center (already added first).
  const seen = new Set<string>()
  return points.filter(p => {
    const key = `${p.x},${p.z}`
    if (seen.has(key)) return false
    seen.add(key)
    return true
  })
}

// ---------------------------------------------------------------------------
// Farm -- harvest mature crops, replant, optionally till nearby dirt.
// ---------------------------------------------------------------------------

interface CropSpec {
  seed: string
  crop: string
  matureAge: number
}

const CROPS: Record<string, CropSpec> = {
  wheat: { seed: "wheat_seeds", crop: "wheat", matureAge: 7 },
  carrots: { seed: "carrot", crop: "carrots", matureAge: 7 },
  potatoes: { seed: "potato", crop: "potatoes", matureAge: 7 },
  beetroots: { seed: "beetroot_seeds", crop: "beetroots", matureAge: 3 },
}

/** Stream a farm-cycle task: harvest mature crops and replant seeds on empty farmland. */
export function farm(call: ServerWritableStream<FarmRequest, TaskProgress>): void {
  const bot = requireBotOrThrow()
  runTask(call, "farm", async tc => runFarm(tc, call.request, bot), bot)
}

async function runFarm(tc: TaskContext, req: FarmRequest, bot: Bot): Promise<TaskResult> {
  if (!req.location) return fail("location required")
  const spec = CROPS[req.crop]
  if (!spec) {
    return fail(`Unsupported crop "${req.crop}". Supported: ${Object.keys(CROPS).join(", ")}`)
  }
  const radius = req.radius > 0 ? req.radius : 8

  const mcData = bot.registry as unknown as IndexedData
  const cropInfo = mcData.blocksByName[spec.crop]
  const farmlandInfo = mcData.blocksByName["farmland"]
  if (!cropInfo || !farmlandInfo) {
    return fail("Farmland/crop block data unavailable")
  }

  const center = new Vec3(
    Math.floor(req.location.x),
    Math.floor(req.location.y),
    Math.floor(req.location.z),
  )

  const before = snapshotInventory(bot)
  let harvested = 0
  let planted = 0

  bot.pathfinder.setMovements(safeMovements(bot))

  // Pass 1: harvest mature crops.
  const crops = bot.findBlocks({
    matching: cropInfo.id,
    maxDistance: radius,
    count: 64,
    point: center,
  })
  const total = crops.length
  tc.progress(`Found ${total} ${spec.crop} blocks`, 0, total)

  for (const pos of crops) {
    if (tc.cancelled()) break
    const block = bot.blockAt(pos)
    if (!block) continue
    const age = ageOf(block)
    if (age < spec.matureAge) continue

    try {
      await gotoWithTimeout(bot, new goals.GoalNear(pos.x, pos.y, pos.z, 3))
      if (tc.cancelled()) break
      const live = bot.blockAt(pos)
      if (!live) continue
      await bot.dig(live)
      harvested++
      tc.progress(`Harvested ${harvested} ${spec.crop}`, harvested, total)
    } catch {
      continue
    }
  }

  // Pass 2: replant on empty farmland.
  if (!tc.cancelled()) {
    const farmland = bot.findBlocks({
      matching: farmlandInfo.id,
      maxDistance: radius,
      count: 64,
      point: center,
    })
    for (const pos of farmland) {
      if (tc.cancelled()) break
      const above = bot.blockAt(pos.offset(0, 1, 0))
      if (above && above.name !== "air") continue
      const seed = findItemInInventory(bot, spec.seed)
      if (!seed) break

      try {
        await gotoWithTimeout(bot, new goals.GoalNear(pos.x, pos.y, pos.z, 3))
        if (tc.cancelled()) break
        const farmlandBlock = bot.blockAt(pos)
        if (!farmlandBlock) continue
        await bot.equip(seed, "hand")
        await bot.placeBlock(farmlandBlock, new Vec3(0, 1, 0))
        planted++
        tc.progress(`Planted ${planted} ${spec.seed}`, planted, farmland.length)
      } catch {
        continue
      }
    }
  }

  return {
    success: true,
    summary: `Farm cycle: harvested ${harvested} ${spec.crop}, planted ${planted} ${spec.seed}`,
    itemsGained: computeGains(bot, before),
    itemsUsed: inventoryDelta(bot, before),
  }
}

function ageOf(block: MfBlock): number {
  const state = (
    block as unknown as { getProperties?: () => Record<string, unknown> }
  ).getProperties?.()
  const age = state?.age
  if (typeof age === "number") return age
  if (typeof age === "string") return parseInt(age, 10) || 0
  // Fall back to metadata.
  const meta = (block as unknown as { metadata?: number }).metadata ?? 0
  return meta
}
