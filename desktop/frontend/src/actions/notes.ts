import type { Action, ActionContext } from './types'

export const noteActions = ({ a, layout }: ActionContext): Action[] => [
  {
    id: 'note',
    label: 'New note',
    group: 'Notes',
    run() {
      layout.showHome('composer')
      a.setComposerMode('note')
      a.focus('composer')
    },
  },
  {
    id: 'show-notes',
    label: 'Show notes',
    group: 'Notes',
    uiCommand: 'notes',
    run() {
      layout.showHome('notes')
      a.setComposerMode('note')
      a.focus('composer')
    },
  },
]
