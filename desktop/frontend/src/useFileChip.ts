import { useEffect, useSyncExternalStore } from 'react'
import { api } from './api'
import { createChipStore } from './chipStore'
import type { FileChip } from './types'

const chips = createChipStore((thread, paths) => api.chatFileChips(thread, paths))

// useFileChip is what the app says is at path, for a thread, undefined until it says. The paths of the chips that show
// together go to it in one call.
export function useFileChip(thread: string | undefined, path: string): FileChip | undefined {
  useEffect(() => {
    if (thread) chips.ask(thread, path)
  }, [thread, path])
  return useSyncExternalStore(chips.subscribe, () => (thread ? chips.chipOf(thread, path) : undefined))
}
