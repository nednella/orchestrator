import { TOAST_MS } from './toastPile.ts'
import type { Toast } from './toastPile'

export type ToastClock = {
  now(): number
  setTimeout(run: () => void, ms: number): unknown
  clearTimeout(timer: unknown): void
}

const browserClock: ToastClock = {
  now: () => performance.now(),
  setTimeout: (run, ms) => window.setTimeout(run, ms),
  clearTimeout: (timer) => window.clearTimeout(timer as number),
}

type Running = { toast: Toast; remaining: number; startedAt: number; timer: unknown }

// The expiry of a pile's toasts: each toast that is not sticky runs for TOAST_MS, all of them stop while the pile is
// paused and go on with what is left of their time.
export function createToastTimers(expire: (key: number) => void, ms = TOAST_MS, clock: ToastClock = browserClock) {
  const running = new Map<number, Running>()
  let paused = false

  const run = (entry: Running, key: number) => {
    entry.startedAt = clock.now()
    entry.timer = clock.setTimeout(() => {
      running.delete(key)
      expire(key)
    }, entry.remaining)
  }

  const stop = (key: number) => {
    const entry = running.get(key)
    if (entry) clock.clearTimeout(entry.timer)
    running.delete(key)
  }

  return {
    // sync starts a timer for each toast it has not seen, restarts one whose toast was replaced, and stops those of toasts that are gone.
    sync(toasts: Toast[]) {
      const keys = new Set(toasts.map((toast) => toast.key))
      for (const key of [...running.keys()]) if (!keys.has(key)) stop(key)
      for (const toast of toasts) {
        if (running.get(toast.key)?.toast === toast) continue
        stop(toast.key)
        if (toast.sticky) continue
        const entry: Running = { toast, remaining: ms, startedAt: 0, timer: undefined }
        running.set(toast.key, entry)
        if (!paused) run(entry, toast.key)
      }
    },
    setPaused(next: boolean) {
      if (next === paused) return
      paused = next
      for (const [key, entry] of running) {
        if (paused) {
          clock.clearTimeout(entry.timer)
          entry.remaining -= clock.now() - entry.startedAt
        } else {
          run(entry, key)
        }
      }
    },
    dispose() {
      for (const entry of running.values()) clock.clearTimeout(entry.timer)
      running.clear()
    },
  }
}
