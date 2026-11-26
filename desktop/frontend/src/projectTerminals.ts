import { useSyncExternalStore } from 'react'
import { readStored, writeStored } from './storage'
import { keepSessions } from './terminal/session'
import { prune, TERMINALS_KEY, withShells } from './terminalGroups'
import type { ProjectTerms } from './terminalGroups'
import { START_GRACE_MS, stillStarted } from './startedShells'
import type { Started } from './startedShells'
import type { Shell } from './types'

let terms = readStored<Record<string, ProjectTerms>>(TERMINALS_KEY, {})
// live are the current project's shells as Go last announced them; started are shells whose start returned before
// their first announce, which an announce sent before the start can come ahead of.
let live: Shell[] = []
let project = ''
let started: Started = new Map()
const listeners = new Set<() => void>()

function subscribe(listener: () => void) {
  listeners.add(listener)
  return () => listeners.delete(listener)
}

const changed = () => listeners.forEach((listener) => listener())

function save(next: Record<string, ProjectTerms>) {
  terms = next
  writeStored(TERMINALS_KEY, next)
}

export function useSavedTerms(key: string): ProjectTerms | undefined {
  return useSyncExternalStore(subscribe, () => terms[key])
}

export function savedTerms(key: string): ProjectTerms | undefined {
  return terms[key]
}

// changeTerms changes the current project's terminals and saves them. Only an announce of the shells takes a pane out.
export function changeTerms(change: (t: ProjectTerms) => ProjectTerms) {
  const { [project]: before, ...rest } = terms
  const next = change(withShells(before, [...live.map((s) => s.id), ...started.keys()]))
  save(next.groups.length > 0 ? { ...rest, [project]: next } : rest)
  changed()
}

// shellStarted keeps a new shell's pane until Go announces the shell, or the grace passes.
export function shellStarted(id: string) {
  if (live.some((s) => s.id === id)) return
  started = new Map(started).set(id, { project, at: Date.now() })
  setTimeout(() => settle(), START_GRACE_MS + 50)
}

// shellClosed forgets a started shell closed before Go announced it.
export function shellClosed(id: string) {
  if (!started.has(id)) return
  started = new Map(started)
  started.delete(id)
}

// startedShells are the shells started that Go has not announced yet.
export function startedShells(): string[] {
  return [...started.keys()]
}

// useStartedShells re-renders when the started shells change, as when the grace ends one.
export function useStartedShells(): string {
  return useSyncExternalStore(subscribe, () => startedShells().join())
}

function settle() {
  const left = stillStarted(started, project, new Set(live.map((s) => s.id)), Date.now())
  if (left.size === started.size) return
  started = left
  shellsChanged(project, live)
}

// shellsChanged takes the project's shells as Go announces them, takes the panes of the gone ones out, and lets the
// terminals of every other shell go; the shells themselves run on in tmux.
export function shellsChanged(key: string, shells: Shell[]) {
  project = key
  live = shells
  started = stillStarted(started, key, new Set(shells.map((s) => s.id)), Date.now())
  const keep = new Set([...shells.map((s) => s.id), ...started.keys()])
  const saved = terms[key]
  if (saved) {
    const pruned = prune(saved, key, keep)
    const { [key]: _, ...rest } = terms
    if (JSON.stringify(pruned) !== JSON.stringify(saved)) save(pruned.groups.length > 0 ? { ...rest, [key]: pruned } : rest)
  }
  keepSessions(keep)
  changed()
}
