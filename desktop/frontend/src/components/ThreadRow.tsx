import { memo, useCallback, useEffect, useId, useRef, useState } from 'react'
import type { KeyboardEvent } from 'react'
import { GitBranch, X } from 'lucide-react'
import { useOrchestrator, useOrchestratorSelector, useLatestOrchestrator } from '../OrchestratorContext'
import { splitBranch } from '../branchLabel'
import { approvalLine } from '../chatQuestions'
import { confirmDelete } from '../confirm'
import { gigabytes, memoryNote } from '../memory'
import { clickCloses } from '../threadClick'
import { threadGoal } from '../threads'
import type { Thread } from '../types'
import type { Point } from '../useFloatingAt'
import { ClaudeMark } from './ClaudeMark'
import { CleanupMark } from './CleanupMark'
import { InlineInput } from './InlineInput'
import { LastActive } from './LastActive'
import { StateIcon } from './StateIcon'
import { ThreadAnswer } from './ThreadAnswer'
import { ThreadCard } from './ThreadCard'
import { ThreadMenu } from './ThreadMenu'
import { ThreadGoal } from './ThreadGoal'
import { useHoverCard } from './useHoverCard'

// needs is the thread the owner must answer for this one: itself, or the descendant of it that waits. selected marks
// the row of the open thread, or of the open thread's top-level thread. menuRequest changes when the list asks this
// row, from the keyboard, to open its menu.
type ThreadRowProps = { thread: Thread; needs: Thread | undefined; selected: boolean; cursor: boolean; menuRequest: number }

const isMenuKey = (e: KeyboardEvent) => e.key === 'ContextMenu' || (e.shiftKey && e.key === 'F10')

function ThreadRowView({ thread, needs, selected, cursor, menuRequest }: ThreadRowProps) {
  const a = useOrchestrator('select', 'showHome', 'focus', 'renameThread', 'wakeThread', 'deleteThread', 'report')
  const latest = useLatestOrchestrator()
  const row = useRef<HTMLDivElement>(null)
  const [menuAt, setMenuAt] = useState<Point | null>(null)
  const [renaming, setRenaming] = useState(false)
  const asker = needs ?? thread
  const pending = useOrchestratorSelector((s) => s.pending[asker.id])
  const open = useOrchestratorSelector((s) => s.selectedId === thread.id)
  // A tool approval names what Allow would let run, from the approval itself, since the hooks' detail can lag behind.
  const asking = pending?.kind === 'tool' ? approvalLine(pending) : asker.detail
  const card = useHoverCard(600)
  const cardId = useId()
  const { show: showCard, close: closeCard } = card
  const settled = thread.state === 'settled'
  const branch = splitBranch(thread.branch)

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

  const askDelete = () => a.report(() => confirmDelete(thread, () => latest().threads, a.deleteThread))
  const line2 = settled || thread.branch !== '' || thread.memory > 0 || threadGoal(thread) !== null || thread.cleanup === 'blocked' || thread.cleanup === 'ask'

  return (
    <div
      ref={row}
      className="row row-inset thread-row group flex-col"
      data-state={needs ? 'waiting' : thread.state}
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
          <StateIcon state={needs ? 'waiting' : thread.state} />
        </span>
        {renaming ? (
          <div className="col-span-2 flex min-w-0">
            <InlineInput
              initial={thread.title}
              blur="submit"
              className="h-7 w-full"
              onSubmit={(title) => {
                setRenaming(false)
                a.report(() => a.renameThread(thread.id, title))
              }}
              onCancel={() => setRenaming(false)}
            />
          </div>
        ) : (
          <>
            <button
              className="thread-open thread-dim min-w-0 truncate text-left text-body leading-[1.375rem] font-medium"
              aria-current={open || undefined}
              aria-describedby={card.anchor ? cardId : undefined}
              onFocus={(e) => e.currentTarget.matches(':focus-visible') && row.current && showCard(row.current)}
              onBlur={closeCard}
              onMouseDown={(e) => e.preventDefault()}
              onClick={(e) => {
                if (open) {
                  if (clickCloses(open, e.detail)) a.showHome(thread.project)
                  return
                }
                a.select(thread.id)
                a.focus('thread')
              }}
            >
              {thread.title}
            </button>
            <span className="col-start-3 flex items-center justify-self-end gap-2">
              <span className="thread-age thread-dim justify-self-end text-label text-dim tabular-nums">
                <LastActive at={thread.lastEventAt} />
              </span>
              {selected && (
                <span className="thread-actions relative z-[1] items-center justify-self-end">
                  <button className="btn btn-ghost h-5 w-6 justify-center px-0" title="Close thread" aria-label="Close thread" onClick={() => a.showHome(thread.project)}>
                    <X size={14} aria-hidden="true" />
                  </button>
                </span>
              )}
              {thread.state === 'working' && <span className="thread-working"><ClaudeMark size={14} /></span>}
            </span>
          </>
        )}
        {line2 && (
          <div className="col-span-2 col-start-2 flex min-w-0 items-center gap-2 text-label">
            <span className="thread-dim flex min-w-0 flex-1 items-center gap-1.5 text-soft">
              {thread.branch && (
                <>
                  <GitBranch size={12} className="flex-none text-dim" aria-hidden="true" />
                  <span className="flex min-w-0">
                    <span className="truncate">{branch.head}</span>
                    {branch.tail && <span className="flex-none">{branch.tail}</span>}
                  </span>
                </>
              )}
              {thread.memory > 0 && (
                <span
                  className="relative z-[1] flex-none cursor-default whitespace-nowrap text-dim tabular-nums"
                  title={`The agent, every process under it, what it started that left its terminal, and its agent-browser. ${memoryNote}`}
                >
                  {thread.branch ? `· ${gigabytes(thread.memory)}` : gigabytes(thread.memory)}
                </span>
              )}
            </span>
            {settled ? (
              <span className="relative z-[1] flex flex-none items-center gap-1.5">
                <CleanupMark thread={thread} />
                <button className="btn h-[1.375rem] px-2 text-label" title="Start this thread again" onClick={() => a.report(() => a.wakeThread(thread.id))}>
                  Wake
                </button>
                <button className="btn h-[1.375rem] px-2 text-label" title="Remove this thread and its chat" onClick={askDelete}>
                  Delete
                </button>
              </span>
            ) : (
              <span className="flex flex-none items-center gap-1.5">
                <CleanupMark thread={thread} />
                <ThreadGoal thread={thread} />
              </span>
            )}
          </div>
        )}
        {needs && (asking || pending) && (
          <div className="thread-question col-span-2 col-start-2 flex min-w-0 flex-col gap-1.5">
            {needs !== thread && <p className="truncate font-medium text-ink">{needs.title}</p>}
            {asking && <p className="truncate">{asking}</p>}
            {pending && <ThreadAnswer key={pending.request} thread={needs} pending={pending} />}
          </div>
        )}
      </div>
      {menuAt && (
        <ThreadMenu
          thread={thread}
          selected={open}
          at={menuAt}
          onClose={() => setMenuAt(null)}
          onRename={() => setRenaming(true)}
          onDelete={askDelete}
        />
      )}
      {card.anchor && !menuAt && !renaming && <ThreadCard id={cardId} thread={thread} anchor={card.anchor} />}
    </div>
  )
}

export const ThreadRow = memo(ThreadRowView)
