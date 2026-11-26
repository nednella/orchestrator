import type { Shortcut } from './types'

const KEY_LABELS: Record<string, string> = { arrowleft: '←', arrowright: '→', arrowup: '↑', arrowdown: '↓' }

export function formatShortcut(shortcut: Shortcut): string {
  const key = shortcut.label ?? KEY_LABELS[shortcut.key] ?? shortcut.key.toUpperCase()
  return `⌘${shortcut.alt ? '⌥' : ''}${shortcut.shift ? '⇧' : ''}${key}`
}

// withShortcut is the label with the shortcut that runs it, as a tooltip shows it.
export function withShortcut(label: string, shortcut: Shortcut | undefined): string {
  return shortcut ? `${label} (${formatShortcut(shortcut)})` : label
}

export function matchShortcut(e: KeyboardEvent, shortcut: Shortcut): boolean {
  if (!e.metaKey || e.ctrlKey || e.altKey !== Boolean(shortcut.alt)) return false
  if (!shortcut.anyShift && e.shiftKey !== Boolean(shortcut.shift)) return false
  if (shortcut.code) return e.code === shortcut.code
  return [shortcut.key, ...(shortcut.aliases ?? [])].includes(e.key.toLowerCase())
}

export const LIST_KEYS: { keys: string; summary: string }[] = [
  { keys: 'W / S or ↑ / ↓', summary: 'Move the row cursor in a focused list' },
  { keys: 'Enter', summary: 'Act on the row: issues opens the issue, sessions opens, notes edits' },
  { keys: 'Menu key or Shift F10', summary: 'Open the menu of a thread card' },
  { keys: 'A / D or ← / →', summary: 'Fold or unfold the Sleeping threads' },
  { keys: 'Space', summary: 'Fold or unfold the section under the cursor' },
  { keys: 'P / E in notes', summary: 'Pin or archive the note under the cursor' },
  { keys: 'A S P I N E B', summary: 'Open a right panel view: Agents, Servers, Pull requests, Issues, Notes, Evidence or Browser' },
  { keys: 'Esc', summary: 'Close a panel or overlay. In a thread, Esc stops the agent’s turn' },
]
