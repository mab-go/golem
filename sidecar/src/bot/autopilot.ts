import type { Bot } from "mineflayer"
import pathfinderPkg from "mineflayer-pathfinder"
import { ARMOR_POINTS } from "./perception.js"

const { goals, Movements } = pathfinderPkg
import { emitAutopilotEvent } from "./events.js"

const TICK_INTERVAL_MS = 2000
const FLEE_HEALTH_THRESHOLD = 8
const FLEE_DISTANCE = 4

const HOSTILE_TYPES = new Set([
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
  "drowned",
  "husk",
  "stray",
  "phantom",
  "pillager",
  "vindicator",
  "ravager",
  "vex",
  "evoker",
  "hoglin",
  "piglin_brute",
  "warden",
])

/** Armor slot index -> equipment destination name. */
const ARMOR_SLOTS: { index: number; dest: string; suffix: string }[] = [
  { index: 5, dest: "head", suffix: "helmet" },
  { index: 6, dest: "torso", suffix: "chestplate" },
  { index: 7, dest: "legs", suffix: "leggings" },
  { index: 8, dest: "feet", suffix: "boots" },
]

/**
 * SurvivalAutopilot runs periodic checks to keep the bot alive between
 * the Go agent's think cycles. Actions are emitted to the event stream
 * so Claude knows what happened.
 */
export class SurvivalAutopilot {
  private bot: Bot | null = null
  private interval: ReturnType<typeof setInterval> | null = null
  private isFleeing = false

  /** Start autopilot ticking, configure auto-eat, and wire the eat event listeners. */
  start(bot: Bot): void {
    this.stop()
    this.bot = bot

    // Configure auto-eat plugin.
    if (bot.autoEat) {
      bot.autoEat.setOpts({
        priority: "foodPoints",
        minHunger: 14,
        bannedFood: ["rotten_flesh", "spider_eye", "poisonous_potato", "pufferfish"],
        eatingTimeout: 3000,
      })
      bot.autoEat.enableAuto()

      bot.on("autoeat_started" as any, (item: any) => {
        emitAutopilotEvent(`Auto-eating ${item?.name ?? "food"}`)
      })
      bot.on("autoeat_finished" as any, () => {
        emitAutopilotEvent("Finished auto-eating")
      })
    }

    this.interval = setInterval(() => {
      this.tick().catch(err => {
        console.error("[autopilot] Tick error:", err)
      })
    }, TICK_INTERVAL_MS)

    console.log("[autopilot] Started")
  }

  /** Stop ticking, disable auto-eat, and release the bot reference. */
  stop(): void {
    if (this.interval) {
      clearInterval(this.interval)
      this.interval = null
    }

    if (this.bot?.autoEat) {
      this.bot.autoEat.disableAuto()
    }

    this.bot = null
    this.isFleeing = false
    console.log("[autopilot] Stopped")
  }

  private async tick(): Promise<void> {
    if (!this.bot) return
    const pos = this.bot.entity.position
    if (!Number.isFinite(pos.x) || !Number.isFinite(pos.y) || !Number.isFinite(pos.z)) return

    try {
      await this.autoArmor()
    } catch (err) {
      console.error("[autopilot] Auto-armor error:", err)
    }

    try {
      await this.threatFlee()
    } catch (err) {
      console.error("[autopilot] Threat-flee error:", err)
    }
  }

  private async autoArmor(): Promise<void> {
    const bot = this.bot
    if (!bot) return

    for (const slot of ARMOR_SLOTS) {
      const equipped = bot.inventory.slots[slot.index]
      const equippedPoints = equipped ? (ARMOR_POINTS[equipped.name] ?? 0) : 0

      // Search inventory for better armor for this slot.
      let bestItem = equipped
      let bestPoints = equippedPoints

      for (const item of bot.inventory.items()) {
        if (!item.name.endsWith(slot.suffix)) continue
        const points = ARMOR_POINTS[item.name] ?? 0
        if (points > bestPoints) {
          bestItem = item
          bestPoints = points
        }
      }

      if (bestItem && bestItem !== equipped) {
        await bot.equip(bestItem, slot.dest as any)
        emitAutopilotEvent(`Auto-equipped ${bestItem.name} (${slot.dest})`)
      }
    }
  }

  private async threatFlee(): Promise<void> {
    const bot = this.bot
    if (!bot || this.isFleeing) return
    if (bot.health >= FLEE_HEALTH_THRESHOLD) return

    // Scan for nearby hostiles.
    let nearestHostile: { id: number; position: { x: number; y: number; z: number } } | null = null
    let nearestDist = Infinity

    for (const entity of Object.values(bot.entities)) {
      if (entity === bot.entity) continue
      const name = (entity.name ?? entity.type ?? "").toLowerCase()
      if (!HOSTILE_TYPES.has(name)) continue

      const dist = entity.position.distanceTo(bot.entity.position)
      if (dist <= FLEE_DISTANCE && dist < nearestDist) {
        nearestHostile = entity
        nearestDist = dist
      }
    }

    if (!nearestHostile) return

    this.isFleeing = true
    const hostileName = (nearestHostile as any).name ?? (nearestHostile as any).type ?? "hostile"
    emitAutopilotEvent(`Fleeing from ${hostileName}! Health: ${bot.health.toFixed(1)}`)

    try {
      const movements = new Movements(bot)
      movements.allowSprinting = true
      bot.pathfinder.setMovements(movements)

      const hp = nearestHostile.position
      const fleeGoal = new goals.GoalInvert(new goals.GoalNear(hp.x, hp.y, hp.z, 20))
      await bot.pathfinder.goto(fleeGoal)
    } catch {
      // Flee attempt failed -- that's okay, we tried.
    } finally {
      this.isFleeing = false
    }
  }
}
