import type { Bot } from "mineflayer"

// prismarine-viewer exports CJS shape only; import with a broad type to avoid
// tight coupling to a specific bundled @types declaration.
// eslint-disable-next-line @typescript-eslint/no-require-imports
import pkg from "prismarine-viewer"

const mineflayerViewer = (
  pkg as unknown as {
    mineflayer: (
      bot: Bot,
      opts: { port: number; firstPerson?: boolean; viewDistance?: number },
    ) => void
  }
).mineflayer

const DEFAULT_PORT = parseInt(process.env.GOLEM_VIEWER_PORT ?? "3000", 10)

let started = false
let port = DEFAULT_PORT

/** Returns the port the viewer is bound to (may differ if the default is in use). */
export function getViewerPort(): number {
  return port
}

/** Starts the prismarine-viewer web server for the given bot. Idempotent. */
export function startViewer(bot: Bot): void {
  if (started) {
    console.log(`[viewer] Already running on port ${port}`)
    return
  }
  port = DEFAULT_PORT
  try {
    mineflayerViewer(bot, { port, firstPerson: true, viewDistance: 10 })
    started = true
    console.log(`[viewer] First-person viewer started on port ${port}`)
  } catch (err) {
    console.error(
      `[viewer] Failed to start on port ${port}:`,
      err instanceof Error ? err.message : err,
    )
  }
}

/**
 * Marks the viewer as stopped from the bot-lifecycle perspective. prismarine-viewer
 * doesn't expose a clean stop API; the HTTP server stays listening, but after
 * `markViewerStale()` the screenshot capture module knows to reload the page
 * on next request.
 */
export function markViewerStale(): void {
  if (!started) return
  console.log("[viewer] Marked stale; capture will reload on next screenshot")
  needsReload = true
}

let needsReload = false

/** Return true and clear the stale flag if the viewer page needs reloading. */
export function consumeReloadFlag(): boolean {
  if (needsReload) {
    needsReload = false
    return true
  }
  return false
}
