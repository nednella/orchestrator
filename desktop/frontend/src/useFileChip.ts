import { useEffect, useSyncExternalStore } from 'react'
import { api } from './api'
import { createChipStore } from './chipStore'
import type { FileChip } from './types'

const chips = createChipStore((thread, paths) => api.chatFileChips(thread, paths))

// useFileChip is what the app says is at path, for a thread, undefined until it says. The paths of the chips that show
// together go to it in one call. A negative answer is asked for again after a while, when a chip next mounts.
export function useFileChip(thread: string | undefined, path: string): FileChip | undefined {
  const chip = useSyncExternalStore(chips.subscribe, () => (thread ? chips.chipOf(thread, path) : undefined))
  // A chip whose answer went asks again.
  const unknown = chip === undefined
  useEffect(() => {
    if (thread) chips.ask(thread, path)
  }, [thread, path, unknown])
  return chip
}
