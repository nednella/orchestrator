import { useSyncExternalStore } from 'react'
import { panelStateOf } from './panelTabs'
import type { PanelState } from './panelTabs'
import { readStored, writeStored } from './storage'

// Each thread's right panel, and Home's under ''.
const PANEL_KEY = 'orchestrator.panel.v2'

let panels = readStored<Record<string, unknown>>(PANEL_KEY, {})
const listeners = new Set<() => void>()

function save(next: Record<string, unknown>) {
  panels = next
  writeStored(PANEL_KEY, next)
  listeners.forEach((listener) => listener())
}

function subscribe(listener: () => void) {
  listeners.add(listener)
  return () => listeners.delete(listener)
}

export function usePanel(key: string): PanelState {
  return panelStateOf(useSyncExternalStore(subscribe, () => panels[key]))
}

// updatePanel changes the panel saved under key. A panel back at its empty state is dropped, so the saved panels hold
// only the threads that show something.
export function updatePanel(key: string, change: (st: PanelState) => PanelState) {
  const before = panelStateOf(panels[key])
  const st = change(before)
  if (st === before) return
  const { [key]: _, ...rest } = panels
  save(st.tabs.length === 0 && !st.expanded ? rest : { ...rest, [key]: st })
}

export function forgetPanel(thread: string) {
  if (!(thread in panels)) return
  const { [thread]: _, ...rest } = panels
  save(rest)
}
