// A group is one or more terminals shown together, in a row or in a column; active is the pane that takes the keys.
// A project's terminals are its groups in tab order, and active is the id of the group the panel shows.
export type SplitDir = 'row' | 'column'
export type TermGroup = { id: string; panes: string[]; active: string; dir: SplitDir }
export type ProjectTerms = { groups: TermGroup[]; active: string }

export const MAX_PANES = 4
// Each project's terminals by project key.
export const TERMINALS_KEY = 'orchestrator.terminals.v2'

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
export function reconcile(saved: ProjectTerms | undefined, shells: string[]): ProjectTerms {
  const live = new Set(shells)
  const placed = new Set<string>()
  const before = Array.isArray(saved?.groups) ? saved.groups.filter((g) => Array.isArray(g.panes)) : []
  const groups: TermGroup[] = []
  for (const g of before) {
    const panes = g.panes.filter((p) => live.has(p) && !placed.has(p))
    panes.forEach((p) => placed.add(p))
    if (panes.length > 0) {
      groups.push({ id: g.id, panes, active: panes.includes(g.active) ? g.active : nearestKept(g.panes, g.active, panes), dir: g.dir === 'column' ? 'column' : 'row' })
    }
  }
  for (const shell of shells) {
    if (placed.has(shell)) continue
    placed.add(shell)
    groups.push({ id: nextGroupId(groups), panes: [shell], active: shell, dir: 'row' })
  }
  const ids = groups.map((g) => g.id)
  const wanted = saved?.active ?? ''
  return { groups, active: ids.includes(wanted) ? wanted : nearestKept(before.map((g) => g.id), wanted, ids) }
}

function panesOf(saved: ProjectTerms | undefined): string[] {
  return Array.isArray(saved?.groups) ? saved.groups.flatMap((g) => (Array.isArray(g.panes) ? g.panes : [])) : []
}

// withShells is the saved terminals, with each shell in no group given a group of its own at the end; no pane goes.
export function withShells(saved: ProjectTerms | undefined, shells: string[]): ProjectTerms {
  return reconcile(saved, [...panesOf(saved), ...shells])
}

// prune drops the panes of the project's shells that are not in keep. Shell ids start with their project's key, and the
// panes of other projects' shells stay.
export function prune(saved: ProjectTerms | undefined, project: string, keep: Set<string>): ProjectTerms {
  return reconcile(saved, panesOf(saved).filter((p) => !p.startsWith(`${project}/`) || keep.has(p)))
}

// without takes shell out of every group, and drops a group it leaves empty.
function without(t: ProjectTerms, shell: string): TermGroup[] {
  return t.groups.flatMap((g) => {
    if (!g.panes.includes(shell)) return [g]
    const panes = g.panes.filter((p) => p !== shell)
    return panes.length > 0 ? [{ ...g, panes, active: g.active === shell ? nearestKept(g.panes, shell, panes) : g.active }] : []
  })
}

// addGroup puts a new terminal in a group of its own at the end, and shows it.
export function addGroup(t: ProjectTerms, shell: string): ProjectTerms {
  const groups = without(t, shell)
  const id = nextGroupId(groups)
  return { groups: [...groups, { id, panes: [shell], active: shell, dir: 'row' }], active: id }
}

// split puts a new terminal beside the active pane of the shown group and makes it the active pane. The direction is
// the whole group's, so a split the other way turns the group. A full group stays as it is.
export function split(t: ProjectTerms, shell: string, dir: SplitDir = 'row'): ProjectTerms {
  const group = shownGroup(t)
  if (!group) return addGroup(t, shell)
  if (group.panes.length >= MAX_PANES) return t
  const groups = without(t, shell).map((g) => {
    if (g.id !== group.id) return g
    const panes = [...g.panes]
    panes.splice(panes.indexOf(g.active) + 1, 0, shell)
    return { ...g, panes, active: shell, dir }
  })
  return { groups, active: group.id }
}

// closePane takes the terminal out; the last pane of a group closes the group, and when that group was shown, its
// neighbour shows.
export function closePane(t: ProjectTerms, shell: string): ProjectTerms {
  const groups = without(t, shell)
  const ids = groups.map((g) => g.id)
  return { groups, active: ids.includes(t.active) ? t.active : nearestKept(t.groups.map((g) => g.id), t.active, ids) }
}

// select shows the group that holds the terminal, with the terminal as its active pane.
export function select(t: ProjectTerms, shell: string): ProjectTerms {
  const group = t.groups.find((g) => g.panes.includes(shell))
  if (!group) return t
  return { groups: t.groups.map((g) => (g === group ? { ...g, active: shell } : g)), active: group.id }
}

// selectGroup shows the group, with the pane it had as the active one.
export function selectGroup(t: ProjectTerms, group: string): ProjectTerms {
  return t.groups.some((g) => g.id === group) && t.active !== group ? { ...t, active: group } : t
}

// shownGroup is the group the panel shows.
export function shownGroup(t: ProjectTerms): TermGroup | undefined {
  return t.groups.find((g) => g.id === t.active)
}

// activeShell is the pane that takes the keys in the shown group, or ''.
export function activeShell(t: ProjectTerms): string {
  return shownGroup(t)?.active ?? ''
}

// terminalNumber is the shell's number: the end of its id, 1 for the project's first shell, which has none.
export function terminalNumber(id: string): number {
  return Number(/-(\d+)$/.exec(id)?.[1] ?? 1)
}

// terminalLabel names a terminal by the process in front in it, else by its number.
export function terminalLabel(command: string, n: number): string {
  return command || `Terminal ${n}`
}

// folderName is the last part of a folder's path.
export function folderName(path: string): string {
  return path.replace(/\/+$/, '').split('/').pop() || path
}

// showsList says whether the group lists its terminals at its side: from two, in place of the buttons over a lone one.
export function showsList(group: TermGroup): boolean {
  return group.panes.length > 1
}
