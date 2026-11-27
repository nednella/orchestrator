import type { Cleanup, Warning } from './types'
import type { ToastInput } from './toastPile'

const NAME_MAX = 40

const shorten = (text: string) => (text.length > NAME_MAX ? `${text.slice(0, NAME_MAX - 1).trimEnd()}…` : text)

// threadName names a thread in a toast: `#<issue> <title>` for an issue's thread, else `Thread "<title>"`. A long title is
// cut, and `full` then holds the whole name, for a tooltip.
export function threadName(title: string, issue: number): { name: string; full?: string } {
  const whole = issue > 0 ? (title.startsWith(`#${issue}`) ? title : `#${issue} ${title}`) : `Thread "${title}"`
  const name = issue > 0 ? shorten(whole) : `Thread "${shorten(title)}"`
  return { name, full: name === whole ? undefined : whole }
}

// cleanupToast has an Open action for the PR the clean-up was for, which open runs with the PR's address.
export function cleanupToast(entry: Cleanup, open: (url: string) => void): ToastInput {
  const { name, full } = threadName(entry.threadTitle, entry.issue)
  const pr = entry.pr > 0 ? `PR #${entry.pr} ${entry.merged ? 'merged' : 'closed'}` : ''
  const removed = entry.removed.some((item) => item.startsWith('worktree ')) ? 'Worktree and branch removed' : 'Thread put to sleep'
  return { tone: 'success', text: `Cleaned up ${name}`, hint: full, detail: [pr, removed].filter(Boolean).join(' · '), action: entry.url ? { label: 'Open', run: () => open(entry.url) } : undefined }
}

const WARNING_SOURCES: Record<Warning['source'], string> = { tmux: 'tmux', github: 'GitHub', 'pull requests': 'pull requests', worktrees: 'worktrees', config: 'config', agent: 'the agent' }

export function warningToast({ source, message }: Warning): ToastInput {
  return { tone: source === 'tmux' ? 'error' : 'warning', text: `Problem with ${WARNING_SOURCES[source]}`, detail: message, sticky: true }
}
