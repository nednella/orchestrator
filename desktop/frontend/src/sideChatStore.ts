import { useSyncExternalStore } from 'react'

// What the app keeps of side chats while it runs: the side chats added to each thread's next message, and why each
// thread whose chat is loaded cannot start one.
const attached = new Map<string, string[]>()
const reasons = new Map<string, string>()
const listeners = new Set<() => void>()
const none: string[] = []

function changed() {
  listeners.forEach((listener) => listener())
}

function subscribe(listener: () => void) {
  listeners.add(listener)
  return () => listeners.delete(listener)
}

// forgetSideChatState drops what is kept of a thread that is gone.
export function forgetSideChatState(thread: string) {
  const had = attached.delete(thread) || reasons.delete(thread)
  if (had) changed()
}

// attachedTo are the side chats added to the thread's next message, oldest first.
export const attachedTo = (thread: string): string[] => attached.get(thread) ?? none

export function useAttached(thread: string): string[] {
  return useSyncExternalStore(subscribe, () => attachedTo(thread))
}

// setAttached gives the thread's next message the side chats.
export function setAttached(thread: string, sideChats: string[]) {
  if (sideChats.length === 0) attached.delete(thread)
  else attached.set(thread, sideChats)
  changed()
}

export function attach(thread: string, sideChat: string) {
  const now = attachedTo(thread)
  if (!now.includes(sideChat)) setAttached(thread, [...now, sideChat])
}

export const detach = (thread: string, sideChat: string) => setAttached(thread, attachedTo(thread).filter((id) => id !== sideChat))

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
