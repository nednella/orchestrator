import type { ComponentType } from 'react'
import { useAgentos, useAgentosSelector } from './AgentosContext'
import { IssuesView } from './components/IssuesView'
import { NotesView } from './components/NotesView'
import { ThreadDetailsView } from './components/ThreadDetailsView'
import type { Issue, Note, Session } from './types'

export type PanelContext = { session: Session | undefined; notes: Note[]; issues: Issue[] }

export type PanelView = {
  id: string
  label: string
  count?(ctx: PanelContext): number
  available(ctx: PanelContext): boolean
  Component: ComponentType<{ ctx: PanelContext }>
}

// The right panel's views, in tab order. A new view is one entry here and its component.
export const PANEL_VIEWS: PanelView[] = [
  { id: 'notes', label: 'Notes', count: (ctx) => ctx.notes.filter((n) => !n.archived).length, available: () => true, Component: NotesView },
  { id: 'issues', label: 'Issues', count: (ctx) => ctx.issues.length, available: () => true, Component: IssuesView },
  { id: 'details', label: 'Details', available: (ctx) => ctx.session !== undefined, Component: ThreadDetailsView },
]

export function usePanelContext(): PanelContext {
  const { notes, issues } = useAgentos('notes', 'issues')
  const session = useAgentosSelector((a) => a.sessions.find((s) => s.id === a.selectedId))
  return { session, notes, issues }
}

export const shownPanelViews = (ctx: PanelContext) => PANEL_VIEWS.filter((view) => view.available(ctx))

// The view the panel shows: the one picked last in the project while it is available, else a thread's details, else notes.
export function panelViewFor(picked: string, ctx: PanelContext): PanelView {
  const shown = shownPanelViews(ctx)
  return shown.find((view) => view.id === picked) ?? shown.find((view) => view.id === (ctx.session ? 'details' : 'notes')) ?? shown[0]
}
