import { useRef } from 'react'
import type { RefObject } from 'react'
import type { ComposerInput } from './components/ComposerEditor'
import type { ComposerDraft } from './composerMarkdown'
import { draftOf, fromMarkdown } from './composerMarkdown'
import type { TimelineItem } from './chatTimeline'
import { canRecall, historyOf, stepDown, stepUp } from './promptHistory'
import type { Recall } from './promptHistory'

type PromptHistoryOptions = {
  input: RefObject<ComposerInput | null>
  thread: string
  items: TimelineItem[]
  value: ComposerDraft
  pending: boolean
  attachments: number
  menuOpen: boolean
  onRecall(draft: ComposerDraft): void
}

// usePromptHistory lets the arrow keys step through the owner's sent prompts, as a terminal does. The history is read
// when a key is pressed, since the timeline changes with each streamed piece.
export function usePromptHistory({ input, thread, items, value, pending, attachments, menuOpen, onRecall }: PromptHistoryOptions) {
  const recall = useRef<Recall>(null)
  const latest = useRef({ items, value })
  latest.current = { items, value }

  const onKeyDown = (e: KeyboardEvent) => {
    if (e.key !== 'ArrowUp' && e.key !== 'ArrowDown') return
    const editor = input.current
    if (!editor) return
    const { items, value } = latest.current
    const held = value.text
    if (held === '') recall.current = null
    const ok = canRecall({
      key: e.key,
      modifier: e.shiftKey || e.altKey || e.ctrlKey || e.metaKey,
      composing: e.isComposing || e.keyCode === 229,
      pending,
      attachments,
      menuOpen,
      value: held,
      recall: recall.current,
      thread,
      onFirstLine: editor.caretOnFirstLine(),
      onLastLine: editor.caretOnLastLine(),
    })
    if (!ok) return
    const history = historyOf(items)
    const step = e.key === 'ArrowUp' ? stepUp(history, recall.current, thread) : stepDown(history, recall.current, thread)
    if (!step) return
    e.preventDefault()
    const draft = draftOf(fromMarkdown(step.text))
    recall.current = step.recall && { ...step.recall, text: draft.text }
    onRecall(draft)
    editor.focus('end')
  }

  // An edit of the recalled text leaves history; the key then moves the caret again.
  const edited = (text: string) => {
    if (recall.current && recall.current.text !== text) recall.current = null
  }

  return { onKeyDown, edited }
}
