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
