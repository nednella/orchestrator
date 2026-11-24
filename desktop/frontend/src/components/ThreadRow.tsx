import { memo, useCallback, useEffect, useId, useRef, useState } from 'react'
import type { KeyboardEvent } from 'react'
import { useAgentos } from '../AgentosContext'
import { currentTask } from '../threads'
import type { Session } from '../types'
import { useArmedConfirm } from '../useArmedConfirm'
import type { Point } from '../useFloatingAt'
import { CleanupControls } from './CleanupControls'
import { CleanupMark } from './CleanupMark'
import { ConfirmRow } from './ConfirmRow'
import { Icon } from './Icon'
import { InlineInput } from './InlineInput'
import { LastActive } from './LastActive'
import { StateIcon } from './StateIcon'
import { ThreadCard } from './ThreadCard'
import { ThreadMenu } from './ThreadMenu'
import { ThreadTask } from './ThreadTask'
import { TimelinePopover } from './TimelinePopover'
import { useHoverCard } from './useHoverCard'

// menuRequest changes when the list asks this row, from the keyboard, to open its menu.
type ThreadRowProps = { session: Session; selected: boolean; cursor: boolean; menuRequest: number }

const isMenuKey = (e: KeyboardEvent) => e.key === 'ContextMenu' || (e.shiftKey && e.key === 'F10')

function ThreadRowView({ session, selected, cursor, menuRequest }: ThreadRowProps) {
  const a = useAgentos('select', 'showHome', 'focus', 'renameSession', 'killSession', 'dismissSession', 'report')
  const row = useRef<HTMLDivElement>(null)
  const [menuAt, setMenuAt] = useState<Point | null>(null)
  const [timelineAt, setTimelineAt] = useState<Point | null>(null)
  const [renaming, setRenaming] = useState(false)
  const kill = useArmedConfirm()
  const cleanup = useArmedConfirm()
  const card = useHoverCard(400)
  const cardId = useId()
  const { show: showCard, close: closeCard } = card
  const ended = session.state === 'ended'

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

  // The list's cursor is the keyboard's place in it: the card follows it while the list holds the focus.
  useEffect(() => {
    const el = row.current
    const list = el?.closest<HTMLElement>('[data-panel]')
    if (!cursor || !el || !list || document.activeElement !== list) return
    showCard(el)
    list.addEventListener('blur', closeCard)
    return () => {
      list.removeEventListener('blur', closeCard)
      closeCard()
    }
  }, [cursor, showCard, closeCard])

  const line2 = session.branch !== '' || currentTask(session) !== null || session.cleanup === 'blocked' || session.cleanup === 'ask'

  return (
    <div
      ref={row}
      className="row row-inset thread-row group flex-col"
      data-state={session.state}
      data-selected={selected}
      data-cursor={cursor}
      data-menu-open={menuAt !== null}
      {...card.hover}
      onMouseDown={closeCard}
      onContextMenu={(e) => {
        e.preventDefault()
        closeCard()
        setMenuAt({ x: e.clientX, y: e.clientY })
      }}
      onKeyDown={(e) => {
        if (!isMenuKey(e)) return
        e.preventDefault()
        e.stopPropagation()
        openMenuBelow()
      }}
    >
      <div className="thread-card">
        <span className="thread-dim flex h-[1.375rem] items-center">
          <StateIcon state={session.state} />
        </span>
        {renaming ? (
          <div className="col-span-2 flex min-w-0">
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
          <>
            <button
              className={`thread-open thread-dim min-w-0 truncate text-left text-body leading-[1.375rem] ${selected ? 'font-semibold' : 'font-medium'}`}
              aria-current={selected}
              aria-disabled={ended}
              aria-describedby={card.anchor ? cardId : undefined}
              style={ended ? { cursor: 'default' } : undefined}
              onFocus={(e) => e.currentTarget.matches(':focus-visible') && row.current && showCard(row.current)}
              onBlur={closeCard}
              onMouseDown={(e) => e.preventDefault()}
              onClick={() => {
                if (selected) return a.showHome(session.project)
                if (ended) return
                a.select(session.id)
                a.focus('terminal')
              }}
            >
              {session.title}
            </button>
            <span className="thread-age thread-dim justify-self-end text-label text-dim tabular-nums">
              <LastActive at={session.lastEventAt} />
            </span>
            <span className="thread-actions relative z-[1] items-center gap-0.5 justify-self-end">
              <button
                className="btn btn-ghost h-5 w-6 justify-center px-0"
                title="Thread actions"
                aria-label={`Actions for ${session.title}`}
                aria-haspopup="menu"
                onClick={(e) => {
                  const box = e.currentTarget.getBoundingClientRect()
                  closeCard()
                  setMenuAt({ x: box.left, y: box.bottom })
                }}
              >
                <Icon name="more" size={14} />
              </button>
              {(selected || ended) && (
                <button
                  className="btn btn-ghost h-5 w-6 justify-center px-0"
                  title={ended ? 'Dismiss' : 'Close thread'}
                  aria-label={ended ? `Dismiss ${session.title}` : 'Close thread'}
                  onClick={() => (ended ? a.report(() => a.dismissSession(session.id)) : a.showHome(session.project))}
                >
                  <Icon name="x" size={14} />
                </button>
              )}
            </span>
          </>
        )}
        {line2 && (
          <>
            <span className="thread-dim col-start-2 flex min-w-0 items-center gap-1.5 text-label text-soft">
              {session.branch && (
                <>
                  <span className="flex flex-none text-dim">
                    <Icon name="branch" size={12} />
                  </span>
                  <span className="truncate">{session.branch}</span>
                </>
              )}
            </span>
            <span className="thread-dim flex items-center gap-1.5 justify-self-end text-label">
              <CleanupMark session={session} />
              <ThreadTask session={session} />
            </span>
          </>
        )}
        {session.state === 'waiting' && session.detail && <p className="thread-question col-span-2 col-start-2 truncate">{session.detail}</p>}
      </div>
      {cleanup.armed && (
        <div className="flex w-full pr-3 pb-2 pl-9">
          <CleanupControls session={session} remove={cleanup} />
        </div>
      )}
      {kill.armed && (
        <div className="flex w-full pr-3 pb-2 pl-9">
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
        <ThreadMenu
          session={session}
          selected={selected}
          at={menuAt}
          onClose={() => setMenuAt(null)}
          onRename={() => setRenaming(true)}
          onTimeline={() => setTimelineAt(menuAt)}
          onCleanup={() => cleanup.arm()}
          onKill={() => kill.arm()}
        />
      )}
      {card.anchor && !menuAt && !renaming && <ThreadCard id={cardId} session={session} anchor={card.anchor} />}
      {timelineAt && <TimelinePopover session={session} at={timelineAt} onClose={() => setTimelineAt(null)} />}
    </div>
  )
}

export const ThreadRow = memo(ThreadRowView)
