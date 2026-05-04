import { describe, it, expect, vi, beforeEach, afterEach } from "vitest"
import { startEventLoopMonitor, stopEventLoopMonitor } from "./eventloop-monitor.js"

describe("eventloop-monitor", () => {
  beforeEach(() => {
    vi.useFakeTimers()
  })

  afterEach(() => {
    stopEventLoopMonitor()
    vi.useRealTimers()
  })

  it("starts and stops without error", () => {
    const logSpy = vi.spyOn(console, "log").mockImplementation(() => {})
    startEventLoopMonitor()
    stopEventLoopMonitor()
    logSpy.mockRestore()
  })

  it("logs startup message", () => {
    const logSpy = vi.spyOn(console, "log").mockImplementation(() => {})
    startEventLoopMonitor()

    expect(logSpy).toHaveBeenCalledWith("[eventloop] Monitor started")
    logSpy.mockRestore()
  })

  it("is idempotent on double start", () => {
    const logSpy = vi.spyOn(console, "log").mockImplementation(() => {})
    startEventLoopMonitor()
    startEventLoopMonitor()

    expect(logSpy).toHaveBeenCalledTimes(1)
    logSpy.mockRestore()
  })

  it("can be stopped when not started", () => {
    expect(() => stopEventLoopMonitor()).not.toThrow()
  })

  it("can be restarted after stop", () => {
    const logSpy = vi.spyOn(console, "log").mockImplementation(() => {})
    startEventLoopMonitor()
    stopEventLoopMonitor()
    startEventLoopMonitor()

    expect(logSpy).toHaveBeenCalledTimes(2)
    logSpy.mockRestore()
  })
})
