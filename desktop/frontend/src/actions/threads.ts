import type { Thread } from '../types'
import type { Action, ActionContext } from './types'

export const OPEN_IN_EDITOR_LABEL = 'Open in VS Code'

const describe = (s: Thread) => `${s.n} ${s.title}`

export const threadActions = ({ a, layout, current }: ActionContext): Action[] => [
  {
    id: 'new-thread',
    label: 'New thread',
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
    shortcut: { key: 'n' },
    run() {
      layout.showHome()
      a.setComposerMode('thread')
      a.setComposerAccess('read')
      a.focus('composer')
    },
  },
  {
    id: 'detach-thread',
    label: current ? `Detach thread ${describe(current)}` : 'Detach thread',
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
    async run() {
      if (!current) throw 'Select a thread first'
      await a.openInEditor(current.id)
    },
  },
  {
    id: 'next-attention',
    label: 'Next that needs you',
    shortcut: { key: 'e' },
    run() {
      a.nextAttention()
    },
  },
]
