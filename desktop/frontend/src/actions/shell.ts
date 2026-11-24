import type { Action, ActionContext } from './types'

export const shellActions = ({ layout }: ActionContext): Action[] => [
  {
    id: 'toggle-shell',
    label: 'Toggle bottom panel (shell)',
    group: 'Navigate',
    shortcut: { key: 's', shift: true },
    run: () => layout.togglePanel('shell'),
  },
]
