import type { Layout } from './LayoutContext'
import type { Orchestrator } from './OrchestratorContext'
import { focusTerminal, startTerminal, termsOf } from './projectTerminalOps'
import type { Placement } from './projectTerminalOps'

type TerminalHost = Pick<Orchestrator, 'project' | 'selectedId' | 'shells'>

// showTerminal opens the right panel on the Terminal surface and gives the keys to the terminal in it: a new terminal
// in a tab of its own when asked for, or when the project has none.
export async function showTerminal(a: TerminalHost, layout: Layout, fresh: boolean) {
  if (!a.project) throw 'The terminal is unavailable'
  layout.showPanelView('terminal')
  const terms = termsOf(a.project.key, a.shells)
  if (fresh || terms.groups.length === 0) await startTerminal(a.project.key, a.selectedId ?? '', a.shells, 'tab')
  else focusTerminal(terms)
}

// splitTerminal puts a new terminal beside the active one, in the Terminal surface.
export async function splitTerminal(a: TerminalHost, layout: Layout, placement: Exclude<Placement, 'tab'>) {
  if (!a.project) throw 'The terminal is unavailable'
  layout.showPanelView('terminal')
  await startTerminal(a.project.key, a.selectedId ?? '', a.shells, placement)
}
