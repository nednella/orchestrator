import { api } from '../api'
import type { Action, ActionContext } from './types'

export const navigateActions = ({ a, layout }: ActionContext): Action[] => [
  {
    id: 'search-threads',
    label: 'Search threads',
    group: 'Navigate',
    shortcut: { key: 'p' },
    run() {
      layout.focusPanel('sidebar')
      a.focus('thread-search')
    },
  },
  {
    id: 'open-repo',
    label: 'Open repository on GitHub',
    group: 'Projects',
    async run() {
      if (!a.project?.repo) throw 'This project has no GitHub repo'
      await api.openURL(`https://github.com/${a.project.repo}`)
    },
  },
  {
    id: 'add-project',
    label: 'Add project…',
    group: 'Projects',
    async run() {
      await a.addProject()
    },
  },
  {
    id: 'set-up-project',
    label: 'Set up project',
    group: 'Projects',
    async run() {
      await a.setUpProject()
    },
  },
  {
    id: 'toggle-sidebar',
    label: 'Toggle thread list',
    group: 'Navigate',
    shortcut: { key: 'a', unlessTyping: true },
    run: layout.toggleSidebar,
  },
  {
    id: 'toggle-panels',
    label: 'Toggle all panels',
    group: 'Navigate',
    shortcut: { key: 'b' },
    run: layout.togglePanels,
  },
  {
    id: 'panel-sidebar',
    label: 'Project home: queue and notes',
    group: 'Navigate',
    shortcut: { key: 'a', shift: true },
    run() {
      layout.showSidebarTab(a.sidebarTab)
      a.focus(a.sidebarTab === 'queue' ? 'queue-filter' : 'home')
    },
  },
  {
    id: 'panel-terminal',
    label: 'Centre panel: the active session',
    group: 'Navigate',
    run() {
      layout.closeCentre()
      layout.focusPanel('centre')
    },
  },
  {
    id: 'panel-threads',
    label: 'Left panel: threads',
    group: 'Navigate',
    shortcut: { key: 'd', shift: true },
    run: () => layout.focusPanel('sidebar'),
  },
  {
    id: 'previous-session',
    label: 'Previous session',
    group: 'Navigate',
    shortcut: { key: '[' },
    run: () => a.stepSession(-1),
  },
  {
    id: 'next-session',
    label: 'Next session',
    group: 'Navigate',
    shortcut: { key: ']' },
    run: () => a.stepSession(1),
  },
  {
    id: 'palette',
    label: 'Command palette',
    group: 'Navigate',
    shortcut: { key: 'k' },
    palette: false,
    run: () => a.setOverlay(a.overlay === 'palette' ? null : 'palette'),
  },
]
