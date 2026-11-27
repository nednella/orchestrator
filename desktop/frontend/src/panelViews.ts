import { CircleDot, Files, GitPullRequestArrow, Globe, Image, MessagesSquare, Newspaper, SquareTerminal, StickyNote } from 'lucide-react'
import type { LucideIcon } from 'lucide-react'
import type { ComponentType } from 'react'
import { BrowserView } from './components/BrowserView'
import { DigestView } from './components/DigestView'
import { EvidenceView } from './components/EvidenceView'
import { FilesView } from './components/FilesView'
import { IssuesView } from './components/IssuesView'
import { NotesView } from './components/NotesView'
import { PullRequestsView } from './components/PullRequestsView'
import { TerminalView } from './components/TerminalView'
import { PANEL_KEYS, SIDE_CHAT_KEY } from './panelTabs'
import type { PanelRowTarget, PanelViewId } from './panelTabs'
import type { Thread } from './types'

// thread is the open thread, none on Home.
export type PanelViewProps = { thread: Thread | undefined }

// thread says the view shows one thread's things: Home lists it, but cannot open it.
export type PanelView = { id: PanelViewId; label: string; icon: LucideIcon; thread: boolean; Component: ComponentType<PanelViewProps> }

const VIEWS: Record<PanelViewId, PanelView> = {
  notes: { id: 'notes', label: 'Notes', icon: StickyNote, thread: false, Component: NotesView },
  files: { id: 'files', label: 'Files', icon: Files, thread: false, Component: FilesView },
  issues: { id: 'issues', label: 'Issues', icon: CircleDot, thread: false, Component: IssuesView },
  pulls: { id: 'pulls', label: 'Pull requests', icon: GitPullRequestArrow, thread: false, Component: PullRequestsView },
  browser: { id: 'browser', label: 'Browser', icon: Globe, thread: true, Component: BrowserView },
  media: { id: 'media', label: 'Media', icon: Image, thread: true, Component: EvidenceView },
  news: { id: 'news', label: 'News', icon: Newspaper, thread: false, Component: DigestView },
  terminal: { id: 'terminal', label: 'Terminal', icon: SquareTerminal, thread: false, Component: TerminalView },
}

export const panelView = (id: PanelViewId): PanelView | undefined => VIEWS[id]

// A row of the panel's list, and what it does.
export type PanelRow = { label: string; icon: LucideIcon; key: string; target: PanelRowTarget }
export type PanelGroup = { heading: string; rows: PanelRow[] }

const rowOf = (id: PanelViewId): PanelRow => ({ label: VIEWS[id].label, icon: VIEWS[id].icon, key: PANEL_KEYS[id], target: id })

export const PANEL_GROUPS: PanelGroup[] = [
  { heading: 'This project', rows: [rowOf('notes'), rowOf('files'), rowOf('issues'), rowOf('pulls'), rowOf('terminal')] },
  { heading: 'This thread', rows: [rowOf('browser'), rowOf('media'), { label: 'Side chat', icon: MessagesSquare, key: SIDE_CHAT_KEY, target: 'sideChat' }] },
  { heading: 'Everywhere', rows: [rowOf('news')] },
]
