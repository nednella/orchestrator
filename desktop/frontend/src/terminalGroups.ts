// A group is one or more terminals shown side by side; active is the pane that takes the keys. A thread's terminals
// are its groups in list order, and active is the id of the group the drawer shows.
export type TermGroup = { id: string; panes: string[]; active: string }
export type ThreadTerms = { groups: TermGroup[]; active: string }

export const MAX_PANES = 4
// Each thread's terminals by thread id, '' for the project's shells.
export const TERMINALS_KEY = 'agentos.terminals.v1'

// Go numbers shells from the lowest free number, so a shell id can come back: a group id is never a shell id.
function nextGroupId(groups: TermGroup[]): string {
  const used = new Set(groups.map((g) => g.id))
  let n = 1
  while (used.has(`g${n}`)) n++
  return `g${n}`
}

// kept is what stays of order once gone has gone: the one before gone that stayed, else the one after, else the first.
function nearestKept(order: string[], gone: string, kept: string[]): string {
  const at = order.indexOf(gone)
  for (let i = at - 1; i >= 0; i--) if (kept.includes(order[i])) return order[i]
  for (let i = at + 1; i < order.length; i++) if (kept.includes(order[i])) return order[i]
  return kept[0] ?? ''
}

// reconcile is the saved terminals as the shells now running allow: a pane whose shell is gone drops, a group left
// empty goes, and a shell in no group gets a group of its own at the end.
export function reconcile(saved: ThreadTerms | undefined, shells: string[]): ThreadTerms {
  const live = new Set(shells)
  const placed = new Set<string>()
  const before = Array.isArray(saved?.groups) ? saved.groups.filter((g) => Array.isArray(g.panes)) : []
  const groups: TermGroup[] = []
  for (const g of before) {
    const panes = g.panes.filter((p) => live.has(p) && !placed.has(p))
    panes.forEach((p) => placed.add(p))
    if (panes.length > 0) groups.push({ id: g.id, panes, active: panes.includes(g.active) ? g.active : nearestKept(g.panes, g.active, panes) })
  }
  for (const shell of shells) if (!placed.has(shell)) groups.push({ id: nextGroupId(groups), panes: [shell], active: shell })
  const ids = groups.map((g) => g.id)
  const wanted = saved?.active ?? ''
  return { groups, active: ids.includes(wanted) ? wanted : nearestKept(before.map((g) => g.id), wanted, ids) }
}

// without takes shell out of every group, and drops a group it leaves empty.
function without(t: ThreadTerms, shell: string): TermGroup[] {
  return t.groups.flatMap((g) => {
    if (!g.panes.includes(shell)) return [g]
    const panes = g.panes.filter((p) => p !== shell)
    return panes.length > 0 ? [{ ...g, panes, active: g.active === shell ? nearestKept(g.panes, shell, panes) : g.active }] : []
  })
}

// addGroup puts a new terminal in a group of its own at the end, and shows it.
export function addGroup(t: ThreadTerms, shell: string): ThreadTerms {
  const groups = without(t, shell)
  const id = nextGroupId(groups)
  return { groups: [...groups, { id, panes: [shell], active: shell }], active: id }
}

// split puts a new terminal beside the active pane of the shown group and makes it the active pane. A full group
// stays as it is.
export function split(t: ThreadTerms, shell: string): ThreadTerms {
  const group = t.groups.find((g) => g.id === t.active)
  if (!group) return addGroup(t, shell)
  if (group.panes.length >= MAX_PANES) return t
  const groups = without(t, shell).map((g) => {
    if (g.id !== group.id) return g
    const panes = [...g.panes]
    panes.splice(panes.indexOf(g.active) + 1, 0, shell)
    return { ...g, panes, active: shell }
  })
  return { groups, active: group.id }
}

// closePane takes the terminal out; the last pane of a group closes the group, and when that group was shown, the
// group above it shows.
export function closePane(t: ThreadTerms, shell: string): ThreadTerms {
  const groups = without(t, shell)
  const ids = groups.map((g) => g.id)
  return { groups, active: ids.includes(t.active) ? t.active : nearestKept(t.groups.map((g) => g.id), t.active, ids) }
}

// select shows the group that holds the terminal, with the terminal as its active pane.
export function select(t: ThreadTerms, shell: string): ThreadTerms {
  const group = t.groups.find((g) => g.panes.includes(shell))
  if (!group) return t
  return { groups: t.groups.map((g) => (g === group ? { ...g, active: shell } : g)), active: group.id }
}

// activeShell is the pane that takes the keys in the shown group, or ''.
export function activeShell(t: ThreadTerms): string {
  return t.groups.find((g) => g.id === t.active)?.active ?? ''
}

// terminalLabel names a terminal by the process in front in it; n is its place in the list, from 1.
export function terminalLabel(command: string, n: number): string {
  return command || `Terminal ${n}`
}

export function showsList(t: ThreadTerms): boolean {
  return t.groups.reduce((n, g) => n + g.panes.length, 0) > 1
}

export function showsGroupHeaders(t: ThreadTerms): boolean {
  return t.groups.some((g) => g.panes.length > 1)
}

// keepRecent puts id first in order, newest first, and keeps the max newest.
export function keepRecent(order: string[], id: string, max = 10): string[] {
  return [id, ...order.filter((x) => x !== id)].slice(0, max)
}
