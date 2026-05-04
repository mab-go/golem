import {
  Server,
  ServerCredentials,
  status,
  type ServerUnaryCall,
  type sendUnaryData,
} from "@grpc/grpc-js"
import { MinecraftServiceService, type MinecraftServiceServer } from "./generated/minecraft.js"
import { getBot } from "../bot/connection.js"
import { readVitalSigns, readSurroundings, readInventory } from "../bot/perception.js"
import { addSubscriber } from "../bot/events.js"
import * as tier0 from "../bot/actions/tier0.js"
import * as tier1 from "../bot/actions/tier1.js"
import * as tier2 from "../bot/actions/tier2.js"
import * as tier3 from "../bot/actions/tier3.js"
import { captureScreenshot } from "../screenshot/capture.js"
import type { Bot } from "mineflayer"

/** Returns the connected bot or throws an UNAVAILABLE gRPC error. */
function requireBot(): Bot {
  const bot = getBot()
  if (!bot) {
    throw { code: status.UNAVAILABLE, details: "Bot not connected to Minecraft server" }
  }
  return bot
}

/** Wraps an async action function as a gRPC unary handler with bot injection. */
function asyncHandler<Req, Res>(
  fn: (bot: Bot, req: Req) => Promise<Res>,
): (call: ServerUnaryCall<Req, Res>, callback: sendUnaryData<Res>) => void {
  return (call, callback) => {
    try {
      const bot = requireBot()
      let cancelled = false
      call.on("cancelled", () => {
        cancelled = true
        bot.pathfinder.stop()
      })
      fn(bot, call.request).then(
        res => {
          if (!cancelled) callback(null, res)
        },
        err => {
          if (!cancelled) {
            callback({ code: status.INTERNAL, details: String(err) } as any)
          }
        },
      )
    } catch (err) {
      callback(err as any)
    }
  }
}

const serviceImpl: MinecraftServiceServer = {
  // ---- Perception -----------------------------------------------------------

  getVitalSigns(_call, callback) {
    try {
      callback(null, readVitalSigns(requireBot()))
    } catch (err) {
      callback(err as any)
    }
  },

  getSurroundings: asyncHandler(async (bot, req) => {
    const radius = req.radius || 16
    return readSurroundings(bot, radius)
  }),

  getInventory: asyncHandler(async (bot, req) => {
    return readInventory(bot, req.includeCraftSuggestions)
  }),

  // ---- Tier 0: SendChat -----------------------------------------------------

  sendChat(call, callback) {
    try {
      const bot = requireBot()
      bot.chat(call.request.message)
      callback(null, {
        result: { success: true, error: "", message: "Chat sent" },
      })
    } catch (err) {
      callback(err as any)
    }
  },

  // ---- Perception -- Screenshot ----------------------------------------------

  takeScreenshot(call, callback) {
    try {
      // Ensure a bot exists before calling capture; the bot is required to
      // stamp camera metadata on the response.
      requireBot()
      captureScreenshot(call.request.resolution, call.request.lookAt).then(
        res => callback(null, res),
        err => callback({ code: status.INTERNAL, details: String(err) } as any),
      )
    } catch (err) {
      callback(err as any)
    }
  },

  // ---- Tier 0: Atomic Actions ------------------------------------------------

  moveTo: asyncHandler(tier0.moveTo),
  lookAt: asyncHandler(tier0.lookAt),
  placeBlock: asyncHandler(tier0.placeBlock),
  digBlock: asyncHandler(tier0.digBlock),
  equipItem: asyncHandler(tier0.equipItem),
  useItem: asyncHandler(tier0.useItem),
  attackEntity: asyncHandler(tier0.attackEntity),
  jump: asyncHandler(tier0.jump),
  setSneak: asyncHandler(tier0.setSneak),

  // ---- Tier 1: Action Verbs --------------------------------------------------

  navigateTo: asyncHandler(tier1.navigateTo),
  interactWithEntity: asyncHandler(tier1.interactWithEntity),
  openContainer: asyncHandler(tier1.openContainer),
  withdrawFromContainer: asyncHandler(tier1.withdrawFromContainer),
  depositToContainer: asyncHandler(tier1.depositToContainer),
  craftItem: asyncHandler(tier1.craftItem),
  smeltItem: asyncHandler(tier1.smeltItem),
  eat: asyncHandler(tier1.eat),

  // ---- Tier 2: Goal-Oriented Tasks (server-streaming) -----------------------

  harvestBlock: tier2.harvestBlock,
  gather: tier2.gather,
  buildStructure: tier2.buildStructure,
  processAll: tier2.processAll,
  organizeInventory: tier2.organizeInventory,
  clearArea: tier2.clearArea,
  farm: tier2.farm,

  // ---- Tier 3: Strategic / Planning -----------------------------------------

  surveyArea: asyncHandler(tier3.surveyArea),
  findNearest: asyncHandler(tier3.findNearest),
  whatCanICraft: asyncHandler(tier3.whatCanICraft),
  assessThreat: asyncHandler(tier3.assessThreat),
  planPath: asyncHandler(tier3.planPath),

  // ---- Events (server-streaming) ---------------------------------------------

  subscribeEvents(call) {
    const filterTypes = call.request.filter ?? []
    const subscriber = addSubscriber(call, filterTypes)

    call.on("cancelled", () => {
      subscriber.unsubscribe()
    })

    call.on("error", err => {
      if ((err as any).code !== status.CANCELLED) {
        console.error(`[events] Subscriber ${subscriber.id} error:`, (err as Error).message)
      }
      subscriber.unsubscribe()
    })
  },
}

/** Creates a gRPC server with the MinecraftService registered. */
export function createServer(): Server {
  const server = new Server()
  server.addService(MinecraftServiceService, serviceImpl)
  return server
}

/** Binds and starts the gRPC server on the given port. */
export function startServer(server: Server, port: number): Promise<number> {
  return new Promise((resolve, reject) => {
    server.bindAsync(`0.0.0.0:${port}`, ServerCredentials.createInsecure(), (err, boundPort) => {
      if (err) {
        reject(err)
        return
      }
      resolve(boundPort)
    })
  })
}
