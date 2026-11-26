// The message column's width, in rem, is the app's: every thread shows its chat at the width last dragged to.
export const COLUMN_KEY = 'orchestrator.chatColumn'
export const COLUMN_DEFAULT_REM = 46

// A drag under the default still moves the column by this share of the pointer, up to MAX_GIVE_REM, and the column
// settles back to the default on release.
const GIVE = 0.2
const MAX_GIVE_REM = 2

// clampColumn keeps a width between the default and the chat pane's full width.
export function clampColumn(rem: number, fullRem: number): number {
  return Math.max(COLUMN_DEFAULT_REM, Math.min(rem, fullRem))
}

// dragColumn is the width a drag shows: the clamped width, less a little give for one dragged under the default.
export function dragColumn(rem: number, fullRem: number): number {
  const under = COLUMN_DEFAULT_REM - rem
  return under > 0 ? COLUMN_DEFAULT_REM - Math.min(under * GIVE, MAX_GIVE_REM) : clampColumn(rem, fullRem)
}

// storedColumn is the width the store holds, or the default for anything that is not a width.
export function storedColumn(value: unknown): number {
  return typeof value === 'number' && Number.isFinite(value) && value >= COLUMN_DEFAULT_REM ? value : COLUMN_DEFAULT_REM
}

export const rootRemPx = () => parseFloat(getComputedStyle(document.documentElement).fontSize)
