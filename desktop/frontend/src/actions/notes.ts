import type { Action, ActionContext } from './types'

export const noteActions = ({ a, layout }: ActionContext): Action[] => [
  {
    id: 'show-notes',
    label: 'Show notes',
    group: 'Notes',
    run() {
      layout.showPanelView('notes')
      a.focus('note-jot')
    },
  },
]
