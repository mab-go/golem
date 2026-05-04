import type { Bot } from "mineflayer"
import type { IndexedData } from "minecraft-data"
import pathfinderPkg from "mineflayer-pathfinder"
const { goals, Movements } = pathfinderPkg
import { Vec3 } from "vec3"
import type {
  SurveyAreaRequest,
  SurveyAreaResponse,
  BlockCluster,
  Structure,
  FindNearestRequest,
  FindNearestResponse,
  WhatCanICraftRequest,
  WhatCanICraftResponse,
  CraftableItem,
  AssessThreatRequest,
  AssessThreatResponse,
  PlanPathRequest,
  PlanPathResponse,
  Entity as ProtoEntity,
} from "../../grpc/generated/minecraft.js"
import { ThreatLevel } from "../../grpc/generated/minecraft.js"
import { findEntityByName, safeMovements, yieldToEventLoop } from "./helpers.js"
import { effectiveLightLevel } from "../perception.js"

// ---------------------------------------------------------------------------
// SurveyArea -- cluster notable blocks, list entities, detect structures.
// ---------------------------------------------------------------------------

const SURVEY_BLOCK_NAMES = [
  "coal_ore",
  "deepslate_coal_ore",
  "iron_ore",
  "deepslate_iron_ore",
  "gold_ore",
  "deepslate_gold_ore",
  "diamond_ore",
  "deepslate_diamond_ore",
  "emerald_ore",
  "deepslate_emerald_ore",
  "lapis_ore",
  "deepslate_lapis_ore",
  "redstone_ore",
  "deepslate_redstone_ore",
  "copper_ore",
  "deepslate_copper_ore",
  "ancient_debris",
  "oak_log",
  "birch_log",
  "spruce_log",
  "jungle_log",
  "acacia_log",
  "dark_oak_log",
  "mangrove_log",
  "cherry_log",
  "water",
  "lava",
  "chest",
  "furnace",
  "crafting_table",
  "spawner",
]

const SURVEY_MAX_BLOCKS_PER_TYPE = 64
const SURVEY_MAX_RADIUS = 128

/** Scan the area for notable block clusters, nearby entities, and heuristic structures. */
export async function surveyArea(bot: Bot, req: SurveyAreaRequest): Promise<SurveyAreaResponse> {
  const radius = Math.min(req.radius > 0 ? req.radius : 32, SURVEY_MAX_RADIUS)
  const mcData = bot.registry as unknown as IndexedData
  const botPos = bot.entity.position

  const blockClusters: BlockCluster[] = []
  for (const name of SURVEY_BLOCK_NAMES) {
    const info = mcData.blocksByName[name]
    if (!info) continue
    const positions = bot.findBlocks({
      matching: info.id,
      maxDistance: radius,
      count: SURVEY_MAX_BLOCKS_PER_TYPE,
    })
    if (positions.length === 0) continue

    let cx = 0
    let cy = 0
    let cz = 0
    for (const p of positions) {
      cx += p.x
      cy += p.y
      cz += p.z
    }
    blockClusters.push({
      blockType: name,
      count: positions.length,
      center: {
        x: cx / positions.length,
        y: cy / positions.length,
        z: cz / positions.length,
      },
    })
    await yieldToEventLoop()
  }

  const entities: ProtoEntity[] = []
  for (const entity of Object.values(bot.entities)) {
    if (entity === bot.entity) continue
    const dist = entity.position.distanceTo(botPos)
    if (dist > radius) continue
    entities.push({
      id: entity.id,
      type: entity.name ?? entity.type ?? "unknown",
      name: entity.username ?? "",
      position: { x: entity.position.x, y: entity.position.y, z: entity.position.z },
      distance: Math.round(dist * 10) / 10,
      health: entity.health ?? 0,
    })
  }
  entities.sort((a, b) => a.distance - b.distance)

  const structures = await detectStructures(bot, radius)
  const summary = buildSurveySummary(blockClusters, entities, structures)

  return { blockClusters, entities, structures, summary }
}

async function detectStructures(bot: Bot, requestedRadius: number): Promise<Structure[]> {
  const radius = Math.min(requestedRadius > 0 ? requestedRadius : 64, SURVEY_MAX_RADIUS)
  const mcData = bot.registry as unknown as IndexedData
  const botPos = bot.entity.position
  const structures: Structure[] = []

  // Village heuristic: cluster of beds + doors.
  const bedIds: number[] = []
  const doorIds: number[] = []
  for (const name of Object.keys(mcData.blocksByName)) {
    if (name.endsWith("_bed")) bedIds.push(mcData.blocksByName[name].id)
    if (name.endsWith("_door") && !name.includes("trapdoor")) {
      doorIds.push(mcData.blocksByName[name].id)
    }
  }
  if (bedIds.length > 0 && doorIds.length > 0) {
    const beds = bot.findBlocks({ matching: bedIds, maxDistance: radius, count: 16 })
    await yieldToEventLoop()
    const doors = bot.findBlocks({ matching: doorIds, maxDistance: radius, count: 32 })
    await yieldToEventLoop()
    if (beds.length >= 2 && doors.length >= 4) {
      const avg = averagePosition([...beds, ...doors])
      structures.push({
        type: "village",
        position: avg,
        distance: Math.round(new Vec3(avg.x, avg.y, avg.z).distanceTo(botPos) * 10) / 10,
      })
    }
  }

  // Mineshaft heuristic: rails.
  const railIds: number[] = []
  for (const name of ["rail", "powered_rail", "detector_rail", "activator_rail"]) {
    const info = mcData.blocksByName[name]
    if (info) railIds.push(info.id)
  }
  if (railIds.length > 0) {
    const rails = bot.findBlocks({ matching: railIds, maxDistance: radius, count: 8 })
    await yieldToEventLoop()
    if (rails.length >= 3) {
      const avg = averagePosition(rails)
      structures.push({
        type: "mineshaft",
        position: avg,
        distance: Math.round(new Vec3(avg.x, avg.y, avg.z).distanceTo(botPos) * 10) / 10,
      })
    }
  }

  return structures
}

function averagePosition(positions: Vec3[]): { x: number; y: number; z: number } {
  let x = 0
  let y = 0
  let z = 0
  for (const p of positions) {
    x += p.x
    y += p.y
    z += p.z
  }
  const n = positions.length
  return { x: x / n, y: y / n, z: z / n }
}

function buildSurveySummary(
  clusters: BlockCluster[],
  entities: ProtoEntity[],
  structures: Structure[],
): string {
  const parts: string[] = []
  if (clusters.length > 0) {
    const top = clusters
      .slice()
      .sort((a, b) => b.count - a.count)
      .slice(0, 3)
      .map(c => `${c.blockType}×${c.count}`)
      .join(", ")
    parts.push(`blocks: ${top}`)
  }
  if (entities.length > 0) {
    parts.push(`${entities.length} entities nearby`)
  }
  if (structures.length > 0) {
    parts.push(`structures: ${structures.map(s => s.type).join(", ")}`)
  }
  return parts.length > 0 ? parts.join("; ") : "Area appears empty."
}

// ---------------------------------------------------------------------------
// FindNearest -- block / entity / structure.
// ---------------------------------------------------------------------------

/** Find the nearest block, entity, or structure of the requested type within radius. */
export async function findNearest(bot: Bot, req: FindNearestRequest): Promise<FindNearestResponse> {
  const radius = req.radius > 0 ? req.radius : 64

  if (req.blockType) {
    const info = (bot.registry as unknown as IndexedData).blocksByName[req.blockType]
    if (!info) {
      return { found: false, type: req.blockType, position: undefined, distance: 0 }
    }
    const pos = bot.findBlock({ matching: info.id, maxDistance: radius })
    if (!pos) {
      return { found: false, type: req.blockType, position: undefined, distance: 0 }
    }
    return {
      found: true,
      type: req.blockType,
      position: { x: pos.position.x, y: pos.position.y, z: pos.position.z },
      distance: Math.round(pos.position.distanceTo(bot.entity.position) * 10) / 10,
    }
  }

  if (req.entityType) {
    const entity = findEntityByName(bot, req.entityType)
    if (!entity) {
      return { found: false, type: req.entityType, position: undefined, distance: 0 }
    }
    const dist = entity.position.distanceTo(bot.entity.position)
    if (dist > radius) {
      return { found: false, type: req.entityType, position: undefined, distance: 0 }
    }
    return {
      found: true,
      type: req.entityType,
      position: { x: entity.position.x, y: entity.position.y, z: entity.position.z },
      distance: Math.round(dist * 10) / 10,
    }
  }

  if (req.structureType) {
    const structures = await detectStructures(bot, radius)
    const match = structures.find(s => s.type === req.structureType)
    if (!match) {
      return { found: false, type: req.structureType, position: undefined, distance: 0 }
    }
    return {
      found: true,
      type: match.type,
      position: match.position,
      distance: match.distance,
    }
  }

  return { found: false, type: "", position: undefined, distance: 0 }
}

// ---------------------------------------------------------------------------
// WhatCanICraft -- enumerate recipes available from inventory.
// ---------------------------------------------------------------------------

const CRAFT_CATALOG = [
  "crafting_table",
  "stick",
  "wooden_pickaxe",
  "wooden_axe",
  "wooden_sword",
  "wooden_shovel",
  "wooden_hoe",
  "stone_pickaxe",
  "stone_axe",
  "stone_sword",
  "stone_shovel",
  "stone_hoe",
  "iron_pickaxe",
  "iron_axe",
  "iron_sword",
  "iron_shovel",
  "iron_hoe",
  "diamond_pickaxe",
  "diamond_axe",
  "diamond_sword",
  "furnace",
  "chest",
  "torch",
  "oak_planks",
  "spruce_planks",
  "birch_planks",
  "jungle_planks",
  "acacia_planks",
  "dark_oak_planks",
  "mangrove_planks",
  "cherry_planks",
  "bread",
  "bowl",
  "bucket",
  "shield",
  "bow",
  "arrow",
  "ladder",
  "fence",
  "door",
  "boat",
]

/** Enumerate craftable items from the current inventory against a curated recipe catalog. */
export async function whatCanICraft(
  bot: Bot,
  _req: WhatCanICraftRequest,
): Promise<WhatCanICraftResponse> {
  const mcData = bot.registry as unknown as IndexedData
  const items: CraftableItem[] = []

  for (let i = 0; i < CRAFT_CATALOG.length; i++) {
    if (i > 0 && i % 10 === 0) await yieldToEventLoop()
    const name = CRAFT_CATALOG[i]
    const itemInfo = mcData.itemsByName[name]
    if (!itemInfo) continue

    const hand = bot.recipesFor(itemInfo.id, null, 1, false)
    const table = bot.recipesFor(itemInfo.id, null, 1, true)
    const recipes = hand.length > 0 ? hand : table
    if (recipes.length === 0) continue

    const recipe = recipes[0]
    const ingredientNames = new Set<string>()
    const delta = recipe.delta ?? []
    for (const d of delta) {
      if (d.count < 0) {
        const ing = mcData.items[d.id]
        if (ing) ingredientNames.add(ing.name)
      }
    }

    items.push({
      itemName: name,
      maxCount: recipe.result?.count ?? 1,
      requiresCraftingTable: hand.length === 0,
      ingredients: Array.from(ingredientNames),
    })
  }

  return { items }
}

// ---------------------------------------------------------------------------
// AssessThreat -- hostile count, nightfall, light.
// ---------------------------------------------------------------------------

const HOSTILE_NAMES = new Set([
  "zombie",
  "skeleton",
  "creeper",
  "spider",
  "cave_spider",
  "enderman",
  "witch",
  "slime",
  "magma_cube",
  "blaze",
  "ghast",
  "wither_skeleton",
  "husk",
  "stray",
  "phantom",
  "drowned",
  "pillager",
  "vindicator",
  "evoker",
  "ravager",
  "vex",
  "guardian",
  "elder_guardian",
  "silverfish",
  "endermite",
  "zoglin",
  "hoglin",
  "piglin_brute",
  "warden",
])

/** Assess nearby hostile entities, local light level, and time until nightfall. */
export async function assessThreat(
  bot: Bot,
  req: AssessThreatRequest,
): Promise<AssessThreatResponse> {
  const radius = req.radius > 0 ? req.radius : 24
  const botPos = bot.entity.position

  const hostiles: ProtoEntity[] = []
  for (const entity of Object.values(bot.entities)) {
    if (entity === bot.entity) continue
    const typeName = entity.name ?? entity.type ?? ""
    if (!HOSTILE_NAMES.has(typeName)) continue
    const dist = entity.position.distanceTo(botPos)
    if (dist > radius) continue
    hostiles.push({
      id: entity.id,
      type: typeName,
      name: entity.username ?? "",
      position: { x: entity.position.x, y: entity.position.y, z: entity.position.z },
      distance: Math.round(dist * 10) / 10,
      health: entity.health ?? 0,
    })
  }
  hostiles.sort((a, b) => a.distance - b.distance)

  const blockAtBot = bot.blockAt(botPos)
  const lightLevel = effectiveLightLevel(
    blockAtBot?.light ?? 0,
    blockAtBot?.skyLight ?? 0,
    bot.time.timeOfDay,
  )
  const timeUntilNightfall = secondsUntilNightfall(bot.time.timeOfDay)

  const threatLevel = computeThreatLevel(hostiles, lightLevel, timeUntilNightfall, bot.health)
  const summary = buildThreatSummary(threatLevel, hostiles, lightLevel, timeUntilNightfall)

  return {
    threatLevel,
    hostileEntities: hostiles,
    timeUntilNightfall,
    localLightLevel: lightLevel,
    summary,
  }
}

function secondsUntilNightfall(timeOfDay: number): number {
  // Night begins ~13000 ticks. 20 ticks/sec.
  if (timeOfDay >= 13000 && timeOfDay < 23000) {
    return -1 // already night
  }
  const ticksUntilNight = timeOfDay < 13000 ? 13000 - timeOfDay : 24000 - timeOfDay + 13000
  return Math.round(ticksUntilNight / 20)
}

function computeThreatLevel(
  hostiles: ProtoEntity[],
  light: number,
  nightfall: number,
  health: number,
): ThreatLevel {
  const closest = hostiles.length > 0 ? hostiles[0].distance : Infinity

  if (closest <= 3 || (health <= 6 && hostiles.length > 0)) return ThreatLevel.THREAT_CRITICAL
  if (closest <= 8 || hostiles.length >= 4) return ThreatLevel.THREAT_HIGH
  if (hostiles.length >= 1) return ThreatLevel.THREAT_MODERATE
  if (light <= 7 || (nightfall !== -1 && nightfall < 60)) return ThreatLevel.THREAT_LOW
  return ThreatLevel.THREAT_SAFE
}

function buildThreatSummary(
  level: ThreatLevel,
  hostiles: ProtoEntity[],
  light: number,
  nightfall: number,
): string {
  const label = ThreatLevel[level].replace("THREAT_", "").toLowerCase()
  const parts = [`Threat: ${label}`]
  if (hostiles.length > 0) {
    const closest = hostiles[0]
    parts.push(
      `${hostiles.length} hostile${hostiles.length === 1 ? "" : "s"} (nearest ${closest.type} ${closest.distance}m)`,
    )
  }
  parts.push(`light ${light}`)
  parts.push(nightfall === -1 ? "night" : `${nightfall}s until night`)
  return parts.join(", ")
}

// ---------------------------------------------------------------------------
// PlanPath -- compute reachability and hazards without walking.
// ---------------------------------------------------------------------------

/** Compute reachability and hazards for a destination without moving the bot. */
export async function planPath(bot: Bot, req: PlanPathRequest): Promise<PlanPathResponse> {
  if (!req.destination) {
    return { reachable: false, distance: 0, estimatedSeconds: 0, hazards: [], waypoints: [] }
  }
  const dest = new Vec3(req.destination.x, req.destination.y, req.destination.z)
  const distance = bot.entity.position.distanceTo(dest)
  const goal = new goals.GoalNear(dest.x, dest.y, dest.z, 1)

  const movements = req.allowDig
    ? (() => {
        const m = new Movements(bot)
        m.allowSprinting = true
        return m
      })()
    : safeMovements(bot)

  const pathfinder = (bot as any).pathfinder
  if (!pathfinder) {
    return {
      reachable: false,
      distance,
      estimatedSeconds: 0,
      hazards: ["pathfinder unavailable"],
      waypoints: [],
    }
  }

  const rawResult = pathfinder.getPathTo(movements, goal, 10000)
  const result =
    typeof (rawResult as Promise<unknown>)?.then === "function" ? await rawResult : rawResult
  const status: string = (result as { status?: string }).status ?? "noPath"
  const path: Array<{ x: number; y: number; z: number }> =
    (result as { path?: Array<{ x: number; y: number; z: number }> }).path ?? []

  const reachable = status === "success"
  const hazards = await scanHazards(bot, path)

  // Sample waypoints: every ~8 steps, capped at 8 points.
  const waypoints: PlanPathResponse["waypoints"] = []
  if (path.length > 0) {
    const step = Math.max(1, Math.floor(path.length / 8))
    for (let i = 0; i < path.length; i += step) {
      const p = path[i]
      waypoints.push({ x: p.x, y: p.y, z: p.z })
    }
  }

  // Rough estimate: 4.3 blocks/sec sprinting, 2x penalty for hazards.
  const pathLen = path.length
  const estimatedSeconds =
    pathLen > 0 ? Math.round((pathLen / 4.3) * (hazards.length > 0 ? 1.3 : 1)) : 0

  return {
    reachable,
    distance: Math.round(distance * 10) / 10,
    estimatedSeconds,
    hazards,
    waypoints,
  }
}

async function scanHazards(
  bot: Bot,
  path: Array<{ x: number; y: number; z: number }>,
): Promise<string[]> {
  const hazards = new Set<string>()
  for (let i = 0; i < path.length; i++) {
    if (i > 0 && i % 50 === 0) await yieldToEventLoop()
    const p = path[i]
    const block = bot.blockAt(new Vec3(p.x, p.y, p.z))
    if (!block) continue
    const name = block.name
    if (name === "lava") hazards.add("lava")
    if (name === "water") hazards.add("water")
    if (name === "fire" || name === "campfire") hazards.add("fire")
    if (name === "magma_block") hazards.add("magma")
    if (name === "cactus") hazards.add("cactus")
  }
  // Cliff detection: any step with >3 block vertical drop between consecutive waypoints.
  for (let i = 1; i < path.length; i++) {
    if (path[i - 1].y - path[i].y > 3) {
      hazards.add("cliff")
      break
    }
  }
  return Array.from(hazards)
}
