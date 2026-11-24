import type { Action, ActionContext } from './types'

export const navigateActions = ({ a, layout, current }: ActionContext): Action[] => [
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
    label: 'Toggle thread list',
    group: 'Navigate',
    shortcut: { key: 'b' },
    run: layout.toggleSidebar,
  },
  {
    id: 'toggle-right',
    label: 'Toggle right panel',
    group: 'Navigate',
    run() {
      if (!current) throw 'No session selected'
      layout.toggleRight()
    },
  },
  {
    id: 'panel-sidebar',
    label: 'Project home: queue and notes',
    group: 'Navigate',
    shortcut: { key: 'a', shift: true },
    run() {
      layout.showHome('queue')
      a.focus('home')
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
]
