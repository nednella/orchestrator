import { memo, useCallback, useEffect, useId, useRef, useState } from 'react'
import type { KeyboardEvent } from 'react'
import { GitBranch } from 'lucide-react'
import { useAgentos, useAgentosSelector, useLatestAgentos } from '../AgentosContext'
import { splitBranch } from '../branchLabel'
import { approvalLine } from '../chatQuestions'
import { confirmDelete } from '../confirm'
import { currentTask } from '../threads'
import type { Session } from '../types'
import type { Point } from '../useFloatingAt'
import { ClaudeMark } from './ClaudeMark'
import { CleanupMark } from './CleanupMark'
import { Icon } from './Icon'
import { InlineInput } from './InlineInput'
import { LastActive } from './LastActive'
import { StateIcon } from './StateIcon'
import { ThreadAnswer } from './ThreadAnswer'
import { ThreadCard } from './ThreadCard'
import { ThreadMenu } from './ThreadMenu'
import { ThreadTask } from './ThreadTask'
import { useHoverCard } from './useHoverCard'

// needs is the thread the owner must answer for this one: itself, or the descendant of it that waits. selected marks
// the row of the open thread, or of the open thread's top-level thread. menuRequest changes when the list asks this
// row, from the keyboard, to open its menu.
type ThreadRowProps = { session: Session; needs: Session | undefined; selected: boolean; cursor: boolean; menuRequest: number }

const isMenuKey = (e: KeyboardEvent) => e.key === 'ContextMenu' || (e.shiftKey && e.key === 'F10')

function ThreadRowView({ session, needs, selected, cursor, menuRequest }: ThreadRowProps) {
  const a = useAgentos('select', 'showHome', 'focus', 'renameSession', 'wakeThread', 'deleteThread', 'report')
  const latest = useLatestAgentos()
  const row = useRef<HTMLDivElement>(null)
  const [menuAt, setMenuAt] = useState<Point | null>(null)
  const [renaming, setRenaming] = useState(false)
  const asker = needs ?? session
  const pending = useAgentosSelector((s) => s.pending[asker.id])
  const open = useAgentosSelector((s) => s.selectedId === session.id)
  // A tool approval names what Allow would let run, from the approval itself, since the hooks' detail can lag behind.
  const asking = pending?.kind === 'tool' ? approvalLine(pending) : asker.detail
  const card = useHoverCard(400)
  const cardId = useId()
  const { show: showCard, close: closeCard } = card
  const settled = session.state === 'settled'
  const branch = splitBranch(session.branch)

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

  const askDelete = () => a.report(() => confirmDelete(session, () => latest().sessions, a.deleteThread))
  const line2 = settled || session.branch !== '' || currentTask(session) !== null || session.cleanup === 'blocked' || session.cleanup === 'ask'

  return (
    <div
      ref={row}
      className="row row-inset thread-row group flex-col"
      data-state={needs ? 'waiting' : session.state}
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
          <StateIcon state={needs ? 'waiting' : session.state} />
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
              className="thread-open thread-dim min-w-0 truncate text-left text-body leading-[1.375rem] font-medium"
              aria-current={open}
              aria-describedby={card.anchor ? cardId : undefined}
              onFocus={(e) => e.currentTarget.matches(':focus-visible') && row.current && showCard(row.current)}
              onBlur={closeCard}
              onMouseDown={(e) => e.preventDefault()}
              onClick={() => {
                if (open) return a.showHome(session.project)
                a.select(session.id)
                a.focus('thread')
              }}
            >
              {session.title}
            </button>
            <span className="col-start-3 flex items-center justify-self-end gap-2">
              <span className="thread-age thread-dim justify-self-end text-label text-dim tabular-nums">
                <LastActive at={session.lastEventAt} />
              </span>
              {selected && (
                <span className="thread-actions relative z-[1] items-center justify-self-end">
                  <button className="btn btn-ghost h-5 w-6 justify-center px-0" title="Close thread" aria-label="Close thread" onClick={() => a.showHome(session.project)}>
                    <Icon name="x" size={14} />
                  </button>
                </span>
              )}
              {session.state === 'working' && <span className="thread-working"><ClaudeMark size={14} /></span>}
            </span>
          </>
        )}
        {line2 && (
          <div className="col-span-2 col-start-2 flex min-w-0 items-center gap-2 text-label">
            <span className="thread-dim flex min-w-0 flex-1 items-center gap-1.5 text-soft">
              {session.branch && (
                <>
                  <GitBranch size={12} className="flex-none text-dim" aria-hidden="true" />
                  <span className="flex min-w-0">
                    <span className="truncate">{branch.head}</span>
                    {branch.tail && <span className="flex-none">{branch.tail}</span>}
                  </span>
                </>
              )}
            </span>
            {settled ? (
              <span className="relative z-[1] flex flex-none items-center gap-1.5">
                <CleanupMark session={session} />
                {session.hosted && (
                  <button className="btn h-[1.375rem] px-2 text-label" title="Start this thread again" onClick={() => a.report(() => a.wakeThread(session.id))}>
                    Wake
                  </button>
                )}
                <button className="btn h-[1.375rem] px-2 text-label" title="Remove this thread and its chat" onClick={askDelete}>
                  Delete
                </button>
              </span>
            ) : (
              <span className="flex flex-none items-center gap-1.5">
                <CleanupMark session={session} />
                <ThreadTask session={session} />
              </span>
            )}
          </div>
        )}
        {needs && (asking || pending) && (
          <div className="thread-question col-span-2 col-start-2 flex min-w-0 flex-col gap-1.5">
            {needs !== session && <p className="truncate font-medium text-ink">{needs.title}</p>}
            {asking && <p className="truncate">{asking}</p>}
            {pending && <ThreadAnswer key={pending.request} session={needs} pending={pending} />}
          </div>
        )}
      </div>
      {menuAt && (
        <ThreadMenu
          session={session}
          selected={open}
          at={menuAt}
          onClose={() => setMenuAt(null)}
          onRename={() => setRenaming(true)}
          onDelete={askDelete}
        />
      )}
      {card.anchor && !menuAt && !renaming && <ThreadCard id={cardId} session={session} anchor={card.anchor} />}
    </div>
  )
}

export const ThreadRow = memo(ThreadRowView)
