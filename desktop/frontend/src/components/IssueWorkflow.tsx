import { useRef, useState } from 'react'
import type { MouseEvent } from 'react'
import { useOrchestrator } from '../OrchestratorContext'
import { api } from '../api'
import type { Issue } from '../types'
import type { Point } from '../useFloatingAt'
import { ContextMenu } from './ContextMenu'
import { Icon } from './Icon'

type IssueWorkflowProps = { issue: Issue; onMenu?(open: boolean): void }

// The section's actions as buttons: the first starts a thread at once, the second sits beside it, the rest wait in ⋯.
export function IssueWorkflow({ issue, onMenu }: IssueWorkflowProps) {
  const { startIssue, pushToast, report } = useOrchestrator('startIssue', 'pushToast', 'report')
  const [menuAt, setMenuAt] = useState<Point | null>(null)
  // A press on ⋯ while the menu is open closes it on mousedown; the click that follows must not open it again.
  const closing = useRef(false)
  const [first, second, ...rest] = issue.actions

  const start = (action: string, e: MouseEvent) => report(() => startIssue(issue.number, action, e.altKey))
  const showMenu = (at: Point | null) => {
    setMenuAt(at)
    onMenu?.(at !== null)
  }
  const copyLink = async () => {
    await navigator.clipboard.writeText(issue.url)
    pushToast({ tone: 'info', text: `Copied the link to #${issue.number}` })
  }

  return (
    <div className="flex flex-wrap items-center gap-1.5">
      {first && (
        <button className="btn btn-primary h-6 px-2" title={`${first}: start a thread for #${issue.number} (hold ⌥ to stay here)`} onClick={(e) => start(first, e)}>
          <Icon name="play" size={10} />
          {first}
        </button>
      )}
      {second && (
        <button className="btn h-6 px-2" title={`${second}: start a thread for #${issue.number} (hold ⌥ to stay here)`} onClick={(e) => start(second, e)}>
          {second}
        </button>
      )}
      <button
        className="btn btn-ghost h-6 w-6 justify-center px-0"
        title="More"
        aria-label={`More for #${issue.number}`}
        aria-haspopup="menu"
        aria-expanded={menuAt !== null}
        onMouseDown={() => {
          closing.current = menuAt !== null
        }}
        onClick={(e) => {
          if (closing.current) {
            closing.current = false
            return
          }
          const box = e.currentTarget.getBoundingClientRect()
          showMenu({ x: box.left, y: box.bottom + 4 })
        }}
      >
        <Icon name="more" />
      </button>
      {menuAt && (
        <ContextMenu
          label={`More for #${issue.number}`}
          at={menuAt}
          onClose={() => showMenu(null)}
          items={[
            ...rest.map((action) => ({ label: action, run: (e: MouseEvent) => start(action, e) })),
            { label: 'Open on GitHub', separated: rest.length > 0, run: () => report(() => api.openURL(issue.url)) },
            { label: 'Copy link', run: () => report(copyLink) },
          ]}
        />
      )}
    </div>
  )
}
