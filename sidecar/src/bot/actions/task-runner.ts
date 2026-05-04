import { randomUUID } from "node:crypto"
import type { ServerWritableStream } from "@grpc/grpc-js"
import type { Bot } from "mineflayer"
import { TaskStatus, type TaskProgress, type TaskResult } from "../../grpc/generated/minecraft.js"

/** Exposed to task bodies for progress reporting and cancellation checks. */
export interface TaskContext {
  readonly taskId: string
  /** Returns true once the client has cancelled the stream. */
  cancelled(): boolean
  /** Send an IN_PROGRESS update. Safe to call frequently. */
  progress(message: string, current: number, total: number): void
}

/**
 * Shared lifecycle wrapper for Tier 2 streaming tasks. Generates a task_id,
 * emits STARTED, forwards IN_PROGRESS calls from the body, and emits a
 * terminal COMPLETED / FAILED / CANCELLED message. Wires `call.on('cancelled')`
 * so long-running loops can poll `tc.cancelled()` and exit cleanly.
 *
 * When a bot is provided, pathfinder.stop() is called on cancellation and
 * after the task body returns, preventing orphaned navigation/dig operations.
 */
export async function runTask<Req>(
  call: ServerWritableStream<Req, TaskProgress>,
  taskName: string,
  body: (tc: TaskContext) => Promise<TaskResult>,
  bot?: Bot,
): Promise<void> {
  const taskId = randomUUID()
  let cancelled = false
  let streamBroken = false

  call.on("cancelled", () => {
    cancelled = true
    bot?.pathfinder.stop()
  })

  const write = (p: Partial<TaskProgress>) => {
    // Once a write has failed the stream is unrecoverable; don't keep
    // calling .write() on a broken stream (and don't re-log the same break).
    if (streamBroken) return
    try {
      call.write({
        taskId,
        status: TaskStatus.TASK_IN_PROGRESS,
        message: "",
        current: 0,
        total: 0,
        result: undefined,
        ...p,
      })
    } catch (err) {
      streamBroken = true
      cancelled = true
      const msg = err instanceof Error ? err.message : String(err)
      console.error(
        `[task-runner] ${taskName} (${taskId}) stream write failed — ` +
          `client will see only pre-break progress. status=${p.status ?? "unknown"} err=${msg}`,
      )
    }
  }

  write({ status: TaskStatus.TASK_STARTED, message: `${taskName} starting` })

  try {
    const tc: TaskContext = {
      taskId,
      cancelled: () => cancelled,
      progress: (message, current, total) =>
        write({ status: TaskStatus.TASK_IN_PROGRESS, message, current, total }),
    }
    const result = await body(tc)
    if (cancelled) {
      write({
        status: TaskStatus.TASK_CANCELLED,
        message: `${taskName} cancelled`,
        result,
      })
    } else {
      write({
        status: TaskStatus.TASK_COMPLETED,
        message: result.summary,
        result,
      })
    }
  } catch (err) {
    const msg = err instanceof Error ? err.message : String(err)
    if (cancelled) {
      write({ status: TaskStatus.TASK_CANCELLED, message: `${taskName} cancelled: ${msg}` })
    } else {
      write({ status: TaskStatus.TASK_FAILED, message: msg })
    }
  } finally {
    bot?.pathfinder.stop()
    try {
      call.end()
    } catch {
      // Stream already closed.
    }
  }
}
