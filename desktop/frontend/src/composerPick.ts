// PickStep is what a key does to a composer's pick list: move the cursor to a row, or pick a row.
export type PickStep = { move: number } | { pick: number }

// pickListKey is what key does to a pick list of count rows whose cursor is at cursor, or null when the key is not
// the list's: Up and Down move, and Enter or Tab picks.
export function pickListKey(key: string, count: number, cursor: number): PickStep | null {
  if (count === 0) return null
  if (key === 'ArrowDown') return { move: Math.min(cursor + 1, count - 1) }
  if (key === 'ArrowUp') return { move: Math.max(cursor - 1, 0) }
  if (key === 'Enter' || key === 'Tab') return { pick: Math.max(cursor, 0) }
  return null
}
