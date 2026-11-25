import type { Session } from '../types'
import type { Action, ActionContext } from './types'

const includes = (text: string, partial: string) => text.toLowerCase().includes(partial.toLowerCase())

function findSession(sessions: Session[], query: string): Session {
  const text = query.trim().replace(/^#?/, '')
  if (!text) throw 'Give a session number or part of its title'
  if (/^\d+$/.test(text)) {
    const byNumber = sessions.find((s) => s.n === Number(text))
    if (byNumber) return byNumber
  }
  const matches = sessions.filter((s) => includes(s.title, query.trim()))
  if (matches.length === 1) return matches[0]
  if (matches.length > 1) throw `${matches.length} sessions match "${query}": ${matches.map((s) => s.n).join(', ')}`
  throw `No session matches "${query}"`
}

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
      a.setComposerAccess('supervised')
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
    id: 'open-session',
    label: 'Open session',
    group: 'Sessions',
    uiCommand: 'open',
    run(args) {
      const session = findSession(a.projectSessions, args.join(' '))
      a.select(session.id)
      return `Opened ${describe(session)}`
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
    id: 'next-attention',
    label: 'Next that needs you',
    group: 'Sessions',
    shortcut: { key: 'e' },
    uiCommand: 'next',
    run() {
      a.nextAttention()
    },
  },
]
