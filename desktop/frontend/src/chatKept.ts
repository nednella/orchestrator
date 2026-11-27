import type { ChatState } from './chatEvents'

// The chats of the threads read last, kept so that one opens again at once.
const keptThreads = 20
const kept = new Map<string, ChatState>()

export const keptChat = (id: string): ChatState | undefined => kept.get(id)

export function keepChat(id: string, state: ChatState) {
  kept.delete(id)
  kept.set(id, state)
  const oldest = kept.keys().next().value
  if (kept.size > keptThreads && oldest !== undefined) kept.delete(oldest)
}

// forgetChat drops the chat kept of a thread that is gone.
export function forgetChat(id: string) {
  kept.delete(id)
}
