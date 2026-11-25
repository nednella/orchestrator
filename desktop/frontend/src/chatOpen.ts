// Which folds and tool calls of a chat the reader has opened, kept outside the rows so that they stay open when the
// list lets go of a row or a turn folds. Keys are a turn's or a call's id, never a row's key.
const keptThreads = 20
const opened = new Map<string, Set<string>>()

export function isOpened(thread: string, key: string): boolean {
  return opened.get(thread)?.has(key) ?? false
}

// setOpened marks a thread as the newest one touched, and drops the oldest past the limit.
export function setOpened(thread: string, key: string, open: boolean) {
  const keys = opened.get(thread) ?? new Set<string>()
  opened.delete(thread)
  if (open) keys.add(key)
  else keys.delete(key)
  opened.set(thread, keys)
  const oldest = opened.keys().next().value
  if (opened.size > keptThreads && oldest !== undefined) opened.delete(oldest)
}
