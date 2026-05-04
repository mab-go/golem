import { describe, it, expect, vi } from "vitest"
import { Vec3 } from "vec3"
import { effectiveLightLevel, ARMOR_POINTS, readVitalSigns } from "./perception.js"
import { createMockBot, createMockItem } from "../test-utils/mock-bot.js"

// ---------------------------------------------------------------------------
// effectiveLightLevel
// ---------------------------------------------------------------------------

describe("effectiveLightLevel", () => {
  it("returns sky light at daytime (no darkening)", () => {
    expect(effectiveLightLevel(0, 15, 6000)).toBe(15)
  })

  it("returns block light when it exceeds effective sky light", () => {
    expect(effectiveLightLevel(14, 15, 14000)).toBe(14)
  })

  it("returns 0 when both lights are 0", () => {
    expect(effectiveLightLevel(0, 0, 6000)).toBe(0)
  })

  it("applies full darkening at night", () => {
    // At 18000 ticks (midnight area), skyDarkening = 11
    // effectiveSky = max(0, 15 - 11) = 4
    expect(effectiveLightLevel(0, 15, 18000)).toBe(4)
  })

  it("applies partial darkening at dusk", () => {
    // At 12600 ticks, skyDarkening = 2
    // effectiveSky = max(0, 15 - 2) = 13
    expect(effectiveLightLevel(0, 15, 12600)).toBe(13)
  })

  it("returns block light when higher than darkened sky at night", () => {
    // Night: skyDarkening = 11, effectiveSky = 4, block = 12
    expect(effectiveLightLevel(12, 15, 18000)).toBe(12)
  })

  it("handles dawn (23600+) with no darkening", () => {
    expect(effectiveLightLevel(0, 15, 23800)).toBe(15)
  })

  it("handles pre-dawn (23000-23600) with partial darkening", () => {
    // skyDarkening = 2
    expect(effectiveLightLevel(0, 15, 23200)).toBe(13)
  })

  it("handles early dusk (13000-13800) with moderate darkening", () => {
    // skyDarkening = 6
    expect(effectiveLightLevel(0, 15, 13400)).toBe(9)
  })
})

// ---------------------------------------------------------------------------
// ARMOR_POINTS
// ---------------------------------------------------------------------------

describe("ARMOR_POINTS", () => {
  it("has correct iron chestplate value", () => {
    expect(ARMOR_POINTS["iron_chestplate"]).toBe(6)
  })

  it("has correct diamond chestplate value", () => {
    expect(ARMOR_POINTS["diamond_chestplate"]).toBe(8)
  })

  it("has correct netherite chestplate value", () => {
    expect(ARMOR_POINTS["netherite_chestplate"]).toBe(8)
  })

  it("has correct leather set total (7)", () => {
    const total =
      ARMOR_POINTS["leather_helmet"] +
      ARMOR_POINTS["leather_chestplate"] +
      ARMOR_POINTS["leather_leggings"] +
      ARMOR_POINTS["leather_boots"]
    expect(total).toBe(7)
  })

  it("has correct diamond set total (20)", () => {
    const total =
      ARMOR_POINTS["diamond_helmet"] +
      ARMOR_POINTS["diamond_chestplate"] +
      ARMOR_POINTS["diamond_leggings"] +
      ARMOR_POINTS["diamond_boots"]
    expect(total).toBe(20)
  })

  it("includes turtle_helmet", () => {
    expect(ARMOR_POINTS["turtle_helmet"]).toBe(2)
  })
})

// ---------------------------------------------------------------------------
// readVitalSigns
// ---------------------------------------------------------------------------

describe("readVitalSigns", () => {
  it("reads health and food from bot", () => {
    const bot = createMockBot({ health: 18, food: 15, foodSaturation: 3.5 })
    const vitals = readVitalSigns(bot)

    expect(vitals.health).toBe(18)
    expect(vitals.maxHealth).toBe(20)
    expect(vitals.food).toBe(15)
    expect(vitals.foodSaturation).toBe(3.5)
  })

  it("reads experience and game mode", () => {
    const bot = createMockBot({ xpLevel: 7, xpProgress: 0.5, gameMode: "creative" })
    const vitals = readVitalSigns(bot)

    expect(vitals.xpLevel).toBe(7)
    expect(vitals.xpProgress).toBe(0.5)
    expect(vitals.gameMode).toBe("creative")
  })

  it("returns armor points from equipped armor", () => {
    const bot = createMockBot()
    // Slots 5-8 are armor slots: helmet, chestplate, leggings, boots
    ;(bot.inventory.slots as any[])[5] = createMockItem("iron_helmet", 1)
    ;(bot.inventory.slots as any[])[6] = createMockItem("iron_chestplate", 1)

    const vitals = readVitalSigns(bot)
    expect(vitals.armor).toBe(8) // 2 + 6
  })

  it("returns 0 armor with no equipment", () => {
    const bot = createMockBot()
    const vitals = readVitalSigns(bot)
    expect(vitals.armor).toBe(0)
  })

  it("returns default oxygen level", () => {
    const bot = createMockBot()
    const vitals = readVitalSigns(bot)
    expect(vitals.oxygen).toBe(300)
  })

  it("maps held item", () => {
    const held = createMockItem("diamond_sword", 1, { maxDurability: 1561, durabilityUsed: 50 })
    const bot = createMockBot({ heldItem: held })
    const vitals = readVitalSigns(bot)

    expect(vitals.mainHand).toBeDefined()
    expect(vitals.mainHand!.name).toBe("diamond_sword")
    expect(vitals.mainHand!.currentDurability).toBe(1511)
  })

  it("returns undefined mainHand when not holding anything", () => {
    const bot = createMockBot({ heldItem: null })
    const vitals = readVitalSigns(bot)
    expect(vitals.mainHand).toBeUndefined()
  })

  it("reads off-hand from slot 45", () => {
    const bot = createMockBot()
    ;(bot.inventory.slots as any[])[45] = createMockItem("shield", 1)
    const vitals = readVitalSigns(bot)

    expect(vitals.offHand).toBeDefined()
    expect(vitals.offHand!.name).toBe("shield")
  })

  it("returns empty effects when none active", () => {
    const bot = createMockBot()
    const vitals = readVitalSigns(bot)
    expect(vitals.effects).toEqual([])
  })
})
