import type { Bot } from "mineflayer"
import type { IndexedData } from "minecraft-data"
import type { Block as MfBlock } from "prismarine-block"
import { Vec3 } from "vec3"
import pathfinderPkg from "mineflayer-pathfinder"
import type { goals as goalsNs } from "mineflayer-pathfinder"
const { goals } = pathfinderPkg
import {
  ok,
  fail,
  collectNearbyDrops,
  countItem,
  computeGains,
  digMovements,
  findEntityById,
  findEntityByName,
  findItemInInventory,
  findItemInWindow,
  gotoWithTimeout,
  mapItemToProto,
  safeMovements,
  snapshotInventory,
  trackInventorySettle,
  validatePosition,
  waitForInventorySettle,
  checkGoalReached,
} from "./helpers.js"
import {
  InteractionType,
  type NavigateToRequest,
  type NavigateToResponse,
  type InteractWithEntityRequest,
  type InteractWithEntityResponse,
  type OpenContainerRequest,
  type OpenContainerResponse,
  type WithdrawFromContainerRequest,
  type WithdrawFromContainerResponse,
  type DepositToContainerRequest,
  type DepositToContainerResponse,
  type CraftItemRequest,
  type CraftItemResponse,
  type SmeltItemRequest,
  type SmeltItemResponse,
  type EatRequest,
  type EatResponse,
  type InventoryItem as ProtoItem,
} from "../../grpc/generated/minecraft.js"

// ---------------------------------------------------------------------------
// NavigateTo
// ---------------------------------------------------------------------------

/** Pathfind to a position, entity, or block type with optional terrain-digging. */
export async function navigateTo(bot: Bot, req: NavigateToRequest): Promise<NavigateToResponse> {
  try {
    validatePosition(bot)
    const mcData = bot.registry as unknown as IndexedData
    const startPos = bot.entity.position.clone()
    const range = req.range || 2

    const movements = req.allowDig ? digMovements(bot) : safeMovements(bot)
    bot.pathfinder.setMovements(movements)

    let goal: goalsNs.Goal

    if (req.position !== undefined) {
      const { x, y, z } = req.position
      goal = new goals.GoalNear(x, y, z, range)
    } else if (req.entityName !== undefined) {
      const entity = findEntityByName(bot, req.entityName)
      if (!entity) {
        return {
          result: fail(`entity "${req.entityName}" not found nearby`),
          finalPosition: undefined,
          distanceTraveled: 0,
        }
      }
      const p = entity.position
      goal = new goals.GoalNear(p.x, p.y, p.z, range)
    } else if (req.blockType !== undefined) {
      const blockInfo = mcData.blocksByName[req.blockType]
      if (!blockInfo) {
        return {
          result: fail(`unknown block type "${req.blockType}"`),
          finalPosition: undefined,
          distanceTraveled: 0,
        }
      }
      const block = bot.findBlock({ matching: blockInfo.id, maxDistance: 64 })
      if (!block) {
        return {
          result: fail(`no ${req.blockType} found within 64 blocks`),
          finalPosition: undefined,
          distanceTraveled: 0,
        }
      }
      const p = block.position
      goal = new goals.GoalGetToBlock(p.x, p.y, p.z)
    } else {
      return {
        result: fail("target is required (position, entity_name, or block_type)"),
        finalPosition: undefined,
        distanceTraveled: 0,
      }
    }

    await gotoWithTimeout(bot, goal)

    const endPos = bot.entity.position
    const distanceTraveled = startPos.distanceTo(endPos)

    if (!checkGoalReached(bot, goal)) {
      const hint = req.allowDig
        ? ""
        : " — try allow_dig=true if terrain requires climbing or tunneling"
      return {
        result: fail(
          `pathfinder resolved but bot did not reach goal (traveled ${distanceTraveled.toFixed(1)} blocks)${hint}`,
        ),
        finalPosition: { x: endPos.x, y: endPos.y, z: endPos.z },
        distanceTraveled: Math.round(distanceTraveled * 10) / 10,
      }
    }

    return {
      result: ok(
        `Navigated to (${endPos.x.toFixed(1)}, ${endPos.y.toFixed(1)}, ${endPos.z.toFixed(1)})`,
      ),
      finalPosition: { x: endPos.x, y: endPos.y, z: endPos.z },
      distanceTraveled: Math.round(distanceTraveled * 10) / 10,
    }
  } catch (err) {
    const pos = bot.entity.position
    const msg = err instanceof Error ? err.message : String(err)
    const hint = req.allowDig
      ? ""
      : " — try allow_dig=true if terrain requires climbing or tunneling"
    return {
      result: fail(`${msg}${hint}`),
      finalPosition: { x: pos.x, y: pos.y, z: pos.z },
      distanceTraveled: 0,
    }
  }
}

// ---------------------------------------------------------------------------
// InteractWithEntity
// ---------------------------------------------------------------------------

const HARVEST_TIMEOUT_MS = 15000
const ATTACK_INTERVAL_MS = 500

/** Navigate to and interact with an entity: harvest, attack, feed, trade, mount, or shear. */
export async function interactWithEntity(
  bot: Bot,
  req: InteractWithEntityRequest,
): Promise<InteractWithEntityResponse> {
  try {
    const mcData = bot.registry as unknown as IndexedData

    // Resolve entity.
    const entity =
      req.entityId > 0 ? findEntityById(bot, req.entityId) : findEntityByName(bot, req.entityName)

    if (!entity) {
      const target = req.entityId > 0 ? `ID ${req.entityId}` : `"${req.entityName}"`
      return { result: fail(`entity ${target} not found`), drops: [], description: "" }
    }

    // Navigate within range. Walk-only -- digging through stone to reach an
    // entity is never the right move.
    bot.pathfinder.setMovements(safeMovements(bot))
    const ep = entity.position
    await gotoWithTimeout(bot, new goals.GoalNear(ep.x, ep.y, ep.z, 3))

    const entityDesc = entity.name ?? entity.type ?? "entity"

    switch (req.action) {
      case InteractionType.INTERACTION_HARVEST:
        return await harvestEntity(bot, entity, entityDesc)

      case InteractionType.INTERACTION_ATTACK: {
        bot.attack(entity)
        return { result: ok(`Attacked ${entityDesc}`), drops: [], description: "" }
      }

      case InteractionType.INTERACTION_FEED: {
        // Feed with whatever is currently held, or auto-select a food item.
        const heldItem = bot.heldItem
        if (!heldItem) {
          // Try to find food in inventory.
          const food = bot.inventory.items().find(i => {
            const itemData = (mcData as any).foodsByName?.[i.name]
            return !!itemData
          })
          if (!food) {
            return {
              result: fail("no food in inventory to feed entity"),
              drops: [],
              description: "",
            }
          }
          await bot.equip(food, "hand")
        }
        await bot.activateEntity(entity)
        const usedItem = bot.heldItem?.name ?? "food"
        return { result: ok(`Fed ${entityDesc} with ${usedItem}`), drops: [], description: "" }
      }

      case InteractionType.INTERACTION_TRADE: {
        const villager = await (bot as any).openVillager(entity)
        const tradeDescs: string[] = []
        if (villager.trades) {
          for (const trade of villager.trades) {
            const input = trade.inputItem1?.name ?? "?"
            const output = trade.outputItem?.name ?? "?"
            tradeDescs.push(`${input} -> ${output}`)
          }
        }
        villager.close()
        const desc = tradeDescs.length > 0 ? tradeDescs.join("; ") : "No trades available"
        return { result: ok(`Opened trade with ${entityDesc}`), drops: [], description: desc }
      }

      case InteractionType.INTERACTION_MOUNT: {
        bot.mount(entity)
        return { result: ok(`Mounted ${entityDesc}`), drops: [], description: "" }
      }

      case InteractionType.INTERACTION_SHEAR: {
        const shears = findItemInInventory(bot, "shears")
        if (!shears) {
          return { result: fail("shears not found in inventory"), drops: [], description: "" }
        }
        await bot.equip(shears, "hand")
        await bot.activateEntity(entity)
        return { result: ok(`Sheared ${entityDesc}`), drops: [], description: "" }
      }

      default:
        return {
          result: fail(`unknown interaction type: ${req.action}`),
          drops: [],
          description: "",
        }
    }
  } catch (err) {
    return { result: fail(err), drops: [], description: "" }
  }
}

async function harvestEntity(
  bot: Bot,
  entity: { id: number; name?: string | null; type?: string; position: Vec3 },
  entityDesc: string,
): Promise<InteractWithEntityResponse> {
  // Equip best weapon (sword or axe).
  const weapons = bot.inventory
    .items()
    .filter(i => i.name.includes("sword") || i.name.includes("axe"))
  if (weapons.length > 0) {
    // Prefer swords over axes; within category, prefer higher-tier materials.
    const sorted = weapons.sort((a, b) => {
      const aIsSword = a.name.includes("sword") ? 1 : 0
      const bIsSword = b.name.includes("sword") ? 1 : 0
      if (aIsSword !== bIsSword) return bIsSword - aIsSword
      return (b.maxDurability ?? 0) - (a.maxDurability ?? 0)
    })
    await bot.equip(sorted[0], "hand")
  }

  const before = snapshotInventory(bot)
  let deathPos = entity.position.clone()

  const settle = trackInventorySettle(bot, {
    firstEventTimeout: 2000,
    quietPeriod: 150,
    maxWait: 5000,
  })

  // Attack loop until entity is dead or timeout.
  const deadline = Date.now() + HARVEST_TIMEOUT_MS
  while (Date.now() < deadline) {
    const target = bot.entities[entity.id]
    if (!target || !target.isValid) break

    deathPos = target.position.clone()
    bot.attack(target)
    await new Promise(r => setTimeout(r, ATTACK_INTERVAL_MS))
  }

  bot.pathfinder.setMovements(safeMovements(bot))
  await collectNearbyDrops(bot, deathPos)

  await settle.wait()
  const drops = computeGains(bot, before)

  return {
    result: ok(`Harvested ${entityDesc}`),
    drops,
    description:
      drops.length > 0
        ? `Collected: ${drops.map(d => `${d.count}x ${d.name}`).join(", ")}`
        : "No drops collected",
  }
}

// ---------------------------------------------------------------------------
// OpenContainer
// ---------------------------------------------------------------------------

const FURNACE_TYPES = new Set(["furnace", "blast_furnace", "smoker"])

const NON_CONTAINER_HINTS: Record<string, string> = {
  crafting_table: "use craft_item while standing near the crafting table instead",
  enchanting_table: "enchanting is not yet supported",
  anvil: "anvil interaction is not yet supported",
  smithing_table: "smithing is not yet supported",
  grindstone: "grindstone interaction is not yet supported",
  stonecutter: "stonecutter interaction is not yet supported",
  cartography_table: "cartography is not yet supported",
  loom: "loom interaction is not yet supported",
}

interface ResolvedContainer {
  block: MfBlock
  containerType: string
}

async function resolveContainer(
  bot: Bot,
  position: { x: number; y: number; z: number } | undefined,
  blockType: string | undefined,
): Promise<ResolvedContainer | string> {
  const mcData = bot.registry as unknown as IndexedData
  let block: MfBlock | null = null

  if (position !== undefined) {
    const { x, y, z } = position
    block = bot.blockAt(new Vec3(x, y, z))
    if (!block) return `no block at (${x}, ${y}, ${z})`
  } else if (blockType !== undefined) {
    const blockInfo = mcData.blocksByName[blockType]
    if (!blockInfo) return `unknown block type "${blockType}"`
    const found = bot.findBlock({ matching: blockInfo.id, maxDistance: 32 })
    if (!found) return `no ${blockType} found within 32 blocks`
    block = found
  } else {
    return "target is required (position or block_type)"
  }

  bot.pathfinder.setMovements(safeMovements(bot))
  const bp = block.position
  await gotoWithTimeout(bot, new goals.GoalNear(bp.x, bp.y, bp.z, 4))

  const containerBlock = bot.blockAt(bp)
  if (!containerBlock) return "container block disappeared"

  const hint = NON_CONTAINER_HINTS[containerBlock.name]
  if (hint) return `${containerBlock.name} is not a container — ${hint}`

  return { block: containerBlock, containerType: containerBlock.name }
}

function snapshotFurnaceRemaining(furnace: any): ProtoItem[] {
  const remaining: ProtoItem[] = []
  const input = furnace.inputItem()
  const fuel = furnace.fuelItem()
  const output = furnace.outputItem()
  if (input) remaining.push(mapItemToProto(input))
  if (fuel) remaining.push(mapItemToProto(fuel))
  if (output) remaining.push(mapItemToProto(output))
  return remaining
}

/** Navigate to and open a container (chest, barrel, or furnace) and list its contents. */
export async function openContainer(
  bot: Bot,
  req: OpenContainerRequest,
): Promise<OpenContainerResponse> {
  try {
    const resolved = await resolveContainer(bot, req.position, req.blockType)
    if (typeof resolved === "string") {
      return { result: fail(resolved), contents: [], containerType: "" }
    }
    const { block, containerType } = resolved
    const contents: ProtoItem[] = []

    if (FURNACE_TYPES.has(containerType)) {
      const furnace = await (bot as any).openFurnace(block)

      const input = furnace.inputItem()
      const fuel = furnace.fuelItem()
      const output = furnace.outputItem()

      if (input) contents.push(mapItemToProto(input))
      if (fuel) contents.push(mapItemToProto(fuel))
      if (output) contents.push(mapItemToProto(output))

      const slotParts: string[] = []
      if (input) slotParts.push(`input: ${input.name}×${input.count}`)
      if (fuel) slotParts.push(`fuel: ${fuel.name}×${fuel.count}`)
      if (output) slotParts.push(`output: ${output.name}×${output.count}`)

      const progress = furnace.progress
      const progressStr =
        progress != null && progress > 0 ? ` (smelting ${Math.round(progress * 100)}% done)` : ""

      furnace.close()

      const slotSummary = slotParts.length > 0 ? slotParts.join(", ") : "empty"
      return {
        result: ok(`Opened ${containerType}: ${slotSummary}${progressStr}`),
        contents,
        containerType,
      }
    }

    const chest = await bot.openContainer(block)
    for (const item of chest.containerItems()) {
      contents.push(mapItemToProto(item))
    }
    chest.close()

    return {
      result: ok(`Opened ${containerType} with ${contents.length} items`),
      contents,
      containerType,
    }
  } catch (err) {
    const msg = err instanceof Error ? err.message : String(err)
    if (msg.includes("neither a block nor an entity")) {
      return {
        result: fail("block is not a supported container type"),
        contents: [],
        containerType: "",
      }
    }
    return { result: fail(err), contents: [], containerType: "" }
  }
}

// ---------------------------------------------------------------------------
// WithdrawFromContainer
// ---------------------------------------------------------------------------

/** Navigate to a container and withdraw the requested item. */
export async function withdrawFromContainer(
  bot: Bot,
  req: WithdrawFromContainerRequest,
): Promise<WithdrawFromContainerResponse> {
  try {
    const mcData = bot.registry as unknown as IndexedData
    const resolved = await resolveContainer(bot, req.position, req.blockType)
    if (typeof resolved === "string") {
      return { result: fail(resolved), transferredCount: 0, containerRemaining: [] }
    }
    const { block, containerType } = resolved

    const itemInfo = mcData.itemsByName[req.itemName]
    if (!itemInfo) {
      return {
        result: fail(`unknown item "${req.itemName}"`),
        transferredCount: 0,
        containerRemaining: [],
      }
    }

    const requestedCount = req.count || 0

    if (FURNACE_TYPES.has(containerType)) {
      const furnace = await (bot as any).openFurnace(block)
      try {
        const slot = req.slot || "output"
        let slotItem: any
        let takeFn: () => Promise<any>

        if (slot === "output") {
          slotItem = furnace.outputItem()
          takeFn = () => furnace.takeOutput()
        } else if (slot === "input") {
          slotItem = furnace.inputItem()
          takeFn = () => furnace.takeInput()
        } else if (slot === "fuel") {
          slotItem = furnace.fuelItem()
          takeFn = () => furnace.takeFuel()
        } else {
          return {
            result: fail(`invalid slot "${slot}" — use input, fuel, or output`),
            transferredCount: 0,
            containerRemaining: [],
          }
        }

        if (!slotItem || slotItem.count === 0) {
          return {
            result: fail(`furnace ${slot} slot is empty`),
            transferredCount: 0,
            containerRemaining: snapshotFurnaceRemaining(furnace),
          }
        }

        if (slotItem.name !== req.itemName) {
          return {
            result: fail(`furnace ${slot} slot contains ${slotItem.name}, not ${req.itemName}`),
            transferredCount: 0,
            containerRemaining: snapshotFurnaceRemaining(furnace),
          }
        }

        const taken = slotItem.count
        await takeFn()

        const remaining = snapshotFurnaceRemaining(furnace)
        return {
          result: ok(`Withdrew ${taken}× ${req.itemName} from ${containerType} ${slot} slot`),
          transferredCount: taken,
          containerRemaining: remaining,
        }
      } finally {
        furnace.close()
      }
    }

    const chest = await bot.openContainer(block)
    try {
      const containerItems = chest.containerItems()
      const matching = containerItems.filter((i: any) => i.name === req.itemName)
      if (matching.length === 0) {
        return {
          result: fail(`${req.itemName} not found in ${containerType}`),
          transferredCount: 0,
          containerRemaining: containerItems.map(mapItemToProto),
        }
      }

      const available = matching.reduce((sum: number, i: any) => sum + i.count, 0)
      const toTake = requestedCount > 0 ? Math.min(requestedCount, available) : available

      await chest.withdraw(itemInfo.id, null, toTake)

      const remaining = chest.containerItems().map(mapItemToProto)
      return {
        result: ok(`Withdrew ${toTake}× ${req.itemName} from ${containerType}`),
        transferredCount: toTake,
        containerRemaining: remaining,
      }
    } finally {
      chest.close()
    }
  } catch (err) {
    return { result: fail(err), transferredCount: 0, containerRemaining: [] }
  }
}

// ---------------------------------------------------------------------------
// DepositToContainer
// ---------------------------------------------------------------------------

/** Navigate to a container and deposit the requested item. */
export async function depositToContainer(
  bot: Bot,
  req: DepositToContainerRequest,
): Promise<DepositToContainerResponse> {
  try {
    const resolved = await resolveContainer(bot, req.position, req.blockType)
    if (typeof resolved === "string") {
      return { result: fail(resolved), transferredCount: 0, containerRemaining: [] }
    }
    const { block, containerType } = resolved

    const requestedCount = req.count || 0

    if (FURNACE_TYPES.has(containerType)) {
      const furnace = await (bot as any).openFurnace(block)
      try {
        const slot = req.slot || "input"
        if (slot === "output") {
          return {
            result: fail("cannot deposit to furnace output slot"),
            transferredCount: 0,
            containerRemaining: snapshotFurnaceRemaining(furnace),
          }
        }

        const invItem = findItemInWindow(furnace, req.itemName)
        if (!invItem) {
          return {
            result: fail(`"${req.itemName}" not in inventory`),
            transferredCount: 0,
            containerRemaining: snapshotFurnaceRemaining(furnace),
          }
        }

        const toDeposit =
          requestedCount > 0 ? Math.min(requestedCount, invItem.count) : invItem.count

        if (slot === "input") {
          await furnace.putInput(invItem.type, invItem.metadata, toDeposit)
        } else if (slot === "fuel") {
          await furnace.putFuel(invItem.type, invItem.metadata, toDeposit)
        } else {
          return {
            result: fail(`invalid slot "${slot}" — use input or fuel`),
            transferredCount: 0,
            containerRemaining: [],
          }
        }

        const remaining = snapshotFurnaceRemaining(furnace)
        return {
          result: ok(`Deposited ${toDeposit}× ${req.itemName} into ${containerType} ${slot} slot`),
          transferredCount: toDeposit,
          containerRemaining: remaining,
        }
      } finally {
        furnace.close()
      }
    }

    const invItem = findItemInInventory(bot, req.itemName)
    if (!invItem) {
      return {
        result: fail(`"${req.itemName}" not in inventory`),
        transferredCount: 0,
        containerRemaining: [],
      }
    }

    const chest = await bot.openContainer(block)
    try {
      const available = invItem.count
      const toDeposit = requestedCount > 0 ? Math.min(requestedCount, available) : available

      await chest.deposit(invItem.type, null, toDeposit)

      const remaining = chest.containerItems().map(mapItemToProto)
      return {
        result: ok(`Deposited ${toDeposit}× ${req.itemName} into ${containerType}`),
        transferredCount: toDeposit,
        containerRemaining: remaining,
      }
    } finally {
      chest.close()
    }
  } catch (err) {
    return { result: fail(err), transferredCount: 0, containerRemaining: [] }
  }
}

// ---------------------------------------------------------------------------
// CraftItem
// ---------------------------------------------------------------------------

// Register windowOpen BEFORE activating the block so the event is never
// missed due to a race between the server response and listener attachment.
async function openCraftingWindow(bot: Bot, block: MfBlock): Promise<void> {
  return new Promise<void>((resolve, reject) => {
    const timer = setTimeout(() => {
      bot.off("windowOpen", onOpen)
      reject(new Error("crafting table window did not open within 5s"))
    }, 5000)
    const onOpen = () => {
      clearTimeout(timer)
      resolve()
    }
    bot.once("windowOpen", onOpen)
    bot.activateBlock(block).catch(err => {
      clearTimeout(timer)
      bot.off("windowOpen", onOpen)
      reject(err)
    })
  })
}

// Re-fetch the block reference and open the crafting window. Retries once
// with re-navigation if the first attempt fails (stale block or missed event).
async function activateCraftingTable(bot: Bot, tablePos: Vec3): Promise<MfBlock> {
  let block = bot.blockAt(tablePos)
  if (!block || block.name !== "crafting_table") {
    throw new Error("crafting table no longer present")
  }
  for (let attempt = 0; attempt <= 1; attempt++) {
    try {
      await openCraftingWindow(bot, block)
      return block
    } catch {
      if (attempt === 0) {
        await gotoWithTimeout(bot, new goals.GoalNear(tablePos.x, tablePos.y, tablePos.z, 2))
        block = bot.blockAt(tablePos)
        if (!block || block.name !== "crafting_table") {
          throw new Error("crafting table disappeared during retry")
        }
      } else {
        throw new Error("crafting table window failed to open after retry")
      }
    }
  }
  return block
}

/**
 * Craft count of the named item, navigating to a crafting table when required.
 * Crafts one execution at a time to work around the Mineflayer bot.craft(N>1)
 * silent item-loss bug.
 */
export async function craftItem(bot: Bot, req: CraftItemRequest): Promise<CraftItemResponse> {
  const mcData = bot.registry as unknown as IndexedData
  const itemInfo = mcData.itemsByName[req.itemName]
  if (!itemInfo) {
    return { result: fail(`unknown item "${req.itemName}"`), craftedCount: 0, inventoryAfter: [] }
  }

  const count = req.count || 1

  // Check recipes: hand first, then with crafting table.
  const handRecipes = bot.recipesFor(itemInfo.id, null, 1, false)
  const tableRecipes = bot.recipesFor(itemInfo.id, null, 1, true)

  if (handRecipes.length === 0 && tableRecipes.length === 0) {
    // Distinguish "recipe doesn't exist" from "missing ingredients".
    const allHand = bot.recipesAll(itemInfo.id, null, false)
    const allTable = bot.recipesAll(itemInfo.id, null, true)
    if (allHand.length > 0 || allTable.length > 0) {
      return {
        result: fail(
          `recipe exists for "${req.itemName}" but you lack the required ingredients — check inventory and gather materials`,
        ),
        craftedCount: 0,
        inventoryAfter: [],
      }
    }
    return {
      result: fail(`no recipe found for "${req.itemName}"`),
      craftedCount: 0,
      inventoryAfter: [],
    }
  }

  // count means desired output items; translate to recipe executions.
  const useTable = handRecipes.length === 0
  const recipe = useTable ? tableRecipes[0] : handRecipes[0]
  const outputPerCraft = Math.max(1, recipe.result?.count ?? 1)
  const executions = Math.ceil(count / outputPerCraft)

  // If a crafting table is required, navigate to it once before the loop.
  let craftingTable: MfBlock | null = null
  let tablePos: Vec3 | null = null
  if (useTable) {
    const tableInfo = mcData.blocksByName["crafting_table"]
    if (!tableInfo) {
      return {
        result: fail("crafting table block not found in game data"),
        craftedCount: 0,
        inventoryAfter: [],
      }
    }

    const found = bot.findBlock({ matching: tableInfo.id, maxDistance: 32 })
    if (!found) {
      return {
        result: fail("crafting table required but none found within 32 blocks"),
        craftedCount: 0,
        inventoryAfter: [],
      }
    }

    bot.pathfinder.setMovements(safeMovements(bot))
    tablePos = found.position
    await gotoWithTimeout(bot, new goals.GoalNear(tablePos.x, tablePos.y, tablePos.z, 2))

    craftingTable = bot.blockAt(tablePos)
    if (!craftingTable) {
      return {
        result: fail("crafting table disappeared after navigation"),
        craftedCount: 0,
        inventoryAfter: [],
      }
    }
  }

  // Craft one execution at a time. Mineflayer's bot.craft(recipe, N) can
  // silently lose items when N > 1 -- ingredients consumed without output
  // reaching the inventory. Crafting individually and checking gains after
  // each iteration limits damage and provides accurate partial-success info.
  const before = countItem(bot, req.itemName)
  let lastError: string | null = null

  for (let i = 0; i < executions; i++) {
    const recipes = bot.recipesFor(itemInfo.id, null, 1, useTable)
    if (recipes.length === 0) break

    if (useTable) {
      try {
        craftingTable = await activateCraftingTable(bot, tablePos!)
      } catch (err) {
        lastError = err instanceof Error ? err.message : String(err)
        break
      }
    }

    const iterBefore = countItem(bot, req.itemName)
    try {
      await bot.craft(recipes[0], 1, craftingTable ?? undefined)
    } catch (err) {
      lastError = err instanceof Error ? err.message : String(err)
      break
    }
    await waitForInventorySettle(bot, {
      firstEventTimeout: 500,
      quietPeriod: 150,
      maxWait: 3000,
    })

    if (countItem(bot, req.itemName) - iterBefore <= 0) break

    // Yield to the event loop so late-arriving set_slot packets update
    // stateId before the next bot.craft() sends window_click packets.
    if (i < executions - 1) {
      await new Promise<void>(resolve => setTimeout(resolve, 150))
    }
  }

  const inventoryAfter = bot.inventory.items().map(i => mapItemToProto(i))
  const crafted = Math.max(0, countItem(bot, req.itemName) - before)

  if (crafted >= count) {
    return {
      result: ok(`Crafted ${crafted}x ${req.itemName}`),
      craftedCount: crafted,
      inventoryAfter,
    }
  }
  if (lastError && crafted === 0) {
    return { result: fail(lastError), craftedCount: 0, inventoryAfter }
  }
  const suffix = lastError ? `, failed partway: ${lastError}` : " before stopping early"
  return {
    result: fail(`crafted ${crafted}/${count} ${req.itemName}${suffix}`),
    craftedCount: crafted,
    inventoryAfter,
  }
}

// ---------------------------------------------------------------------------
// SmeltItem
// ---------------------------------------------------------------------------

const SMELT_POLL_MS = 200
const SMELT_TIMEOUT_PER_ITEM_MS = 12000

const FUEL_ITEMS = [
  "coal",
  "charcoal",
  "oak_planks",
  "spruce_planks",
  "birch_planks",
  "jungle_planks",
  "acacia_planks",
  "dark_oak_planks",
  "stick",
  "bamboo",
  "dried_kelp_block",
  "blaze_rod",
  "lava_bucket",
]

/** Navigate to a nearby furnace, load fuel and input, and wait for smelting to complete. */
export async function smeltItem(bot: Bot, req: SmeltItemRequest): Promise<SmeltItemResponse> {
  try {
    const mcData = bot.registry as unknown as IndexedData
    const count = req.count || 1

    // Find a furnace-type block.
    const furnaceTypes = ["furnace", "blast_furnace", "smoker"]
    const furnaceIds: number[] = []
    for (const name of furnaceTypes) {
      const info = mcData.blocksByName[name]
      if (info) furnaceIds.push(info.id)
    }

    const found = bot.findBlock({ matching: furnaceIds, maxDistance: 32 })
    if (!found) {
      return { result: fail("no furnace found within 32 blocks"), smeltedCount: 0 }
    }

    // Navigate to furnace.
    bot.pathfinder.setMovements(safeMovements(bot))
    const fp = found.position
    await gotoWithTimeout(bot, new goals.GoalNear(fp.x, fp.y, fp.z, 4))

    const furnaceBlock = bot.blockAt(fp)
    if (!furnaceBlock) {
      return { result: fail("furnace block disappeared"), smeltedCount: 0 }
    }

    const furnace = await (bot as any).openFurnace(furnaceBlock)

    // Find input item in the furnace window's inventory slots -- the same
    // slot range that putInput searches, avoiding bot.inventory mismatch.
    const inputItem = findItemInWindow(furnace, req.itemName)
    if (!inputItem) {
      furnace.close()
      return { result: fail(`item "${req.itemName}" not in inventory`), smeltedCount: 0 }
    }

    let fuelItem
    if (req.fuel) {
      fuelItem = findItemInWindow(furnace, req.fuel)
      if (!fuelItem) {
        furnace.close()
        return { result: fail(`fuel "${req.fuel}" not in inventory`), smeltedCount: 0 }
      }
    } else {
      for (const fuelName of FUEL_ITEMS) {
        fuelItem = findItemInWindow(furnace, fuelName)
        if (fuelItem) break
      }
      if (!fuelItem) {
        furnace.close()
        return { result: fail("no fuel found in inventory"), smeltedCount: 0 }
      }
    }

    const inputCount = Math.min(count, inputItem.count)
    const sameStack = inputItem === fuelItem
    const maxFuel = sameStack ? inputItem.count - inputCount : fuelItem.count
    const fuelNeeded = Math.min(inputCount, maxFuel)

    if (fuelNeeded <= 0) {
      furnace.close()
      return {
        result: fail(`not enough "${fuelItem.name}" to use as both input and fuel`),
        smeltedCount: 0,
      }
    }

    await furnace.putFuel(fuelItem.type, fuelItem.metadata, fuelNeeded)
    await furnace.putInput(inputItem.type, inputItem.metadata, inputCount)

    // Wait for smelting to complete, collecting output incrementally.
    const timeout = inputCount * SMELT_TIMEOUT_PER_ITEM_MS
    const deadline = Date.now() + timeout
    let smeltedCount = 0
    let pollCount = 0
    const EARLY_BAIL_POLLS = 20

    while (Date.now() < deadline) {
      const output = furnace.outputItem()
      if (output && output.count > 0) {
        smeltedCount += output.count
        try {
          await furnace.takeOutput()
        } catch {
          /* keep polling */
        }
        if (smeltedCount >= inputCount) break
      }

      pollCount++
      if (pollCount === EARLY_BAIL_POLLS && smeltedCount === 0 && furnace.progress === null) {
        furnace.close()
        return {
          result: fail(
            `furnace shows no smelting progress — "${req.itemName}" may not be a valid smelting recipe`,
          ),
          smeltedCount: 0,
        }
      }

      await new Promise(r => setTimeout(r, SMELT_POLL_MS))
    }

    // Final sweep for anything that landed after the last poll.
    const trailing = furnace.outputItem()
    if (trailing && trailing.count > 0) {
      smeltedCount += trailing.count
      try {
        await furnace.takeOutput()
      } catch {
        /* ok */
      }
    }

    furnace.close()

    if (smeltedCount >= inputCount) {
      return { result: ok(`Smelted ${smeltedCount}x ${req.itemName}`), smeltedCount }
    } else if (smeltedCount > 0) {
      const remaining = inputCount - smeltedCount
      return {
        result: ok(
          `Smelted ${smeltedCount} of ${inputCount} ${req.itemName} (${remaining} may remain in furnace at ${fp.x},${fp.y},${fp.z})`,
        ),
        smeltedCount,
      }
    } else {
      return { result: fail("smelting timed out — check fuel and recipe"), smeltedCount: 0 }
    }
  } catch (err) {
    return { result: fail(err), smeltedCount: 0 }
  }
}

// ---------------------------------------------------------------------------
// Eat
// ---------------------------------------------------------------------------

const FOOD_SATURATION: Record<string, number> = {
  golden_carrot: 14.4,
  cooked_porkchop: 12.8,
  steak: 12.8,
  cooked_salmon: 9.6,
  cooked_mutton: 9.6,
  cooked_cod: 6,
  baked_potato: 6,
  bread: 6,
  cooked_chicken: 7.2,
  cooked_rabbit: 6,
  golden_apple: 9.6,
  enchanted_golden_apple: 9.6,
  carrot: 3.6,
  apple: 2.4,
  sweet_berries: 1.2,
  melon_slice: 1.2,
  dried_kelp: 0.6,
  cookie: 0.4,
}

/** Consume food from inventory; auto-selects the highest-saturation item when none specified. */
export async function eat(bot: Bot, req: EatRequest): Promise<EatResponse> {
  try {
    let foodItem

    if (req.foodName) {
      foodItem = findItemInInventory(bot, req.foodName)
      if (!foodItem) {
        return {
          result: fail(`food "${req.foodName}" not in inventory`),
          foodUsed: "",
          hungerRestored: 0,
        }
      }
    } else {
      // Auto-select best food by saturation value.
      const foods = bot.inventory.items().filter(i => i.name in FOOD_SATURATION)
      if (foods.length === 0) {
        // Fallback: try any food-like item the game recognizes.
        const mcData = bot.registry as unknown as IndexedData
        const anyFood = bot.inventory.items().find(i => {
          const itemData = mcData.foodsByName?.[i.name]
          return !!itemData
        })
        if (!anyFood) {
          return { result: fail("no food found in inventory"), foodUsed: "", hungerRestored: 0 }
        }
        foodItem = anyFood
      } else {
        foods.sort((a, b) => (FOOD_SATURATION[b.name] ?? 0) - (FOOD_SATURATION[a.name] ?? 0))
        foodItem = foods[0]
      }
    }

    const prevFood = bot.food
    await bot.equip(foodItem, "hand")
    await bot.consume()
    const hungerRestored = bot.food - prevFood

    return {
      result: ok(`Ate ${foodItem.name}`),
      foodUsed: foodItem.name,
      hungerRestored: Math.max(0, hungerRestored),
    }
  } catch (err) {
    return { result: fail(err), foodUsed: "", hungerRestored: 0 }
  }
}
