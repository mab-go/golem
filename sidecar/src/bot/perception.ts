import type { Bot } from "mineflayer"
import type { Item } from "prismarine-item"
import type { IndexedData } from "minecraft-data"
import type {
  GetVitalSignsResponse,
  GetSurroundingsResponse,
  GetInventoryResponse,
  InventoryItem as ProtoItem,
  StatusEffect as ProtoEffect,
  Entity as ProtoEntity,
  Block as ProtoBlock,
} from "../grpc/generated/minecraft.js"
import { validatePosition, yieldToEventLoop } from "./actions/helpers.js"

// ---------------------------------------------------------------------------
// Vital Signs
// ---------------------------------------------------------------------------

/** Read current health, hunger, armor, XP, held items, and active effects. */
export function readVitalSigns(bot: Bot): GetVitalSignsResponse {
  const mcData = bot.registry as unknown as IndexedData

  return {
    health: bot.health,
    maxHealth: 20,
    food: bot.food,
    foodSaturation: bot.foodSaturation,
    oxygen: bot.oxygenLevel ?? 300,
    armor: sumArmorPoints(bot),
    xpLevel: bot.experience.level,
    xpProgress: bot.experience.progress,
    gameMode: bot.game.gameMode,
    mainHand: mapItem(bot.heldItem),
    offHand: mapItem(bot.inventory.slots[45] ?? null),
    effects: mapEffects(bot.entity.effects, mcData),
  }
}

// ---------------------------------------------------------------------------
// Surroundings
// ---------------------------------------------------------------------------

/** Read position, biome, time, nearby entities, and notable blocks within radius. */
export async function readSurroundings(bot: Bot, radius: number): Promise<GetSurroundingsResponse> {
  validatePosition(bot)
  const pos = bot.entity.position
  const blockAtBot = bot.blockAt(pos)
  const dimension = bot.game.dimension.replace(/^minecraft:/, "")

  return {
    position: { x: pos.x, y: pos.y, z: pos.z },
    biome: blockAtBot?.biome?.name ?? "unknown",
    timeOfDay: ticksToPhase(bot.time.timeOfDay),
    isRaining: bot.isRaining,
    isThundering: bot.thunderState > 0,
    lightLevel: effectiveLightLevel(
      blockAtBot?.light ?? 0,
      blockAtBot?.skyLight ?? 0,
      bot.time.timeOfDay,
    ),
    dimension,
    yaw: bot.entity.yaw,
    pitch: bot.entity.pitch,
    nearbyEntities: gatherNearbyEntities(bot, radius),
    notableBlocks: await gatherNotableBlocks(bot, radius),
  }
}

// ---------------------------------------------------------------------------
// Inventory
// ---------------------------------------------------------------------------

/** Read inventory items, empty slot count, armor, and optionally craft suggestions. */
export async function readInventory(
  bot: Bot,
  includeCraftSuggestions: boolean,
): Promise<GetInventoryResponse> {
  const items = bot.inventory.items()
  const mapped = items.map(i => mapItem(i)!)
  const armorSlots = bot.inventory.slots.slice(5, 9)
  const armorItems = armorSlots.filter(Boolean).map(i => mapItem(i)!)

  return {
    items: mapped,
    emptySlots: 36 - items.length,
    armorItems,
    craftSuggestions: includeCraftSuggestions ? await getCraftSuggestions(bot) : [],
  }
}

// ---------------------------------------------------------------------------
// Helpers -- Items
// ---------------------------------------------------------------------------

function mapItem(item: Item | null): ProtoItem | undefined {
  if (!item) return undefined
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
// Helpers -- Armor
// ---------------------------------------------------------------------------

/** Armor point values by material for each slot type. */
export const ARMOR_POINTS: Record<string, number> = {
  leather_helmet: 1,
  leather_chestplate: 3,
  leather_leggings: 2,
  leather_boots: 1,
  chainmail_helmet: 2,
  chainmail_chestplate: 5,
  chainmail_leggings: 4,
  chainmail_boots: 1,
  iron_helmet: 2,
  iron_chestplate: 6,
  iron_leggings: 5,
  iron_boots: 2,
  golden_helmet: 2,
  golden_chestplate: 5,
  golden_leggings: 3,
  golden_boots: 1,
  diamond_helmet: 3,
  diamond_chestplate: 8,
  diamond_leggings: 6,
  diamond_boots: 3,
  netherite_helmet: 3,
  netherite_chestplate: 8,
  netherite_leggings: 6,
  netherite_boots: 3,
  turtle_helmet: 2,
}

function sumArmorPoints(bot: Bot): number {
  let total = 0
  for (let slot = 5; slot <= 8; slot++) {
    const item = bot.inventory.slots[slot]
    if (item) {
      total += ARMOR_POINTS[item.name] ?? 0
    }
  }
  return total
}

// ---------------------------------------------------------------------------
// Helpers -- Effects
// ---------------------------------------------------------------------------

interface MineflayerEffect {
  id: number
  amplifier: number
  duration: number
}

function mapEffects(
  effects: MineflayerEffect[] | Record<string, MineflayerEffect> | null | undefined,
  mcData: IndexedData,
): ProtoEffect[] {
  if (effects == null) return []
  const list = Array.isArray(effects) ? effects : Object.values(effects)
  return list.map(e => {
    const info = (mcData.effects as Record<number, { name: string }>)?.[e.id]
    return {
      name: info?.name ?? `effect_${e.id}`,
      amplifier: e.amplifier,
      durationTicks: e.duration,
    }
  })
}

// ---------------------------------------------------------------------------
// Helpers -- Time
// ---------------------------------------------------------------------------

function ticksToPhase(ticks: number): string {
  // MC day cycle: 0 = dawn (06:00), 6000 = noon (12:00), 12000 = dusk (18:00), 18000 = midnight (00:00)
  if (ticks < 1000) return "dawn"
  if (ticks < 6000) return "morning"
  if (ticks < 12000) return "noon"
  if (ticks < 13000) return "afternoon"
  if (ticks < 13800) return "dusk"
  if (ticks < 22300) return "night"
  if (ticks < 23000) return "midnight"
  return "dawn" // 23000-24000 wraps to dawn
}

// Approximate vanilla sky-light reduction by time of day (0 = no reduction, 11 = full night).
function skyDarkening(timeOfDay: number): number {
  if (timeOfDay < 12542) return 0
  if (timeOfDay < 13000) return 2
  if (timeOfDay < 13800) return 6
  if (timeOfDay < 22200) return 11
  if (timeOfDay < 23000) return 6
  if (timeOfDay < 23600) return 2
  return 0
}

/** Compute effective light level accounting for sky-light reduction at night. */
export function effectiveLightLevel(
  blockLight: number,
  skyLight: number,
  timeOfDay: number,
): number {
  const effectiveSky = Math.max(0, skyLight - skyDarkening(timeOfDay))
  return Math.max(blockLight, effectiveSky)
}

// ---------------------------------------------------------------------------
// Helpers -- Entities
// ---------------------------------------------------------------------------

function gatherNearbyEntities(bot: Bot, radius: number): ProtoEntity[] {
  const botPos = bot.entity.position
  const result: ProtoEntity[] = []

  for (const entity of Object.values(bot.entities)) {
    if (entity === bot.entity) continue
    const dist = entity.position.distanceTo(botPos)
    if (dist > radius) continue

    result.push({
      id: entity.id,
      type: entity.name ?? entity.type ?? "unknown",
      name: entity.username ?? "",
      position: {
        x: entity.position.x,
        y: entity.position.y,
        z: entity.position.z,
      },
      distance: Math.round(dist * 10) / 10,
      health: entity.health ?? 0,
    })
  }

  // Sort by distance, closest first.
  result.sort((a, b) => a.distance - b.distance)
  return result
}

// ---------------------------------------------------------------------------
// Helpers -- Notable Blocks
// ---------------------------------------------------------------------------

interface BlockCategory {
  blocks: string[]
  budget: number
  addBedVariants?: boolean
}

const NOTABLE_BLOCK_CATEGORIES: BlockCategory[] = [
  // "What should I mine?" -- highest value for decision-making.
  {
    blocks: [
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
      "nether_gold_ore",
      "nether_quartz_ore",
    ],
    budget: 16,
  },
  // "What should I explore?" -- rare, game-changing finds.
  {
    blocks: [
      "chest",
      "trapped_chest",
      "barrel",
      "ender_chest",
      "spawner",
      "end_portal_frame",
      "nether_portal",
    ],
    budget: 6,
  },
  // "What's dangerous here?" -- survival-critical awareness.
  {
    blocks: ["lava", "fire", "magma_block", "cactus", "sweet_berry_bush"],
    budget: 8,
  },
  // "What can I use?" -- crafting/smelting availability + beds.
  {
    blocks: [
      "crafting_table",
      "furnace",
      "blast_furnace",
      "smoker",
      "anvil",
      "enchanting_table",
      "brewing_stand",
      "grindstone",
      "smithing_table",
      "loom",
      "cartography_table",
      "stonecutter",
      "bed",
    ],
    budget: 10,
    addBedVariants: true,
  },
  // Low-value ambient awareness -- small budget so torches never crowd out ores.
  {
    blocks: ["torch", "soul_torch", "campfire", "soul_campfire"],
    budget: 4,
  },
]

async function gatherNotableBlocks(bot: Bot, radius: number): Promise<ProtoBlock[]> {
  const mcData = bot.registry as unknown as IndexedData
  const botPos = bot.entity.position
  const result: ProtoBlock[] = []

  for (const category of NOTABLE_BLOCK_CATEGORIES) {
    const matchIds: number[] = []
    for (const name of category.blocks) {
      const block = mcData.blocksByName[name]
      if (block) matchIds.push(block.id)
    }

    if (category.addBedVariants) {
      for (const name of Object.keys(mcData.blocksByName)) {
        if (name.endsWith("_bed") && !matchIds.includes(mcData.blocksByName[name].id)) {
          matchIds.push(mcData.blocksByName[name].id)
        }
      }
    }

    if (matchIds.length === 0) continue

    const positions = bot.findBlocks({
      matching: matchIds,
      maxDistance: radius,
      count: category.budget,
    })

    for (const pos of positions) {
      const block = bot.blockAt(pos)
      if (!block) continue
      result.push({
        type: block.name,
        position: { x: pos.x, y: pos.y, z: pos.z },
        distance: Math.round(pos.distanceTo(botPos) * 10) / 10,
      })
    }

    await yieldToEventLoop()
  }

  result.sort((a, b) => a.distance - b.distance)
  return result
}

// ---------------------------------------------------------------------------
// Helpers -- Craft Suggestions
// ---------------------------------------------------------------------------

/** Items to check for craft suggestions, ordered by general usefulness. */
const CRAFT_CHECK_ITEMS = [
  "crafting_table",
  "stick",
  "wooden_pickaxe",
  "wooden_axe",
  "wooden_sword",
  "wooden_shovel",
  "stone_pickaxe",
  "stone_axe",
  "stone_sword",
  "stone_shovel",
  "iron_pickaxe",
  "iron_axe",
  "iron_sword",
  "iron_shovel",
  "furnace",
  "chest",
  "torch",
  "oak_planks",
  "spruce_planks",
  "birch_planks",
  "jungle_planks",
  "acacia_planks",
  "dark_oak_planks",
  "bucket",
  "shield",
  "bread",
  "bowl",
  "boat",
]

const MAX_CRAFT_SUGGESTIONS = 10

async function getCraftSuggestions(bot: Bot): Promise<GetInventoryResponse["craftSuggestions"]> {
  const mcData = bot.registry as unknown as IndexedData
  const suggestions: GetInventoryResponse["craftSuggestions"] = []

  for (let i = 0; i < CRAFT_CHECK_ITEMS.length; i++) {
    if (i > 0 && i % 7 === 0) await yieldToEventLoop()
    if (suggestions.length >= MAX_CRAFT_SUGGESTIONS) break
    const name = CRAFT_CHECK_ITEMS[i]
    const itemInfo = mcData.itemsByName[name]
    if (!itemInfo) continue

    // Check recipes without a crafting table first, then with.
    const recipesHand = bot.recipesFor(itemInfo.id, null, 1, false)
    const recipesTable = bot.recipesFor(itemInfo.id, null, 1, true)
    const recipes = recipesHand.length > 0 ? recipesHand : recipesTable

    if (recipes.length > 0) {
      // Estimate max count from the first recipe.
      const recipe = recipes[0]
      suggestions.push({
        itemName: name,
        maxCount: recipe.result?.count ?? 1,
        requiresCraftingTable: recipesHand.length === 0,
      })
    }
  }

  return suggestions
}
