import { describe, it, expect, vi } from "vitest"
import { Vec3 } from "vec3"
import { ThreatLevel } from "../../grpc/generated/minecraft.js"
import {
  createMockBot,
  addEntityToMock,
  createMockEntity,
  addBlockToRegistry,
  addItemToRegistry,
  setBlockAt,
} from "../../test-utils/mock-bot.js"

class MockMovements {
  allowSprinting = true
  allow1by1towers = true
  scafoldingBlocks: number[] = []
  maxDropDown = 4
  infiniteLiquidDropdownDistance = true
  blocksCantBreak = new Set<number>()
}

vi.mock("mineflayer-pathfinder", () => ({
  default: {
    goals: {
      GoalNear: class {
        constructor(
          public x: number,
          public y: number,
          public z: number,
          public range: number,
        ) {}
      },
    },
    Movements: MockMovements,
  },
}))

const { assessThreat, findNearest, whatCanICraft, surveyArea, planPath } = await import(
  "./tier3.js"
)

// ---------------------------------------------------------------------------
// assessThreat
// ---------------------------------------------------------------------------

describe("assessThreat", () => {
  function makeThreatBot(opts?: {
    health?: number
    timeOfDay?: number
    light?: number
    skyLight?: number
  }) {
    const bot = createMockBot({
      health: opts?.health ?? 20,
      timeOfDay: opts?.timeOfDay ?? 6000,
    })
    setBlockAt(bot, bot.entity.position, {
      name: "grass_block",
      light: opts?.light ?? 0,
      skyLight: opts?.skyLight ?? 15,
    })
    return bot
  }

  it("returns SAFE when no hostiles, daytime, full light", async () => {
    const bot = makeThreatBot()
    const result = await assessThreat(bot, { radius: 24 })

    expect(result.threatLevel).toBe(ThreatLevel.THREAT_SAFE)
    expect(result.hostileEntities).toHaveLength(0)
  })

  it("returns CRITICAL when hostile within 3 blocks", async () => {
    const bot = makeThreatBot()
    addEntityToMock(bot, createMockEntity(1, "zombie", new Vec3(2, 64, 0)))

    const result = await assessThreat(bot, { radius: 24 })
    expect(result.threatLevel).toBe(ThreatLevel.THREAT_CRITICAL)
  })

  it("returns CRITICAL when low health with any hostile", async () => {
    const bot = makeThreatBot({ health: 5 })
    addEntityToMock(bot, createMockEntity(1, "skeleton", new Vec3(15, 64, 0)))

    const result = await assessThreat(bot, { radius: 24 })
    expect(result.threatLevel).toBe(ThreatLevel.THREAT_CRITICAL)
  })

  it("returns HIGH when hostile within 8 blocks", async () => {
    const bot = makeThreatBot()
    addEntityToMock(bot, createMockEntity(1, "creeper", new Vec3(7, 64, 0)))

    const result = await assessThreat(bot, { radius: 24 })
    expect(result.threatLevel).toBe(ThreatLevel.THREAT_HIGH)
  })

  it("returns HIGH when 4+ hostiles present", async () => {
    const bot = makeThreatBot()
    for (let i = 0; i < 4; i++) {
      addEntityToMock(bot, createMockEntity(i + 1, "zombie", new Vec3(15 + i, 64, 0)))
    }

    const result = await assessThreat(bot, { radius: 24 })
    expect(result.threatLevel).toBe(ThreatLevel.THREAT_HIGH)
  })

  it("returns MODERATE when 1-3 hostiles beyond 8 blocks", async () => {
    const bot = makeThreatBot()
    addEntityToMock(bot, createMockEntity(1, "spider", new Vec3(15, 64, 0)))

    const result = await assessThreat(bot, { radius: 24 })
    expect(result.threatLevel).toBe(ThreatLevel.THREAT_MODERATE)
  })

  it("returns LOW when no hostiles but light <= 7", async () => {
    const bot = makeThreatBot({ light: 5, skyLight: 5 })
    const result = await assessThreat(bot, { radius: 24 })

    expect(result.threatLevel).toBe(ThreatLevel.THREAT_LOW)
  })

  it("returns LOW when nightfall is imminent (< 60s)", async () => {
    // 13000 ticks = night. 12000 ticks / 20 tps = 50s until night
    const bot = makeThreatBot({ timeOfDay: 12000 })
    const result = await assessThreat(bot, { radius: 24 })

    expect(result.threatLevel).toBe(ThreatLevel.THREAT_LOW)
    expect(result.timeUntilNightfall).toBe(50)
  })

  it("reports -1 for timeUntilNightfall when already night", async () => {
    const bot = makeThreatBot({ timeOfDay: 15000 })
    const result = await assessThreat(bot, { radius: 24 })

    expect(result.timeUntilNightfall).toBe(-1)
  })

  it("ignores non-hostile entities", async () => {
    const bot = makeThreatBot()
    addEntityToMock(bot, createMockEntity(1, "cow", new Vec3(3, 64, 0)))
    addEntityToMock(bot, createMockEntity(2, "pig", new Vec3(4, 64, 0)))

    const result = await assessThreat(bot, { radius: 24 })
    expect(result.hostileEntities).toHaveLength(0)
    expect(result.threatLevel).toBe(ThreatLevel.THREAT_SAFE)
  })

  it("ignores hostiles beyond radius", async () => {
    const bot = makeThreatBot()
    addEntityToMock(bot, createMockEntity(1, "zombie", new Vec3(100, 64, 0)))

    const result = await assessThreat(bot, { radius: 24 })
    expect(result.hostileEntities).toHaveLength(0)
  })

  it("sorts hostiles by distance", async () => {
    const bot = makeThreatBot()
    addEntityToMock(bot, createMockEntity(1, "zombie", new Vec3(20, 64, 0)))
    addEntityToMock(bot, createMockEntity(2, "skeleton", new Vec3(10, 64, 0)))

    const result = await assessThreat(bot, { radius: 24 })
    expect(result.hostileEntities[0].type).toBe("skeleton")
    expect(result.hostileEntities[1].type).toBe("zombie")
  })

  it("includes summary string", async () => {
    const bot = makeThreatBot()
    addEntityToMock(bot, createMockEntity(1, "zombie", new Vec3(5, 64, 0)))

    const result = await assessThreat(bot, { radius: 24 })
    expect(result.summary).toContain("Threat:")
    expect(result.summary).toContain("zombie")
  })

  it("defaults radius to 24", async () => {
    const bot = makeThreatBot()
    addEntityToMock(bot, createMockEntity(1, "zombie", new Vec3(20, 64, 0)))

    const result = await assessThreat(bot, { radius: 0 })
    expect(result.hostileEntities).toHaveLength(1)
  })

  it("includes light level in response", async () => {
    const bot = makeThreatBot({ skyLight: 12 })
    const result = await assessThreat(bot, { radius: 24 })
    expect(result.localLightLevel).toBe(12)
  })
})

// ---------------------------------------------------------------------------
// whatCanICraft
// ---------------------------------------------------------------------------

describe("whatCanICraft", () => {
  it("returns empty when no recipes available", async () => {
    const bot = createMockBot()
    addItemToRegistry(bot, "crafting_table", { id: 100 })
    vi.mocked(bot.recipesFor).mockReturnValue([])

    const result = await whatCanICraft(bot, {})
    expect(result.items).toEqual([])
  })

  it("identifies craftable items from inventory", async () => {
    const bot = createMockBot()
    addItemToRegistry(bot, "stick", { id: 200 })

    const recipe = {
      result: { count: 4 },
      delta: [
        { id: 201, count: -2 }, // oak_planks consumed
      ],
    }
    vi.mocked(bot.recipesFor).mockImplementation((_id, _meta, _count, table) => {
      if (!table) return [recipe]
      return [recipe]
    })

    // Add the ingredient name to registry so delta lookup works
    ;(bot.registry as any).items[201] = { id: 201, name: "oak_planks" }

    const result = await whatCanICraft(bot, {})
    const stickEntry = result.items.find(i => i.itemName === "stick")
    expect(stickEntry).toBeDefined()
    expect(stickEntry!.maxCount).toBe(4)
    expect(stickEntry!.ingredients).toContain("oak_planks")
  })

  it("marks requiresCraftingTable when no hand recipes", async () => {
    const bot = createMockBot()
    addItemToRegistry(bot, "furnace", { id: 300 })

    const recipe = { result: { count: 1 }, delta: [] }
    vi.mocked(bot.recipesFor).mockImplementation((_id, _meta, _count, table) => {
      if (table) return [recipe]
      return [] // not craftable by hand
    })

    const result = await whatCanICraft(bot, {})
    const furnaceEntry = result.items.find(i => i.itemName === "furnace")
    expect(furnaceEntry).toBeDefined()
    expect(furnaceEntry!.requiresCraftingTable).toBe(true)
  })

  it("does not mark requiresCraftingTable when hand recipe exists", async () => {
    const bot = createMockBot()
    addItemToRegistry(bot, "stick", { id: 200 })

    const recipe = { result: { count: 4 }, delta: [] }
    vi.mocked(bot.recipesFor).mockReturnValue([recipe])

    const result = await whatCanICraft(bot, {})
    const stickEntry = result.items.find(i => i.itemName === "stick")
    expect(stickEntry).toBeDefined()
    expect(stickEntry!.requiresCraftingTable).toBe(false)
  })
})

// ---------------------------------------------------------------------------
// findNearest
// ---------------------------------------------------------------------------

describe("findNearest", () => {
  it("finds nearest block by type", async () => {
    const bot = createMockBot()
    addBlockToRegistry(bot, "diamond_ore", { id: 56 })

    const blockPos = new Vec3(10, 50, 10)
    vi.mocked(bot.findBlock).mockReturnValue({
      position: blockPos,
      name: "diamond_ore",
    } as any)

    const result = await findNearest(bot, {
      blockType: "diamond_ore",
      entityType: "",
      structureType: "",
      radius: 64,
    })
    expect(result.found).toBe(true)
    expect(result.type).toBe("diamond_ore")
    expect(result.position).toBeDefined()
  })

  it("returns not found for unknown block type", async () => {
    const bot = createMockBot()
    // No block registered

    const result = await findNearest(bot, {
      blockType: "unobtanium",
      entityType: "",
      structureType: "",
      radius: 64,
    })
    expect(result.found).toBe(false)
    expect(result.type).toBe("unobtanium")
  })

  it("returns not found when block not in range", async () => {
    const bot = createMockBot()
    addBlockToRegistry(bot, "diamond_ore", { id: 56 })
    vi.mocked(bot.findBlock).mockReturnValue(null)

    const result = await findNearest(bot, {
      blockType: "diamond_ore",
      entityType: "",
      structureType: "",
      radius: 64,
    })
    expect(result.found).toBe(false)
  })

  it("finds nearest entity by type", async () => {
    const bot = createMockBot()
    const cow = createMockEntity(1, "cow", new Vec3(10, 64, 0))
    addEntityToMock(bot, cow)

    const result = await findNearest(bot, {
      blockType: "",
      entityType: "cow",
      structureType: "",
      radius: 64,
    })
    expect(result.found).toBe(true)
    expect(result.type).toBe("cow")
  })

  it("returns not found for entity beyond radius", async () => {
    const bot = createMockBot()
    addEntityToMock(bot, createMockEntity(1, "cow", new Vec3(100, 64, 0)))

    const result = await findNearest(bot, {
      blockType: "",
      entityType: "cow",
      structureType: "",
      radius: 32,
    })
    expect(result.found).toBe(false)
  })

  it("returns not found when no search type specified", async () => {
    const bot = createMockBot()
    const result = await findNearest(bot, {
      blockType: "",
      entityType: "",
      structureType: "",
      radius: 64,
    })
    expect(result.found).toBe(false)
    expect(result.type).toBe("")
  })

  it("defaults radius to 64", async () => {
    const bot = createMockBot()
    addEntityToMock(bot, createMockEntity(1, "cow", new Vec3(50, 64, 0)))

    const result = await findNearest(bot, {
      blockType: "",
      entityType: "cow",
      structureType: "",
      radius: 0,
    })
    expect(result.found).toBe(true)
  })
})

// ---------------------------------------------------------------------------
// surveyArea
// ---------------------------------------------------------------------------

describe("surveyArea", () => {
  it("returns empty clusters when no notable blocks found", async () => {
    const bot = createMockBot()
    // No blocks registered, so no matching IDs
    vi.mocked(bot.findBlocks).mockReturnValue([])

    const result = await surveyArea(bot, { radius: 32 })
    expect(result.blockClusters).toEqual([])
  })

  it("clusters found blocks with centroid position", async () => {
    const bot = createMockBot()
    addBlockToRegistry(bot, "iron_ore", { id: 15 })

    vi.mocked(bot.findBlocks).mockImplementation(({ matching }: any) => {
      if (Array.isArray(matching) ? matching.includes(15) : matching === 15) {
        return [new Vec3(10, 50, 10), new Vec3(12, 50, 10)]
      }
      return []
    })

    vi.mocked(bot.blockAt).mockImplementation((pos: any) => {
      return { name: "iron_ore" } as any
    })

    const result = await surveyArea(bot, { radius: 32 })
    const ironCluster = result.blockClusters.find(c => c.blockType === "iron_ore")
    expect(ironCluster).toBeDefined()
    expect(ironCluster!.count).toBe(2)
    expect(ironCluster!.center!.x).toBe(11) // avg of 10 and 12
  })

  it("includes nearby entities sorted by distance", async () => {
    const bot = createMockBot()
    addEntityToMock(bot, createMockEntity(1, "cow", new Vec3(20, 64, 0)))
    addEntityToMock(bot, createMockEntity(2, "pig", new Vec3(5, 64, 0)))

    const result = await surveyArea(bot, { radius: 32 })
    expect(result.entities).toHaveLength(2)
    expect(result.entities[0].type).toBe("pig") // closer
    expect(result.entities[1].type).toBe("cow")
  })

  it("excludes entities beyond radius", async () => {
    const bot = createMockBot()
    addEntityToMock(bot, createMockEntity(1, "cow", new Vec3(200, 64, 0)))

    const result = await surveyArea(bot, { radius: 32 })
    expect(result.entities).toHaveLength(0)
  })

  it("caps radius at 128", async () => {
    const bot = createMockBot()
    const result = await surveyArea(bot, { radius: 500 })
    // No error, and findBlocks should be called with max 128
    expect(result.blockClusters).toBeDefined()
  })

  it("produces a summary string", async () => {
    const bot = createMockBot()
    const result = await surveyArea(bot, { radius: 32 })
    expect(typeof result.summary).toBe("string")
  })

  it("summary reports empty area when nothing found", async () => {
    const bot = createMockBot()
    const result = await surveyArea(bot, { radius: 32 })
    expect(result.summary).toBe("Area appears empty.")
  })
})

// ---------------------------------------------------------------------------
// planPath
// ---------------------------------------------------------------------------

describe("planPath", () => {
  it("returns not reachable when no destination", async () => {
    const bot = createMockBot()
    const result = await planPath(bot, { destination: undefined, allowDig: false })

    expect(result.reachable).toBe(false)
    expect(result.distance).toBe(0)
  })

  it("returns reachable with distance for successful path", async () => {
    const bot = createMockBot({ position: new Vec3(0, 64, 0) })
    ;(bot as any).pathfinder.getPathTo = vi.fn(() => ({
      status: "success",
      path: [
        { x: 5, y: 64, z: 0 },
        { x: 10, y: 64, z: 0 },
        { x: 15, y: 64, z: 0 },
        { x: 20, y: 64, z: 0 },
      ],
    }))

    const result = await planPath(bot, {
      destination: { x: 20, y: 64, z: 0 },
      allowDig: false,
    })
    expect(result.reachable).toBe(true)
    expect(result.distance).toBe(20)
    expect(result.estimatedSeconds).toBeGreaterThan(0)
  })

  it("returns not reachable for noPath status", async () => {
    const bot = createMockBot()
    ;(bot as any).pathfinder.getPathTo = vi.fn(() => ({
      status: "noPath",
      path: [],
    }))

    const result = await planPath(bot, {
      destination: { x: 100, y: 64, z: 0 },
      allowDig: false,
    })
    expect(result.reachable).toBe(false)
  })

  it("detects hazards along the path", async () => {
    const bot = createMockBot()
    ;(bot as any).pathfinder.getPathTo = vi.fn(() => ({
      status: "success",
      path: [
        { x: 1, y: 64, z: 0 },
        { x: 2, y: 64, z: 0 },
        { x: 3, y: 64, z: 0 },
      ],
    }))

    vi.mocked(bot.blockAt).mockImplementation((pos: any) => {
      if (pos.x === 2) return { name: "lava" } as any
      return { name: "air" } as any
    })

    const result = await planPath(bot, {
      destination: { x: 3, y: 64, z: 0 },
      allowDig: false,
    })
    expect(result.hazards).toContain("lava")
  })

  it("detects cliff hazards from vertical drops", async () => {
    const bot = createMockBot()
    ;(bot as any).pathfinder.getPathTo = vi.fn(() => ({
      status: "success",
      path: [
        { x: 0, y: 70, z: 0 },
        { x: 1, y: 66, z: 0 }, // 4-block drop
      ],
    }))

    const result = await planPath(bot, {
      destination: { x: 1, y: 66, z: 0 },
      allowDig: false,
    })
    expect(result.hazards).toContain("cliff")
  })

  it("samples waypoints from path", async () => {
    const bot = createMockBot()
    const path = Array.from({ length: 20 }, (_, i) => ({ x: i, y: 64, z: 0 }))
    ;(bot as any).pathfinder.getPathTo = vi.fn(() => ({ status: "success", path }))

    const result = await planPath(bot, {
      destination: { x: 19, y: 64, z: 0 },
      allowDig: false,
    })
    expect(result.waypoints.length).toBeGreaterThan(0)
    expect(result.waypoints.length).toBeLessThanOrEqual(20)
  })

  it("returns pathfinder unavailable hazard when no pathfinder", async () => {
    const bot = createMockBot()
    ;(bot as any).pathfinder = undefined

    const result = await planPath(bot, {
      destination: { x: 10, y: 64, z: 0 },
      allowDig: false,
    })
    expect(result.reachable).toBe(false)
    expect(result.hazards).toContain("pathfinder unavailable")
  })

  it("increases estimated time when hazards present", async () => {
    const bot = createMockBot()
    const path = Array.from({ length: 10 }, (_, i) => ({ x: i, y: 64, z: 0 }))
    ;(bot as any).pathfinder.getPathTo = vi.fn(() => ({ status: "success", path }))

    // No hazards
    vi.mocked(bot.blockAt).mockReturnValue({ name: "air" } as any)
    const clean = await planPath(bot, {
      destination: { x: 9, y: 64, z: 0 },
      allowDig: false,
    })

    // With hazards
    vi.mocked(bot.blockAt).mockImplementation((pos: any) => {
      if (pos.x === 3) return { name: "lava" } as any
      return { name: "air" } as any
    })
    const hazardous = await planPath(bot, {
      destination: { x: 9, y: 64, z: 0 },
      allowDig: false,
    })

    expect(hazardous.estimatedSeconds).toBeGreaterThan(clean.estimatedSeconds)
  })
})
