import { memo, useCallback, useEffect, useRef, useState } from 'react'
import type { KeyboardEvent } from 'react'
import { useAgentos } from '../AgentosContext'
import { useLayout } from '../LayoutContext'
import type { Session } from '../types'
import { useArmedConfirm } from '../useArmedConfirm'
import type { Point } from '../useFloatingAt'
import { Ago } from './Ago'
import { CleanupControls } from './CleanupControls'
import { ConfirmRow } from './ConfirmRow'
import { ContextMenu } from './ContextMenu'
import { Icon } from './Icon'
import { InlineInput } from './InlineInput'
import { PRMark } from './PRMark'
import { StateDot } from './StateDot'
import { TimelinePopover } from './TimelinePopover'

// menuRequest changes when the list asks this row, from the keyboard, to open its menu.
type ThreadRowProps = { session: Session; selected: boolean; cursor: boolean; menuRequest: number }

const CLEANUP_LABEL: Record<Session['cleanup'], string> = { '': '', pending: '', ask: 'Clean up', blocked: 'Clean up anyway' }

const isMenuKey = (e: KeyboardEvent) => e.key === 'ContextMenu' || (e.shiftKey && e.key === 'F10')

function ThreadRowView({ session, selected, cursor, menuRequest }: ThreadRowProps) {
  const a = useAgentos('select', 'showHome', 'focus', 'renameSession', 'killSession', 'dismissSession', 'resumeSession', 'openPR', 'pushToast', 'report')
  const { mode, mobilePanel, rightOpen, setRightOpen, toggleRight } = useLayout()
  const row = useRef<HTMLDivElement>(null)
  const [menuAt, setMenuAt] = useState<Point | null>(null)
  const [timelineAt, setTimelineAt] = useState<Point | null>(null)
  const [renaming, setRenaming] = useState(false)
  const kill = useArmedConfirm()
  const cleanup = useArmedConfirm()
  const ended = session.state === 'ended'
  const panelShown = selected && (mode === 'narrow' ? mobilePanel === 'details' : rightOpen)

  // The request this row has answered: a row that mounts, or gets the cursor back, with an old one does not open.
  const answered = useRef(menuRequest)

  const openMenuBelow = useCallback(() => {
    const box = row.current?.getBoundingClientRect()
    if (box) setMenuAt({ x: box.left + 12, y: box.bottom })
  }, [])

  useEffect(() => {
    if (menuRequest === 0 || menuRequest === answered.current) return
    answered.current = menuRequest
    openMenuBelow()
  }, [menuRequest, openMenuBelow])

  const toggleOwnPanel = () => {
    if (selected) return a.report(toggleRight)
    a.select(session.id)
    setRightOpen(true)
  }

  const copyBranch = async () => {
    await navigator.clipboard.writeText(session.branch)
    a.pushToast({ tone: 'info', text: `Copied ${session.branch}` })
  }

  return (
    <div
      ref={row}
      className="row row-inset thread-row group flex-col"
      data-state={session.state}
      data-selected={selected}
      data-cursor={cursor}
      data-menu-open={menuAt !== null}
      style={{ opacity: ended ? 0.6 : 1 }}
      onContextMenu={(e) => {
        e.preventDefault()
        setMenuAt({ x: e.clientX, y: e.clientY })
      }}
      onKeyDown={(e) => {
        if (!isMenuKey(e)) return
        e.preventDefault()
        e.stopPropagation()
        openMenuBelow()
      }}
    >
      <div className="flex w-full items-center">
        {renaming ? (
          <div className="flex min-w-0 flex-1 py-1 pr-2 pl-3">
            <InlineInput
              initial={session.title}
              blur="submit"
              className="h-7 w-full"
              onSubmit={(title) => {
                setRenaming(false)
                a.report(() => a.renameSession(session.id, title))
              }}
              onCancel={() => setRenaming(false)}
            />
          </div>
        ) : (
          <button
            className="flex min-w-0 flex-1 flex-col gap-0.5 py-1.5 pr-2 pl-3 text-left"
            aria-current={selected}
            disabled={ended}
            title={ended ? `${session.title} (ended)` : session.title}
            style={ended ? { cursor: 'default', opacity: 1 } : undefined}
            onMouseDown={(e) => e.preventDefault()}
            onClick={() => {
              if (selected) return a.showHome(session.project)
              a.select(session.id)
              a.focus('terminal')
            }}
          >
            <span className="flex w-full items-center gap-2">
              <StateDot state={session.state} />
              <span className={`min-w-0 flex-1 truncate text-body ${selected ? 'font-semibold' : 'font-medium'}`}>{session.title}</span>
              <span className="mono flex-none text-label text-dim">
                <Ago at={session.lastEventAt} />
              </span>
            </span>
            {(session.branch || session.pr) && (
              <span className="flex w-full items-center gap-2 pl-4">
                <span className="mono min-w-0 flex-1 truncate text-label text-dim">{session.branch}</span>
                {session.pr && <PRMark pr={session.pr} />}
              </span>
            )}
          </button>
        )}
        {(selected || ended) && !renaming && (
          <button
            className={`btn btn-ghost mr-1 h-6 w-6 flex-none justify-center px-0 ${selected ? '' : 'opacity-0 group-focus-within:opacity-100 group-hover:opacity-100'}`}
            title={ended ? 'Dismiss' : 'Close thread'}
            aria-label={ended ? `Dismiss ${session.title}` : 'Close thread'}
            onClick={() => (ended ? a.report(() => a.dismissSession(session.id)) : a.showHome(session.project))}
          >
            <Icon name="close" size={12} />
          </button>
        )}
      </div>
      {session.cleanup !== '' && (
        <div className="flex w-full pr-2 pb-1.5 pl-7">
          <CleanupControls session={session} remove={cleanup} />
        </div>
      )}
      {kill.armed && (
        <div className="flex w-full pr-2 pb-1.5 pl-7">
          <ConfirmRow
            danger
            stacked
            label="Kill this thread"
            message={
              <>
                Kill this thread? The agent stops now. Its conversation stays on disk and can be resumed from a terminal with <code className="mono">claude --resume</code>.
              </>
            }
            confirmLabel="Kill"
            onCancel={kill.disarm}
            onConfirm={() => {
              kill.disarm()
              a.report(() => a.killSession(session.id))
            }}
          />
        </div>
      )}
      {menuAt && (
        <ContextMenu
          label={`Actions for ${session.title}`}
          at={menuAt}
          onClose={() => setMenuAt(null)}
          items={[
            { label: 'Rename', run: () => setRenaming(true) },
            !session.chat && { label: 'Show timeline', run: () => setTimelineAt(menuAt) },
            session.pr !== null && { label: `Open PR #${session.pr.number}`, run: () => a.report(() => a.openPR(session)) },
            session.branch !== '' && { label: 'Copy branch', run: () => a.report(copyBranch) },
            CLEANUP_LABEL[session.cleanup] !== '' && { label: CLEANUP_LABEL[session.cleanup], run: () => cleanup.arm() },
            { label: panelShown ? 'Hide right panel' : 'Show right panel', run: toggleOwnPanel },
            ended && session.resumable && { label: 'Resume', separated: true, run: () => a.report(() => a.resumeSession(session.id)) },
            ended && { label: 'Dismiss', separated: !session.resumable, run: () => a.report(() => a.dismissSession(session.id)) },
            !ended && { label: 'Kill thread', danger: true, separated: true, run: () => kill.arm() },
          ]}
        />
      )}
      {timelineAt && <TimelinePopover session={session} at={timelineAt} onClose={() => setTimelineAt(null)} />}
    </div>
  )
}

export const ThreadRow = memo(ThreadRowView)
