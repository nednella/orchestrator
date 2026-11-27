import type { FileChip } from './types'

// A path found missing is looked up again after this long, when a chip asks for it anew: its file may have been
// written since. A path found there is kept for good.
export const MISS_KEPT_MS = 30_000
const KEPT = 2000

type Lookup = (thread: string, paths: string[]) => Promise<FileChip[]>

// createChipStore keeps what each thread's paths are, as the app says. Paths asked for in one tick go to the app in
// one call per thread, and a path it has an answer for is not asked again.
export function createChipStore(lookup: Lookup, now: () => number = Date.now) {
  const answers = new Map<string, { chip: FileChip; at: number }>()
  const waiting = new Map<string, Set<string>>()
  const asked = new Set<string>()
  const listeners = new Set<() => void>()
  let scheduled = false
  const keyOf = (thread: string, path: string) => `${thread}\0${path}`
  const none = (path: string): FileChip => ({ path, exists: false, dir: false, size: 0, abs: path, inside: false, rel: '' })

  function keep(thread: string, chips: FileChip[], at: number) {
    for (const chip of chips) {
      const key = keyOf(thread, chip.path)
      answers.delete(key)
      answers.set(key, { chip, at })
      asked.delete(key)
    }
    for (const old of answers.keys()) {
      if (answers.size <= KEPT) break
      answers.delete(old)
    }
    listeners.forEach((listener) => listener())
  }

  async function flush() {
    scheduled = false
    const batches = [...waiting]
    waiting.clear()
    await Promise.all(
      batches.map(async ([thread, paths]) => {
        const list = [...paths]
        let chips: FileChip[]
        try {
          chips = await lookup(thread, list)
        } catch {
          chips = list.map(none)
        }
        // A path the app left out is taken as missing, so that it is not asked for again at once.
        keep(thread, list.map((path, i) => (chips[i]?.path === path ? chips[i] : none(path))), now())
      }),
    )
  }

  return {
    // chipOf is what the app said of path, undefined before it said.
    chipOf: (thread: string, path: string): FileChip | undefined => answers.get(keyOf(thread, path))?.chip,
    // ask has the app look at path, unless it answered already.
    ask(thread: string, path: string) {
      const key = keyOf(thread, path)
      const known = answers.get(key)
      if (asked.has(key) || (known && (known.chip.exists || now() - known.at < MISS_KEPT_MS))) return
      asked.add(key)
      const paths = waiting.get(thread) ?? new Set()
      paths.add(path)
      waiting.set(thread, paths)
      if (!scheduled) {
        scheduled = true
        queueMicrotask(() => void flush())
      }
    },
    subscribe(listener: () => void) {
      listeners.add(listener)
      return () => {
        listeners.delete(listener)
      }
    },
  }
}
