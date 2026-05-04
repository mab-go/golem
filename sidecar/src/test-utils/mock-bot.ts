import { vi } from "vitest"
import type { Bot } from "mineflayer"
import { Vec3 } from "vec3"

interface MockItem {
  name: string
  count: number
  slot: number
  displayName: string
  maxDurability?: number
  durabilityUsed?: number
}

interface MockEntity {
  id: number
  name?: string
  username?: string
  type?: string
  position: Vec3
  health?: number
  effects?: unknown[]
  yaw?: number
  pitch?: number
}

interface MockBlock {
  name: string
  light?: number
  skyLight?: number
  position?: Vec3
  biome?: { name: string }
}

interface MockBotOptions {
  position?: Vec3
  health?: number
  food?: number
  foodSaturation?: number
  oxygenLevel?: number
  xpLevel?: number
  xpProgress?: number
  gameMode?: string
  dimension?: string
  timeOfDay?: number
  isRaining?: boolean
  thunderState?: number
  heldItem?: MockItem | null
}

export function createMockBot(opts?: MockBotOptions): Bot {
  const position = opts?.position ?? new Vec3(0, 64, 0)
  const inventoryItems: MockItem[] = []
  const inventorySlots: (MockItem | null)[] = new Array(46).fill(null)
  const inventoryListeners = new Map<string, Set<(...args: unknown[]) => void>>()

  const selfEntity: MockEntity = {
    id: 0,
    name: "player",
    type: "player",
    position,
    health: opts?.health ?? 20,
    effects: [],
    yaw: 0,
    pitch: 0,
  }

  const entities: Record<number, MockEntity> = { 0: selfEntity }

  const blocksByName: Record<string, { id: number; boundingBox?: string }> = {}
  const itemsByName: Record<string, { id: number; name: string }> = {}
  const itemsById: Record<number, { id: number; name: string }> = {}

  const blockMap = new Map<string, MockBlock>()

  const bot = {
    entity: selfEntity,
    entities,
    health: opts?.health ?? 20,
    food: opts?.food ?? 20,
    foodSaturation: opts?.foodSaturation ?? 5,
    oxygenLevel: opts?.oxygenLevel ?? 300,
    experience: {
      level: opts?.xpLevel ?? 0,
      progress: opts?.xpProgress ?? 0,
    },
    game: {
      gameMode: opts?.gameMode ?? "survival",
      dimension: opts?.dimension ?? "minecraft:overworld",
    },
    time: { timeOfDay: opts?.timeOfDay ?? 6000 },
    isRaining: opts?.isRaining ?? false,
    thunderState: opts?.thunderState ?? 0,
    heldItem: opts?.heldItem ?? null,
    inventory: {
      items: vi.fn(() => [...inventoryItems]),
      slots: inventorySlots,
      on: vi.fn((event: string, fn: (...args: unknown[]) => void) => {
        if (!inventoryListeners.has(event)) inventoryListeners.set(event, new Set())
        inventoryListeners.get(event)!.add(fn)
      }),
      removeListener: vi.fn((event: string, fn: (...args: unknown[]) => void) => {
        inventoryListeners.get(event)?.delete(fn)
      }),
    },
    registry: {
      blocksByName,
      itemsByName,
      items: itemsById,
      blocksArray: [] as Array<{ id: number; boundingBox?: string }>,
      effects: {} as Record<number, { name: string }>,
    },
    blockAt: vi.fn((pos: Vec3) => {
      const key = `${pos.x},${pos.y},${pos.z}`
      return blockMap.get(key) ?? null
    }),
    findBlock: vi.fn(() => null),
    findBlocks: vi.fn(() => []),
    recipesFor: vi.fn(() => []),
    recipesAll: vi.fn(() => []),
    pathfinder: {
      goto: vi.fn(() => Promise.resolve()),
      stop: vi.fn(),
      setMovements: vi.fn(),
      getPathTo: vi.fn(() => ({ status: "noPath", path: [] })),
    },
    craft: vi.fn(),
    equip: vi.fn(),
    attack: vi.fn(),
    chat: vi.fn(),
    lookAt: vi.fn(),
  } as unknown as Bot

  return bot
}

export function addItemToInventory(bot: Bot, item: MockItem): void {
  const inv = (bot as any).inventory
  const items = inv.items as ReturnType<typeof vi.fn>
  const current = items.getMockImplementation()?.() ?? []
  current.push(item)
  items.mockImplementation(() => [...current])
}

export function addEntityToMock(bot: Bot, entity: MockEntity): void {
  ;(bot as any).entities[entity.id] = entity
}

export function addBlockToRegistry(
  bot: Bot,
  name: string,
  opts?: { id?: number; boundingBox?: string },
): void {
  const reg = (bot as any).registry
  const id = opts?.id ?? Object.keys(reg.blocksByName).length + 1
  const entry = { id, name, boundingBox: opts?.boundingBox ?? "block" }
  reg.blocksByName[name] = entry
  reg.blocksArray.push(entry)
}

export function addItemToRegistry(bot: Bot, name: string, opts?: { id?: number }): void {
  const reg = (bot as any).registry
  const id = opts?.id ?? Object.keys(reg.itemsByName).length + 1
  const entry = { id, name }
  reg.itemsByName[name] = entry
  reg.items[id] = entry
}

export function setBlockAt(bot: Bot, pos: Vec3, block: MockBlock): void {
  const blockMap = getBlockMap(bot)
  block.position = pos
  blockMap.set(`${pos.x},${pos.y},${pos.z}`, block)
}

function getBlockMap(bot: Bot): Map<string, MockBlock> {
  const fn = (bot as any).blockAt as ReturnType<typeof vi.fn>
  const impl = fn.getMockImplementation()
  if (!impl) {
    const map = new Map<string, MockBlock>()
    fn.mockImplementation((pos: Vec3) => map.get(`${pos.x},${pos.y},${pos.z}`) ?? null)
    return map
  }
  // Access the closure's map by calling with a sentinel and checking behavior.
  // Simpler: just re-create with a shared map.
  const map = new Map<string, MockBlock>()
  fn.mockImplementation((pos: Vec3) => map.get(`${pos.x},${pos.y},${pos.z}`) ?? null)
  return map
}

export function createMockItem(
  name: string,
  count: number = 1,
  overrides?: Partial<MockItem>,
): MockItem {
  return {
    name,
    count,
    slot: overrides?.slot ?? 0,
    displayName: overrides?.displayName ?? name.replace(/_/g, " "),
    maxDurability: overrides?.maxDurability,
    durabilityUsed: overrides?.durabilityUsed,
  }
}

export function createMockEntity(
  id: number,
  name: string,
  position: Vec3,
  overrides?: Partial<MockEntity>,
): MockEntity {
  return {
    id,
    name,
    type: overrides?.type ?? name,
    position,
    health: overrides?.health ?? 20,
    username: overrides?.username,
    effects: overrides?.effects ?? [],
    yaw: overrides?.yaw ?? 0,
    pitch: overrides?.pitch ?? 0,
  }
}
