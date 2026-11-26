import type { TimelineItem } from './chatTimeline'

// Recall is the prompt the composer holds from the arrow keys, and where it sits in the thread's history.
export type Recall = { thread: string; index: number; text: string } | null

type Step = { recall: Recall; text: string }

// historyOf is the owner's own messages in the thread, oldest first, with equal ones in a row counted once.
export function historyOf(items: TimelineItem[]): string[] {
  const prompts: string[] = []
  for (const item of items) {
    if (item.kind !== 'user' || item.from || item.text.trim() === '') continue
    if (prompts[prompts.length - 1] !== item.text) prompts.push(item.text)
  }
  return prompts
}

type RecallKey = {
  key: 'ArrowUp' | 'ArrowDown'
  modifier: boolean
  composing: boolean
  pending: boolean
  attachments: number
  menuOpen: boolean
  value: string
  recall: Recall
  thread: string
  onFirstLine: boolean
  onLastLine: boolean
}

// canRecall is whether an arrow key steps through history rather than moving the caret. The composer must be empty or
// hold exactly the last recall, so what the owner typed is never replaced.
export function canRecall(k: RecallKey): boolean {
  if (k.modifier || k.composing || k.pending || k.attachments > 0 || k.menuOpen) return false
  if (k.value !== '' && !(k.recall !== null && k.recall.thread === k.thread && k.recall.text === k.value)) return false
  return k.key === 'ArrowUp' ? k.onFirstLine : k.onLastLine
}

export function stepUp(history: string[], recall: Recall, thread: string): Step | null {
  if (history.length === 0) return null
  const active = recall !== null && recall.thread === thread
  const index = active ? Math.min(recall.index, history.length) - 1 : history.length - 1
  if (index < 0) return null
  return { recall: { thread, index, text: history[index] }, text: history[index] }
}

export function stepDown(history: string[], recall: Recall, thread: string): Step | null {
  if (recall === null || recall.thread !== thread) return null
  const index = recall.index + 1
  if (index >= history.length) return { recall: null, text: '' }
  return { recall: { thread, index, text: history[index] }, text: history[index] }
}
