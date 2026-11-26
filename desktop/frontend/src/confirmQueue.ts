// ConfirmRequest is one question the confirm modal asks: its title, the paragraphs and list under it, an optional
// box with the hint under it or why it is off, and the confirm button's label and tone.
export type ConfirmRequest = {
  id: number
  title: string
  body?: string[]
  list?: string[]
  confirm: string
  tone: 'danger' | 'primary'
  checkbox?: { label: string; hint?: string; checked: boolean; disabled?: string }
}

// The confirms asked for, in the order they came. Only the first shows; the others wait for it.
export type ConfirmQueue = readonly ConfirmRequest[]

export function enqueue(q: ConfirmQueue, r: ConfirmRequest): ConfirmQueue {
  return [...q, r]
}

// settle takes the answered confirm out, so the next one shows.
export function settle(q: ConfirmQueue, id: number): ConfirmQueue {
  return q.filter((r) => r.id !== id)
}

export function current(q: ConfirmQueue): ConfirmRequest | undefined {
  return q[0]
}

// check ticks or unticks the box of the confirm id.
export function check(q: ConfirmQueue, id: number, checked: boolean): ConfirmQueue {
  if (!q.some((r) => r.id === id && r.checkbox)) return q
  return q.map((r) => (r.id === id && r.checkbox ? { ...r, checkbox: { ...r.checkbox, checked } } : r))
}

// answerFor is the answer a key gives the confirm shown: Esc cancels. Enter acts on the focused button, the confirm at
// first.
export function answerFor(key: string): false | undefined {
  return key === 'Escape' ? false : undefined
}

// initialFocus is the button a confirm focuses when it shows: Cancel for a danger one, so Enter never deletes.
export function initialFocus(tone: ConfirmRequest['tone']): 'cancel' | 'confirm' {
  return tone === 'danger' ? 'cancel' : 'confirm'
}
