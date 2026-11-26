import type { Session } from '../types'
import type { Action, ActionContext } from './types'

export const OPEN_IN_EDITOR_LABEL = 'Open in VS Code'

const describe = (s: Session) => `${s.n} ${s.title}`

export const sessionActions = ({ a, layout, current }: ActionContext): Action[] => [
  {
    id: 'new-session',
    label: 'New thread',
    group: 'Sessions',
    shortcut: { key: 'm' },
    run() {
      layout.showHome()
      a.setComposerMode('thread')
      a.setComposerAccess(a.settings.threadAccess || 'supervised')
      a.focus('composer')
    },
  },
  {
    id: 'new-chat',
    label: 'New read-only thread',
    group: 'Sessions',
    shortcut: { key: 'n' },
    run() {
      layout.showHome()
      a.setComposerMode('thread')
      a.setComposerAccess('read')
      a.focus('composer')
    },
  },
  {
    id: 'detach-session',
    label: current ? `Detach session ${describe(current)}` : 'Detach session',
    group: 'Sessions',
    shortcut: { key: 'w' },
    run() {
      if (!current) return
      a.detach()
      return `Detached ${describe(current)}`
    },
  },
  {
    id: 'open-in-editor',
    label: OPEN_IN_EDITOR_LABEL,
    group: 'Sessions',
    async run() {
      if (!current) throw 'Select a thread first'
      await a.openInEditor(current.id)
    },
  },
  {
    id: 'next-attention',
    label: 'Next that needs you',
    group: 'Sessions',
    shortcut: { key: 'e' },
    run() {
      a.nextAttention()
    },
  },
]
