const CHECK_INTERVAL_MS = 2000
const LAG_WARN_THRESHOLD_MS = 500
const LAG_CRITICAL_THRESHOLD_MS = 5000

let timer: ReturnType<typeof setInterval> | null = null

export function startEventLoopMonitor(): void {
  if (timer) return
  let lastTick = Date.now()
  timer = setInterval(() => {
    const now = Date.now()
    const lag = now - lastTick - CHECK_INTERVAL_MS
    lastTick = now
    if (lag > LAG_CRITICAL_THRESHOLD_MS) {
      console.error(`[eventloop] CRITICAL: event loop blocked for ${lag}ms`)
    } else if (lag > LAG_WARN_THRESHOLD_MS) {
      console.warn(`[eventloop] WARNING: event loop lag ${lag}ms`)
    }
  }, CHECK_INTERVAL_MS)
  timer.unref()
  console.log("[eventloop] Monitor started")
}

export function stopEventLoopMonitor(): void {
  if (timer) {
    clearInterval(timer)
    timer = null
  }
}
