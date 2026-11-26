import type { Action, ActionContext } from './types'

export const navigateActions = ({ a, layout }: ActionContext): Action[] => [
  {
    id: 'search-threads',
    label: 'Search threads',
    shortcut: { key: 'p' },
    run() {
      layout.focusPanel('sidebar')
      a.focus('thread-search')
    },
  },
  {
    id: 'add-project',
    label: 'Add project…',
    run: () => a.setOverlay('add-project'),
  },
  {
    id: 'toggle-sidebar',
    label: 'Toggle left sidebar',
    shortcut: { key: 'a', shift: true },
    run: () => layout.togglePanel('sidebar'),
  },
  {
    id: 'toggle-right',
    label: 'Toggle right panel',
    shortcut: { key: 'd', shift: true },
    run: () => layout.togglePanel('right'),
  },
  {
    id: 'history-back',
    label: 'Back',
    shortcut: { key: '[', code: 'BracketLeft', alt: true, label: '[' },
    run: () => a.goBack(),
  },
  {
    id: 'history-forward',
    label: 'Forward',
    shortcut: { key: ']', code: 'BracketRight', alt: true, label: ']' },
    run: () => a.goForward(),
  },
  {
    id: 'previous-session',
    label: 'Previous session',
    shortcut: { key: '[' },
    run: () => a.stepSession(-1),
  },
  {
    id: 'next-session',
    label: 'Next session',
    shortcut: { key: ']' },
    run: () => a.stepSession(1),
  },
]
