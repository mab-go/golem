import { chromium, type Browser, type Page } from "playwright"
import { Vec3 } from "vec3"
import { getBot } from "../bot/connection.js"
import { consumeReloadFlag, getViewerPort } from "./viewer.js"
import {
  ScreenshotResolution,
  type TakeScreenshotResponse,
  type Vec3 as ProtoVec3,
} from "../grpc/generated/minecraft.js"

interface Resolution {
  w: number
  h: number
}

function resolveDimensions(resolution: ScreenshotResolution): Resolution {
  if (resolution === ScreenshotResolution.SCREENSHOT_RESOLUTION_HIGH) {
    return { w: 960, h: 720 }
  }
  return { w: 480, h: 360 }
}

let browser: Browser | null = null
let page: Page | null = null
let ensurePromise: Promise<void> | null = null

// Polls until the Three.js canvas contains at least one rendered frame (non-uniform
// pixels), reading pixels inside requestAnimationFrame so we see the frame before
// the WebGL buffer is cleared. Throws if the scene hasn't rendered within timeoutMs.
async function waitForRender(page: Page, timeoutMs = 5_000): Promise<void> {
  await page
    .waitForFunction(
      (): Promise<boolean> =>
        new Promise(resolve => {
          requestAnimationFrame(() => {
            const canvas = document.querySelector("canvas") as HTMLCanvasElement | null
            if (!canvas || canvas.width === 0 || canvas.height === 0) {
              resolve(false)
              return
            }
            try {
              const off = document.createElement("canvas")
              off.width = 8
              off.height = 8
              const ctx = off.getContext("2d")
              if (!ctx) {
                resolve(false)
                return
              }
              ctx.drawImage(canvas, 0, 0, 8, 8)
              const { data } = ctx.getImageData(0, 0, 8, 8)
              const r0 = data[0],
                g0 = data[1],
                b0 = data[2]
              for (let i = 4; i < data.length; i += 4) {
                if (data[i] !== r0 || data[i + 1] !== g0 || data[i + 2] !== b0) {
                  resolve(true)
                  return
                }
              }
              resolve(false)
            } catch {
              resolve(false)
            }
          })
        }),
      { timeout: timeoutMs, polling: 100 },
    )
    .catch(() => {
      throw new Error(`viewer not rendering after ${timeoutMs}ms -- the scene may still be loading`)
    })
}

async function ensureBrowser(): Promise<void> {
  if (page) return
  if (ensurePromise) return ensurePromise

  ensurePromise = (async () => {
    const port = getViewerPort()
    console.log(`[capture] Launching headless Chromium; connecting to viewer on port ${port}`)
    browser = await chromium.launch({ headless: true })
    page = await browser.newPage()
    await page.goto(`http://127.0.0.1:${port}`, { waitUntil: "networkidle" })
    await waitForRender(page!)
    console.log("[capture] Playwright page ready")
  })()

  try {
    await ensurePromise
  } finally {
    ensurePromise = null
  }
}

/** Capture a PNG screenshot from the prismarine-viewer page at the requested resolution. */
export async function captureScreenshot(
  resolution: ScreenshotResolution,
  lookAt?: ProtoVec3,
): Promise<TakeScreenshotResponse> {
  await ensureBrowser()
  if (!page) {
    throw new Error("Playwright page unavailable")
  }

  if (consumeReloadFlag()) {
    const port = getViewerPort()
    await page.goto(`http://127.0.0.1:${port}`, { waitUntil: "networkidle" })
    await waitForRender(page)
  }

  const dim = resolveDimensions(resolution)
  await page.setViewportSize({ width: dim.w, height: dim.h })

  const bot = getBot()
  if (lookAt && bot) {
    try {
      await bot.lookAt(new Vec3(lookAt.x, lookAt.y, lookAt.z))
    } catch {
      // Non-fatal -- take the screenshot regardless.
    }
    // Allow the viewer's websocket to sync the new camera state.
    await page.waitForTimeout(150)
  }

  const buffer = await page.screenshot({ type: "png" })

  const pos = bot?.entity.position
  return {
    imageData: buffer,
    width: dim.w,
    height: dim.h,
    cameraPosition: pos ? { x: pos.x, y: pos.y, z: pos.z } : { x: 0, y: 0, z: 0 },
    cameraYaw: bot?.entity.yaw ?? 0,
    cameraPitch: bot?.entity.pitch ?? 0,
  }
}

/** Closes the browser on sidecar shutdown. */
export async function closeBrowser(): Promise<void> {
  if (page) {
    try {
      await page.close()
    } catch {
      // ignore
    }
    page = null
  }
  if (browser) {
    try {
      await browser.close()
    } catch {
      // ignore
    }
    browser = null
  }
}
