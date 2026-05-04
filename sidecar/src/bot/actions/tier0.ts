import type { Bot } from "mineflayer"
import { Vec3 } from "vec3"
import pathfinderPkg from "mineflayer-pathfinder"
const { goals, Movements } = pathfinderPkg
import {
  ok,
  fail,
  computeGains,
  findEntityById,
  findItemInInventory,
  gotoWithTimeout,
  snapshotInventory,
  trackInventorySettle,
  waitForInventorySettle,
  checkGoalReached,
  distanceTo,
  FACE_VECTORS,
  EQUIP_DESTINATIONS,
  matchesPlacedBlock,
} from "./helpers.js"
import type {
  MoveToRequest,
  MoveToResponse,
  LookAtRequest,
  LookAtResponse,
  PlaceBlockRequest,
  PlaceBlockResponse,
  DigBlockRequest,
  DigBlockResponse,
  EquipItemRequest,
  EquipItemResponse,
  UseItemRequest,
  UseItemResponse,
  AttackEntityRequest,
  AttackEntityResponse,
  JumpRequest,
  JumpResponse,
  SetSneakRequest,
  SetSneakResponse,
} from "../../grpc/generated/minecraft.js"

// ---------------------------------------------------------------------------
// MoveTo
// ---------------------------------------------------------------------------

/** Move the bot to the target position using pathfinding. */
export async function moveTo(bot: Bot, req: MoveToRequest): Promise<MoveToResponse> {
  try {
    if (!req.target) {
      return {
        result: fail("target position is required"),
        finalPosition: undefined,
        distanceRemaining: 0,
      }
    }

    const { x, y, z } = req.target
    const range = req.range || 1

    const movements = new Movements(bot)
    movements.allowSprinting = req.sprint
    bot.pathfinder.setMovements(movements)

    const goal = new goals.GoalNear(x, y, z, range)
    await gotoWithTimeout(bot, goal)

    const pos = bot.entity.position
    const remaining = distanceTo(pos, x, y, z)

    if (!checkGoalReached(bot, goal)) {
      return {
        result: fail(
          `pathfinder resolved but bot is ${remaining.toFixed(1)} blocks from target (need <= ${range})`,
        ),
        finalPosition: { x: pos.x, y: pos.y, z: pos.z },
        distanceRemaining: Math.round(remaining * 10) / 10,
      }
    }

    return {
      result: ok(`Moved to (${pos.x.toFixed(1)}, ${pos.y.toFixed(1)}, ${pos.z.toFixed(1)})`),
      finalPosition: { x: pos.x, y: pos.y, z: pos.z },
      distanceRemaining: 0,
    }
  } catch (err) {
    const pos = bot.entity.position
    const remaining = req.target ? distanceTo(pos, req.target.x, req.target.y, req.target.z) : 0
    return {
      result: fail(err),
      finalPosition: { x: pos.x, y: pos.y, z: pos.z },
      distanceRemaining: Math.round(remaining * 10) / 10,
    }
  }
}

// ---------------------------------------------------------------------------
// LookAt
// ---------------------------------------------------------------------------

/** Rotate the bot's camera to face the target position immediately. */
export async function lookAt(bot: Bot, req: LookAtRequest): Promise<LookAtResponse> {
  try {
    if (!req.target) {
      return { result: fail("target position is required") }
    }
    const { x, y, z } = req.target
    await bot.lookAt(new Vec3(x, y, z), true)
    return { result: ok(`Looking at (${x.toFixed(1)}, ${y.toFixed(1)}, ${z.toFixed(1)})`) }
  } catch (err) {
    return { result: fail(err) }
  }
}

// ---------------------------------------------------------------------------
// PlaceBlock
// ---------------------------------------------------------------------------

function verifyPlacement(
  bot: Bot,
  dest: Vec3,
  refPos: Vec3,
  itemName: string,
  blockName: string,
  face: string,
): PlaceBlockResponse {
  const destName = bot.blockAt(dest)?.name ?? "air"
  if (matchesPlacedBlock(itemName, destName)) {
    const label = destName !== blockName ? `${blockName} (as ${destName})` : blockName
    return {
      result: ok(`Placed ${label} on ${face || "top"} face at (${dest.x}, ${dest.y}, ${dest.z})`),
    }
  }

  // Fallback: check the reference position (Minecraft may replace a
  // block at the reference rather than placing adjacent to it).
  const refName = bot.blockAt(refPos)?.name ?? "air"
  if (matchesPlacedBlock(itemName, refName)) {
    const label = refName !== blockName ? `${blockName} (as ${refName})` : blockName
    return {
      result: ok(
        `Placed ${label} on ${face || "top"} face at (${refPos.x}, ${refPos.y}, ${refPos.z})`,
      ),
    }
  }

  return {
    result: fail(
      `placed wrong block: expected "${blockName}" but got "${destName}" at (${dest.x}, ${dest.y}, ${dest.z})`,
    ),
  }
}

/** Place the named block against the reference position on the given face. */
export async function placeBlock(bot: Bot, req: PlaceBlockRequest): Promise<PlaceBlockResponse> {
  try {
    if (!req.position) {
      return { result: fail("position is required") }
    }

    const faceVec = FACE_VECTORS[req.face || "top"]
    if (!faceVec) {
      return {
        result: fail(`unknown face "${req.face}", valid: top, bottom, north, south, east, west`),
      }
    }

    const item = findItemInInventory(bot, req.blockName)
    if (!item) {
      return { result: fail(`item "${req.blockName}" not found in inventory`) }
    }

    await bot.equip(item, "hand")
    await new Promise(r => setTimeout(r, 50))

    if (!bot.heldItem || bot.heldItem.name !== item.name) {
      const retryItem = findItemInInventory(bot, req.blockName)
      if (retryItem) {
        await bot.equip(retryItem, "hand")
        await new Promise(r => setTimeout(r, 50))
      }
      if (!bot.heldItem || bot.heldItem.name !== (retryItem ?? item).name) {
        return {
          result: fail(
            `equip failed: holding "${bot.heldItem?.name ?? "nothing"}" instead of "${req.blockName}"`,
          ),
        }
      }
    }

    const { x, y, z } = req.position
    const refBlock = bot.blockAt(new Vec3(x, y, z))
    if (!refBlock) {
      return { result: fail(`no block at (${x}, ${y}, ${z})`) }
    }

    if (bot.heldItem?.name !== item.name) {
      return {
        result: fail(`held item changed to "${bot.heldItem?.name ?? "nothing"}" before placement`),
      }
    }

    const dest = refBlock.position.plus(faceVec)
    const oldBlockType = bot.blockAt(dest)?.type

    // Pre-register to catch blockUpdate events that fire before bot.placeBlock's
    // internal listener is registered (Mineflayer race on fast/local servers).
    let preRegisteredPlacement = false
    const onBlockUpdate = (_: unknown, newBlock: { position: Vec3 }) => {
      if (newBlock.position.equals(dest) || newBlock.position.equals(refBlock.position)) {
        preRegisteredPlacement = true
      }
    }
    bot.on("blockUpdate", onBlockUpdate)

    try {
      await bot.placeBlock(refBlock, faceVec)
    } catch (err) {
      if (!preRegisteredPlacement) {
        // No blockUpdate observed yet -- yield so the world cache can settle.
        await new Promise(r => setTimeout(r, 50))
      }
      const placed =
        preRegisteredPlacement ||
        bot.blockAt(dest)?.type !== oldBlockType ||
        matchesPlacedBlock(item.name, bot.blockAt(refBlock.position)?.name ?? "air")
      if (placed) {
        return verifyPlacement(bot, dest, refBlock.position, item.name, req.blockName, req.face)
      }
      return { result: fail(err) }
    } finally {
      bot.off("blockUpdate", onBlockUpdate)
    }

    return verifyPlacement(bot, dest, refBlock.position, item.name, req.blockName, req.face)
  } catch (err) {
    return { result: fail(err) }
  }
}

// ---------------------------------------------------------------------------
// DigBlock
// ---------------------------------------------------------------------------

/** Mine the block at the given position and return the items gained. */
export async function digBlock(bot: Bot, req: DigBlockRequest): Promise<DigBlockResponse> {
  try {
    if (!req.position) {
      return { result: fail("position is required"), drops: [] }
    }

    const { x, y, z } = req.position
    const block = bot.blockAt(new Vec3(x, y, z))
    if (!block || block.name === "air") {
      return { result: fail(`no diggable block at (${x}, ${y}, ${z})`), drops: [] }
    }

    // Snapshot inventory before digging.
    const before = snapshotInventory(bot)

    const settle = trackInventorySettle(bot)
    await bot.tool.equipForBlock(block)
    await bot.dig(block)
    await settle.wait()

    const drops = computeGains(bot, before)

    return {
      result: ok(`Dug ${block.name} at (${x}, ${y}, ${z})`),
      drops,
    }
  } catch (err) {
    return { result: fail(err), drops: [] }
  }
}

// ---------------------------------------------------------------------------
// EquipItem
// ---------------------------------------------------------------------------

/** Equip the named item into the given destination slot (hand, head, torso, legs, feet). */
export async function equipItem(bot: Bot, req: EquipItemRequest): Promise<EquipItemResponse> {
  try {
    let item = findItemInInventory(bot, req.itemName)
    if (!item) {
      await waitForInventorySettle(bot, {
        firstEventTimeout: 100,
        quietPeriod: 60,
        maxWait: 300,
      })
      item = findItemInInventory(bot, req.itemName)
    }
    if (!item) {
      return { result: fail(`item "${req.itemName}" not found in inventory`) }
    }

    const dest = EQUIP_DESTINATIONS[req.destination || "hand"] ?? "hand"
    await bot.equip(item, dest as any)
    return { result: ok(`Equipped ${req.itemName} in ${dest}`) }
  } catch (err) {
    return { result: fail(err) }
  }
}

// ---------------------------------------------------------------------------
// UseItem
// ---------------------------------------------------------------------------

/** Activate the item currently held in the main hand. */
export async function useItem(bot: Bot, _req: UseItemRequest): Promise<UseItemResponse> {
  try {
    bot.activateItem()
    return { result: ok("Used item in hand") }
  } catch (err) {
    return { result: fail(err) }
  }
}

// ---------------------------------------------------------------------------
// AttackEntity
// ---------------------------------------------------------------------------

/** Perform a single melee attack on the entity with the given ID. */
export async function attackEntity(
  bot: Bot,
  req: AttackEntityRequest,
): Promise<AttackEntityResponse> {
  try {
    const entity = findEntityById(bot, req.entityId)
    if (!entity) {
      return { result: fail(`entity with ID ${req.entityId} not found`) }
    }

    bot.attack(entity)
    return { result: ok(`Attacked ${entity.name ?? entity.type ?? "entity"} (ID ${req.entityId})`) }
  } catch (err) {
    return { result: fail(err) }
  }
}

// ---------------------------------------------------------------------------
// Jump
// ---------------------------------------------------------------------------

/** Trigger a single jump by briefly setting the jump control state. */
export async function jump(bot: Bot, _req: JumpRequest): Promise<JumpResponse> {
  try {
    bot.setControlState("jump", true)
    await new Promise(resolve => setTimeout(resolve, 50))
    bot.setControlState("jump", false)
    return { result: ok("Jumped") }
  } catch (err) {
    return { result: fail(err) }
  }
}

// ---------------------------------------------------------------------------
// SetSneak
// ---------------------------------------------------------------------------

/** Enable or disable the bot's sneak control state. */
export async function setSneak(bot: Bot, req: SetSneakRequest): Promise<SetSneakResponse> {
  try {
    bot.setControlState("sneak", req.enabled)
    return { result: ok(req.enabled ? "Now sneaking" : "Stopped sneaking") }
  } catch (err) {
    return { result: fail(err) }
  }
}
