import { useOrchestrator } from '../OrchestratorContext'
import type { Orchestrator } from '../OrchestratorContext'
import { useLayout } from '../LayoutContext'
import type { Layout } from '../LayoutContext'
import { modelActions } from './models'
import { navigateActions } from './navigate'
import { queueActions } from './queue'
import { threadActions } from './threads'
import { shellActions } from './shell'
import type { Action } from './types'
import { viewActions } from './views'

export type { Action, Shortcut } from './types'
export { formatShortcut, LIST_KEYS, matchShortcut, withShortcut } from './shortcut'

export function useActions(): Action[] {
  return actionsFor(useOrchestrator(), useLayout())
}

export function actionsFor(a: Orchestrator, layout: Layout): Action[] {
  const context = { a, layout, current: a.threads.find((s) => s.id === a.selectedId) }
  return [
    ...threadActions(context),
    ...navigateActions(context),
    ...queueActions(context),
    ...modelActions(),
    ...viewActions(context),
    ...shellActions(context),
  ]
}
