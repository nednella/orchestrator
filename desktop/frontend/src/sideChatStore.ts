import { useSyncExternalStore } from 'react'

// What the app keeps of side chats while it runs: why each thread whose chat is loaded cannot start one.
const reasons = new Map<string, string>()
const listeners = new Set<() => void>()

function changed() {
  listeners.forEach((listener) => listener())
}

function subscribe(listener: () => void) {
  listeners.add(listener)
  return () => listeners.delete(listener)
}

// noteSideChatReason keeps why the thread cannot start a side chat, '' when it can, as its loaded chat says.
export function noteSideChatReason(thread: string, reason: string) {
  if (reasons.get(thread) === reason) return
  reasons.set(thread, reason)
  changed()
}

// useSideChatReason is why the thread cannot start a side chat as far as its chat says, '' when it can or while its chat
// has not loaded.
export function useSideChatReason(thread: string): string {
  return useSyncExternalStore(subscribe, () => reasons.get(thread) ?? '')
}
