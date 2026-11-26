import type { Action, ActionContext } from './types'

export const queueActions = ({ a }: ActionContext): Action[] => [
  {
    id: 'refresh-issues',
    label: 'Refresh issues and PRs',
    shortcut: { key: 'r' },
    async run() {
      await Promise.all([a.refreshIssues(), a.refreshPRs()])
    },
  },
]
