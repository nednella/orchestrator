import type { Action, ActionContext } from './types'

export const noteActions = ({ a, layout }: ActionContext): Action[] => [
  {
    id: 'show-notes',
    label: 'Show notes',
    group: 'Notes',
    uiCommand: 'notes',
    run() {
      layout.showHome('notes')
      a.focus('composer')
    },
  },
]
