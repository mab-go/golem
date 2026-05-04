import { connectBot, disconnectBot, botEvents } from "./bot/connection.js"
import { startEventLoopMonitor, stopEventLoopMonitor } from "./bot/eventloop-monitor.js"
import { registerBotListeners, unregisterBotListeners } from "./bot/events.js"
import { createServer, startServer } from "./grpc/server.js"
import { SurvivalAutopilot } from "./bot/autopilot.js"
import { startViewer, markViewerStale } from "./screenshot/viewer.js"
import { closeBrowser } from "./screenshot/capture.js"
import type { Bot } from "mineflayer"

const autopilot = new SurvivalAutopilot()

const port = parseInt(process.env.GOLEM_SIDECAR_PORT ?? "50051", 10)

// 1. Connect bot and wait for spawn.
console.log("[sidecar] Connecting to Minecraft server...")
const bot = await connectBot()
console.log("[sidecar] Bot connected and spawned.")

// 2. Register event listeners on the live bot and start autopilot.
registerBotListeners(bot)
autopilot.start(bot)
startViewer(bot)
startEventLoopMonitor()

// 3. Wire reconnection lifecycle.
botEvents.on("botLost", (oldBot: Bot) => {
  console.log("[sidecar] Bot lost connection, unregistering listeners...")
  autopilot.stop()
  unregisterBotListeners(oldBot)
  markViewerStale()
})

botEvents.on("botReady", (newBot: Bot) => {
  console.log("[sidecar] Bot reconnected, re-registering listeners...")
  registerBotListeners(newBot)
  autopilot.start(newBot)
  startViewer(newBot)
})

// 4. Start gRPC server (after bot is live).
const server = createServer()
const boundPort = await startServer(server, port)
console.log(`[sidecar] gRPC server listening on 0.0.0.0:${boundPort}`)

// 5. Graceful shutdown.
async function shutdown() {
  console.log("[sidecar] Shutting down...")
  stopEventLoopMonitor()
  autopilot.stop()
  await closeBrowser()
  disconnectBot()
  server.tryShutdown(err => {
    if (err) {
      console.error("[sidecar] Error during gRPC shutdown:", err)
      process.exit(1)
    }
    console.log("[sidecar] Shutdown complete.")
    process.exit(0)
  })
}

process.on("SIGINT", shutdown)
process.on("SIGTERM", shutdown)
