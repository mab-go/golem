import { describe, it, expect, vi, beforeEach, afterEach } from "vitest"
import { Vec3 } from "vec3"
import {
  ok,
  fail,
  distanceTo,
  matchesPlacedBlock,
  isInteractiveBlock,
  FACE_VECTORS,
  EQUIP_DESTINATIONS,
  isPathRetryable,
  findEntityById,
  findEntityByName,
  findItemInInventory,
  snapshotInventory,
  countItem,
  countGainSince,
  computeGains,
  mapItemToProto,
  validatePosition,
  findSurfaceBlock,
  yieldToEventLoop,
  INTERACTIVE_BLOCKS,
} from "./helpers.js"
import {
  createMockBot,
  addEntityToMock,
  addItemToInventory,
  createMockItem,
  createMockEntity,
  setBlockAt,
  addBlockToRegistry,
} from "../../test-utils/mock-bot.js"

// ---------------------------------------------------------------------------
// ActionResult constructors
// ---------------------------------------------------------------------------

describe("ok", () => {
  it("returns a successful ActionResult", () => {
    const result = ok("done")
    expect(result).toEqual({ success: true, error: "", message: "done" })
  })

  it("handles empty message", () => {
    expect(ok("").message).toBe("")
    expect(ok("").success).toBe(true)
  })
})

describe("fail", () => {
  it("extracts message from Error", () => {
    const result = fail(new Error("broken"))
    expect(result).toEqual({ success: false, error: "broken", message: "" })
  })

  it("converts string to error field", () => {
    const result = fail("something went wrong")
    expect(result).toEqual({ success: false, error: "something went wrong", message: "" })
  })

  it("converts non-string non-Error via String()", () => {
    const result = fail(42)
    expect(result).toEqual({ success: false, error: "42", message: "" })
  })

  it("handles null", () => {
    expect(fail(null).error).toBe("null")
  })
})

// ---------------------------------------------------------------------------
// yieldToEventLoop
// ---------------------------------------------------------------------------

describe("yieldToEventLoop", () => {
  it("returns a promise that resolves", async () => {
    await expect(yieldToEventLoop()).resolves.toBeUndefined()
  })
})

// ---------------------------------------------------------------------------
// distanceTo
// ---------------------------------------------------------------------------

describe("distanceTo", () => {
  it("computes a 3-4-5 triangle", () => {
    expect(distanceTo({ x: 0, y: 0, z: 0 }, 3, 4, 0)).toBe(5)
  })

  it("returns 0 for same point", () => {
    expect(distanceTo({ x: 5, y: 10, z: 3 }, 5, 10, 3)).toBe(0)
  })

  it("computes 3D distance", () => {
    const d = distanceTo({ x: 1, y: 1, z: 1 }, 2, 2, 2)
    expect(d).toBeCloseTo(Math.sqrt(3))
  })

  it("handles negative coordinates", () => {
    const d = distanceTo({ x: -1, y: -1, z: -1 }, 1, 1, 1)
    expect(d).toBeCloseTo(Math.sqrt(12))
  })
})

// ---------------------------------------------------------------------------
// matchesPlacedBlock
// ---------------------------------------------------------------------------

describe("matchesPlacedBlock", () => {
  it("matches identical names", () => {
    expect(matchesPlacedBlock("stone", "stone")).toBe(true)
  })

  it("rejects mismatched names", () => {
    expect(matchesPlacedBlock("stone", "dirt")).toBe(false)
  })

  it("matches torch to wall_torch variant", () => {
    expect(matchesPlacedBlock("torch", "wall_torch")).toBe(true)
    expect(matchesPlacedBlock("torch", "torch")).toBe(true)
  })

  it("matches soul_torch variants", () => {
    expect(matchesPlacedBlock("soul_torch", "soul_wall_torch")).toBe(true)
    expect(matchesPlacedBlock("soul_torch", "soul_torch")).toBe(true)
  })

  it("matches redstone_torch variants", () => {
    expect(matchesPlacedBlock("redstone_torch", "redstone_wall_torch")).toBe(true)
  })

  it("rejects torch variants for wrong item", () => {
    expect(matchesPlacedBlock("stone", "wall_torch")).toBe(false)
  })
})

// ---------------------------------------------------------------------------
// isInteractiveBlock / INTERACTIVE_BLOCKS
// ---------------------------------------------------------------------------

describe("isInteractiveBlock", () => {
  it("returns true for known interactive blocks", () => {
    expect(isInteractiveBlock("chest")).toBe(true)
    expect(isInteractiveBlock("crafting_table")).toBe(true)
    expect(isInteractiveBlock("furnace")).toBe(true)
    expect(isInteractiveBlock("anvil")).toBe(true)
    expect(isInteractiveBlock("barrel")).toBe(true)
  })

  it("returns false for non-interactive blocks", () => {
    expect(isInteractiveBlock("dirt")).toBe(false)
    expect(isInteractiveBlock("stone")).toBe(false)
    expect(isInteractiveBlock("air")).toBe(false)
  })

  it("includes all shulker box colors", () => {
    expect(isInteractiveBlock("white_shulker_box")).toBe(true)
    expect(isInteractiveBlock("black_shulker_box")).toBe(true)
    expect(isInteractiveBlock("red_shulker_box")).toBe(true)
  })
})

// ---------------------------------------------------------------------------
// FACE_VECTORS
// ---------------------------------------------------------------------------

describe("FACE_VECTORS", () => {
  it("has all 6 cardinal directions", () => {
    expect(Object.keys(FACE_VECTORS)).toHaveLength(6)
  })

  it("maps top to +Y", () => {
    expect(FACE_VECTORS["top"]).toEqual(new Vec3(0, 1, 0))
  })

  it("maps bottom to -Y", () => {
    expect(FACE_VECTORS["bottom"]).toEqual(new Vec3(0, -1, 0))
  })

  it("maps north to -Z", () => {
    expect(FACE_VECTORS["north"]).toEqual(new Vec3(0, 0, -1))
  })

  it("maps south to +Z", () => {
    expect(FACE_VECTORS["south"]).toEqual(new Vec3(0, 0, 1))
  })

  it("maps east to +X", () => {
    expect(FACE_VECTORS["east"]).toEqual(new Vec3(1, 0, 0))
  })

  it("maps west to -X", () => {
    expect(FACE_VECTORS["west"]).toEqual(new Vec3(-1, 0, 0))
  })
})

// ---------------------------------------------------------------------------
// EQUIP_DESTINATIONS
// ---------------------------------------------------------------------------

describe("EQUIP_DESTINATIONS", () => {
  it("contains hand and off-hand", () => {
    expect(EQUIP_DESTINATIONS["hand"]).toBe("hand")
    expect(EQUIP_DESTINATIONS["off-hand"]).toBe("off-hand")
  })

  it("contains all armor slots", () => {
    expect(EQUIP_DESTINATIONS["head"]).toBe("head")
    expect(EQUIP_DESTINATIONS["torso"]).toBe("torso")
    expect(EQUIP_DESTINATIONS["legs"]).toBe("legs")
    expect(EQUIP_DESTINATIONS["feet"]).toBe("feet")
  })

  it("has exactly 6 entries", () => {
    expect(Object.keys(EQUIP_DESTINATIONS)).toHaveLength(6)
  })
})

// ---------------------------------------------------------------------------
// isPathRetryable
// ---------------------------------------------------------------------------

describe("isPathRetryable", () => {
  it("returns true for PathStopped", () => {
    const err = new Error("path stopped")
    err.name = "PathStopped"
    expect(isPathRetryable(err)).toBe(true)
  })

  it("returns true for GoalChanged", () => {
    const err = new Error("goal changed")
    err.name = "GoalChanged"
    expect(isPathRetryable(err)).toBe(true)
  })

  it("returns false for other errors", () => {
    expect(isPathRetryable(new Error("timeout"))).toBe(false)
  })

  it("returns false for non-Error values", () => {
    expect(isPathRetryable("PathStopped")).toBe(false)
    expect(isPathRetryable(null)).toBe(false)
    expect(isPathRetryable(undefined)).toBe(false)
  })
})

// ---------------------------------------------------------------------------
// Entity resolution
// ---------------------------------------------------------------------------

describe("findEntityById", () => {
  it("returns the entity with matching id", () => {
    const bot = createMockBot()
    const zombie = createMockEntity(42, "zombie", new Vec3(10, 64, 10))
    addEntityToMock(bot, zombie)

    expect(findEntityById(bot, 42)).toBe(zombie)
  })

  it("returns undefined for unknown id", () => {
    const bot = createMockBot()
    expect(findEntityById(bot, 999)).toBeUndefined()
  })
})

describe("findEntityByName", () => {
  it("finds entity by name", () => {
    const bot = createMockBot()
    const zombie = createMockEntity(1, "zombie", new Vec3(10, 64, 10))
    addEntityToMock(bot, zombie)

    expect(findEntityByName(bot, "zombie")).toBe(zombie)
  })

  it("is case-insensitive", () => {
    const bot = createMockBot()
    const zombie = createMockEntity(1, "zombie", new Vec3(10, 64, 10))
    addEntityToMock(bot, zombie)

    expect(findEntityByName(bot, "Zombie")).toBe(zombie)
    expect(findEntityByName(bot, "ZOMBIE")).toBe(zombie)
  })

  it("returns the closest matching entity", () => {
    const bot = createMockBot({ position: new Vec3(0, 64, 0) })
    const far = createMockEntity(1, "zombie", new Vec3(20, 64, 0))
    const near = createMockEntity(2, "zombie", new Vec3(5, 64, 0))
    addEntityToMock(bot, far)
    addEntityToMock(bot, near)

    expect(findEntityByName(bot, "zombie")).toBe(near)
  })

  it("skips the bot's own entity", () => {
    const bot = createMockBot()
    expect(findEntityByName(bot, "player")).toBeUndefined()
  })

  it("returns undefined when no match", () => {
    const bot = createMockBot()
    expect(findEntityByName(bot, "nonexistent")).toBeUndefined()
  })

  it("matches by username", () => {
    const bot = createMockBot()
    const player = createMockEntity(1, "player", new Vec3(5, 64, 0), { username: "Steve" })
    addEntityToMock(bot, player)

    expect(findEntityByName(bot, "Steve")).toBe(player)
  })

  it("matches by type", () => {
    const bot = createMockBot()
    const mob = createMockEntity(1, "", new Vec3(5, 64, 0), { type: "mob" })
    addEntityToMock(bot, mob)

    expect(findEntityByName(bot, "mob")).toBe(mob)
  })
})

// ---------------------------------------------------------------------------
// Inventory helpers
// ---------------------------------------------------------------------------

describe("findItemInInventory", () => {
  it("finds item by name", () => {
    const bot = createMockBot()
    const pick = createMockItem("diamond_pickaxe", 1)
    addItemToInventory(bot, pick)

    expect(findItemInInventory(bot, "diamond_pickaxe")).toBe(pick)
  })

  it("is case-insensitive", () => {
    const bot = createMockBot()
    addItemToInventory(bot, createMockItem("stone", 32))

    expect(findItemInInventory(bot, "Stone")).toBeDefined()
  })

  it("returns undefined when not found", () => {
    const bot = createMockBot()
    expect(findItemInInventory(bot, "diamond")).toBeUndefined()
  })
})

describe("snapshotInventory", () => {
  it("returns empty map for empty inventory", () => {
    const bot = createMockBot()
    const snap = snapshotInventory(bot)
    expect(snap.size).toBe(0)
  })

  it("aggregates stacks by name", () => {
    const bot = createMockBot()
    addItemToInventory(bot, createMockItem("cobblestone", 64, { slot: 0 }))
    addItemToInventory(bot, createMockItem("cobblestone", 32, { slot: 1 }))
    addItemToInventory(bot, createMockItem("dirt", 10, { slot: 2 }))

    const snap = snapshotInventory(bot)
    expect(snap.get("cobblestone")).toBe(96)
    expect(snap.get("dirt")).toBe(10)
  })
})

describe("countItem", () => {
  it("returns 0 for absent item", () => {
    const bot = createMockBot()
    expect(countItem(bot, "diamond")).toBe(0)
  })

  it("sums across stacks", () => {
    const bot = createMockBot()
    addItemToInventory(bot, createMockItem("oak_log", 64, { slot: 0 }))
    addItemToInventory(bot, createMockItem("oak_log", 30, { slot: 1 }))

    expect(countItem(bot, "oak_log")).toBe(94)
  })
})

describe("countGainSince", () => {
  it("returns 0 when nothing changed", () => {
    const bot = createMockBot()
    addItemToInventory(bot, createMockItem("stone", 10))
    const before = snapshotInventory(bot)

    expect(countGainSince(bot, before)).toBe(0)
  })

  it("counts only positive diffs", () => {
    const bot = createMockBot()
    addItemToInventory(bot, createMockItem("stone", 10))
    const before = new Map([
      ["stone", 5],
      ["dirt", 20],
    ])

    expect(countGainSince(bot, before)).toBe(5)
  })

  it("counts new items not in snapshot", () => {
    const bot = createMockBot()
    addItemToInventory(bot, createMockItem("diamond", 3))
    const before = new Map<string, number>()

    expect(countGainSince(bot, before)).toBe(3)
  })
})

describe("computeGains", () => {
  it("returns empty array when nothing gained", () => {
    const bot = createMockBot()
    addItemToInventory(bot, createMockItem("stone", 10))
    const before = new Map([["stone", 10]])

    expect(computeGains(bot, before)).toEqual([])
  })

  it("returns gained items with correct count", () => {
    const bot = createMockBot()
    addItemToInventory(bot, createMockItem("iron_ore", 8))
    const before = new Map([["iron_ore", 3]])

    const gains = computeGains(bot, before)
    expect(gains).toHaveLength(1)
    expect(gains[0].name).toBe("iron_ore")
    expect(gains[0].count).toBe(5)
  })
})

// ---------------------------------------------------------------------------
// mapItemToProto
// ---------------------------------------------------------------------------

describe("mapItemToProto", () => {
  it("maps a basic item", () => {
    const item = createMockItem("stone", 64)
    const proto = mapItemToProto(item as any)

    expect(proto.name).toBe("stone")
    expect(proto.count).toBe(64)
    expect(proto.displayName).toBe("stone")
    expect(proto.maxDurability).toBe(0)
    expect(proto.currentDurability).toBe(0)
  })

  it("computes durability correctly", () => {
    const item = createMockItem("diamond_pickaxe", 1, {
      maxDurability: 1561,
      durabilityUsed: 100,
    })
    const proto = mapItemToProto(item as any)

    expect(proto.maxDurability).toBe(1561)
    expect(proto.currentDurability).toBe(1461)
  })

  it("handles zero durability used", () => {
    const item = createMockItem("iron_sword", 1, {
      maxDurability: 250,
      durabilityUsed: 0,
    })
    const proto = mapItemToProto(item as any)

    expect(proto.currentDurability).toBe(250)
  })
})

// ---------------------------------------------------------------------------
// validatePosition
// ---------------------------------------------------------------------------

describe("validatePosition", () => {
  it("returns position when valid", () => {
    const bot = createMockBot({ position: new Vec3(10, 64, -5) })
    const pos = validatePosition(bot)
    expect(pos.x).toBe(10)
    expect(pos.y).toBe(64)
    expect(pos.z).toBe(-5)
  })

  it("throws when x is NaN", () => {
    const bot = createMockBot({ position: new Vec3(NaN, 64, 0) })
    expect(() => validatePosition(bot)).toThrow("bot position is NaN")
  })

  it("throws when y is NaN", () => {
    const bot = createMockBot({ position: new Vec3(0, NaN, 0) })
    expect(() => validatePosition(bot)).toThrow("bot position is NaN")
  })

  it("throws when z is NaN", () => {
    const bot = createMockBot({ position: new Vec3(0, 64, NaN) })
    expect(() => validatePosition(bot)).toThrow("bot position is NaN")
  })
})

// ---------------------------------------------------------------------------
// findSurfaceBlock
// ---------------------------------------------------------------------------

describe("findSurfaceBlock", () => {
  it("returns null when no blocks found", () => {
    const bot = createMockBot()
    vi.mocked(bot.findBlocks).mockReturnValue([])

    expect(findSurfaceBlock(bot, 1, 32)).toBeNull()
  })

  it("prefers surface-exposed blocks", () => {
    const bot = createMockBot()
    const buried = new Vec3(5, 60, 5)
    const surface = new Vec3(10, 63, 10)
    vi.mocked(bot.findBlocks).mockReturnValue([buried, surface])

    const buriedBlock = { name: "iron_ore" }
    const surfaceBlock = { name: "iron_ore" }
    const air = { name: "air" }
    const stone = { name: "stone" }

    vi.mocked(bot.blockAt).mockImplementation((pos: any) => {
      const key = `${pos.x},${pos.y},${pos.z}`
      const blocks: Record<string, any> = {
        "5,60,5": buriedBlock,
        "10,63,10": surfaceBlock,
        // Neighbors of buried -- all solid
        "5,61,5": stone,
        "5,59,5": stone,
        "6,60,5": stone,
        "4,60,5": stone,
        "5,60,6": stone,
        "5,60,4": stone,
        // Neighbors of surface -- one is air
        "10,64,10": air,
      }
      return blocks[key] ?? stone
    })

    expect(findSurfaceBlock(bot, 1, 32)).toBe(surfaceBlock)
  })

  it("falls back to closest block when none exposed", () => {
    const bot = createMockBot()
    const pos = new Vec3(5, 60, 5)
    vi.mocked(bot.findBlocks).mockReturnValue([pos])

    const block = { name: "diamond_ore" }
    const stone = { name: "stone" }
    vi.mocked(bot.blockAt).mockImplementation((p: any) => {
      if (p.x === 5 && p.y === 60 && p.z === 5) return block as any
      return stone as any
    })

    expect(findSurfaceBlock(bot, 1, 32)).toBe(block)
  })

  it("filters by maxY when provided", () => {
    const bot = createMockBot()
    const high = new Vec3(5, 100, 5)
    const low = new Vec3(5, 50, 5)
    vi.mocked(bot.findBlocks).mockReturnValue([high, low])

    const block = { name: "coal_ore" }
    vi.mocked(bot.blockAt).mockReturnValue(block as any)

    const result = findSurfaceBlock(bot, 1, 32, 60)
    expect(result).toBe(block)
    // blockAt should only be called for positions with y <= 60
  })
})
