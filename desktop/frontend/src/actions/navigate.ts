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
    id: 'add-project',
    label: 'Add project…',
    group: 'Projects',
    async run() {
      await a.addProject()
    },
  },
  {
    id: 'toggle-sidebar',
    label: 'Toggle left sidebar',
    group: 'Navigate',
    shortcut: { key: 'a', shift: true },
    run: () => layout.togglePanel('sidebar'),
  },
  {
    id: 'toggle-right',
    label: 'Toggle right panel',
    group: 'Navigate',
    shortcut: { key: 'd', shift: true },
    run: () => layout.togglePanel('right'),
  },
  {
    id: 'toggle-panel-expanded',
    label: 'Expand or restore the right panel',
    group: 'Navigate',
    run() {
      if (!layout.docked.right) throw 'The window is too narrow to expand the panel'
      layout.togglePanelExpanded()
    },
  },
  {
    id: 'show-pull-requests',
    label: 'Pull requests',
    group: 'Navigate',
    run: () => layout.showPanelView('pulls', { expanded: true }),
  },
  {
    id: 'show-issues',
    label: 'Issues',
    group: 'Navigate',
    run: () => layout.showPanelView('issues', { expanded: true }),
  },
  {
    id: 'history-back',
    label: 'Back',
    group: 'Navigate',
    shortcut: { key: '[', code: 'BracketLeft', alt: true, label: '[' },
    run: () => a.goBack(),
  },
  {
    id: 'history-forward',
    label: 'Forward',
    group: 'Navigate',
    shortcut: { key: ']', code: 'BracketRight', alt: true, label: ']' },
    run: () => a.goForward(),
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
]
