import { useSyncExternalStore } from 'react'

let open = false
const listeners = new Set<() => void>()

function set(next: boolean) {
  if (open === next) return
  open = next
  listeners.forEach((l) => l())
}

function subscribe(listener: () => void) {
  listeners.add(listener)
  return () => listeners.delete(listener)
}

export function useUsageOpen(): boolean {
  return useSyncExternalStore(subscribe, () => open)
}

export const openUsage = () => set(true)
export const closeUsage = () => set(false)
export const toggleUsage = () => set(!open)
