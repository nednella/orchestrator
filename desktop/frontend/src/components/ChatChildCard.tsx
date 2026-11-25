import { Folder, GitBranch } from 'lucide-react'
import { memo, useLayoutEffect, useRef, useState } from 'react'
import type { KeyboardEvent } from 'react'
import { ACCESS_HINT, ACCESS_LABEL } from '../access'
import { useAgentos, useAgentosSelector } from '../AgentosContext'
import { api } from '../api'
import { childFacts } from '../childCards'
import type { ChildFacts } from '../childCards'
import { STATE_LABEL } from '../stateMeta'
import { useArmedConfirm } from '../useArmedConfirm'
import type { Point } from '../useFloatingAt'
import { ChatMarkdown } from './ChatMarkdown'
import { ContextMenu } from './ContextMenu'
import { DeleteConfirm } from './DeleteConfirm'
import { StateDot } from './StateDot'

// id is the child's thread, '' while the call that starts it runs; title is what the call named it, for a child the
// sessions list does not hold. result is the child's result, for the card that shows it.
type ChatChildCardProps = { id: string; title: string; result?: string }

const isMenuKey = (e: KeyboardEvent) => e.key === 'ContextMenu' || (e.shiftKey && e.key === 'F10')

// ChatChildCard is a child thread in its parent's chat, live from the sessions list, so a change of the child renders
// this card and nothing else of the chat. A click on the card, but not in its result, opens the child; the card's line
// is the button that does it from the keyboard, so nothing interactive nests in another. Its actions are in its
// right-click menu.
export const ChatChildCard = memo(function ChatChildCard({ id, title, result }: ChatChildCardProps) {
  const a = useAgentos('select', 'settleThread', 'report')
  const child = useAgentosSelector((s) => (id ? s.sessions.find((x) => x.id === id) : undefined))
  const parentProject = useAgentosSelector((s) => s.sessions.find((x) => x.id === child?.parent)?.project ?? '')
  const facts = childFacts(child, title)
  const projectName = useAgentosSelector((s) => s.projects.find((p) => p.key === facts.project)?.name ?? facts.project)
  const [menuAt, setMenuAt] = useState<Point | null>(null)
  const remove = useArmedConfirm()
  const card = useRef<HTMLDivElement>(null)
  const working = facts.state === 'working' || facts.state === 'waiting'

  const open = () => child && a.select(child.id)
  const onKeyDown = (e: KeyboardEvent) => {
    if (!isMenuKey(e) || !child) return
    e.preventDefault()
    const box = card.current?.getBoundingClientRect()
    if (box) setMenuAt({ x: box.left + 12, y: box.bottom })
  }

  return (
    <div className="flex flex-col gap-2">
      <div
        ref={card}
        className={`chat-card gap-1.5 ${child ? 'cursor-pointer hover:bg-hover' : ''}`}
        data-state={facts.state}
        // A click that ends a selection of the result's text does not open the child.
        onClick={() => window.getSelection()?.isCollapsed !== false && open()}
        onKeyDown={onKeyDown}
        onContextMenu={(e) => {
          if (!child) return
          e.preventDefault()
          setMenuAt({ x: e.clientX, y: e.clientY })
        }}
      >
        {child ? (
          <button className="flex min-w-0 items-center gap-2 text-left text-small outline-none focus-visible:underline" aria-label={`Open ${facts.title}`}>
            <CardLine id={id} facts={facts} />
          </button>
        ) : (
          <div className="flex min-w-0 items-center gap-2 text-small">
            <CardLine id={id} facts={facts} />
          </div>
        )}
        {child && (
          <div className="flex min-w-0 items-center gap-x-3 pl-4 text-small text-dim">
            {facts.project !== parentProject && <span className="flex-none">{projectName}</span>}
            {facts.access && <span title={ACCESS_HINT[facts.access]}>{ACCESS_LABEL[facts.access]}</span>}
            <span className="flex min-w-0 items-center gap-1">
              {child.worktree ? <GitBranch size={12} className="flex-none" aria-hidden /> : <Folder size={12} className="flex-none" aria-hidden />}
              <span className="mono min-w-0 truncate">{facts.where}</span>
            </span>
          </div>
        )}
        {result && <ChildResult text={result} />}
      </div>
      {remove.armed && child && <DeleteConfirm session={child} onClose={remove.disarm} />}
      {menuAt && child && (
        <ContextMenu
          label={`Actions for ${facts.title}`}
          at={menuAt}
          onClose={() => setMenuAt(null)}
          items={[
            { label: 'Open', run: open },
            working && { label: 'Stop', run: () => a.report(() => api.chatInterrupt(child.id)) },
            facts.state !== 'settled' && { label: 'Sleep', danger: true, separated: true, run: () => a.report(() => a.settleThread(child.id)) },
            { label: 'Delete', danger: true, separated: facts.state === 'settled', run: () => remove.arm() },
          ]}
        />
      )}
    </div>
  )
})

type CardLineProps = { id: string; facts: ChildFacts }

function CardLine({ id, facts }: CardLineProps) {
  return (
    <>
      <ChildDot state={id ? facts.state : 'starting'} />
      <span className="min-w-0 truncate font-medium text-ink">{facts.title}</span>
      <span className="ml-auto flex-none text-dim">{stateText(id, facts.state)}</span>
    </>
  )
}

function stateText(id: string, state: ChildFacts['state']): string {
  if (!id) return 'Starting'
  if (state === 'gone') return 'Gone'
  return STATE_LABEL[state]
}

type ChildDotProps = { state: ChildFacts['state'] | 'starting' }

// ChildDot is the child's state dot; a child not started yet or gone from the list has only a faint ring.
function ChildDot({ state }: ChildDotProps) {
  if (state === 'gone' || state === 'starting') return <span className="size-2 flex-none rounded-full shadow-[inset_0_0_0_1.5px_var(--text-dim)]" aria-hidden />
  return <StateDot state={state} />
}

type ChildResultProps = { text: string }

// ChildResult is the child's result, cut to three lines until the owner shows the rest. A click in it, on a link, a
// copy button or text, never opens the child.
function ChildResult({ text }: ChildResultProps) {
  const body = useRef<HTMLDivElement>(null)
  const [full, setFull] = useState(false)
  const [cut, setCut] = useState(false)

  useLayoutEffect(() => {
    const el = body.current
    if (!el || full) return
    const measure = () => setCut(el.scrollHeight > el.clientHeight + 1)
    measure()
    const observer = new ResizeObserver(measure)
    observer.observe(el)
    return () => observer.disconnect()
  }, [text, full])

  return (
    <div className="flex cursor-auto flex-col items-start gap-1 border-t border-line pt-2" onClick={(e) => e.stopPropagation()}>
      <div ref={body} className={`w-full text-ink select-text ${full ? '' : 'line-clamp-3'}`}>
        <ChatMarkdown text={text} />
      </div>
      {(cut || full) && (
        <button className="text-small text-dim hover:text-ink" onClick={() => setFull(!full)}>
          {full ? 'Show less' : 'Show more'}
        </button>
      )}
    </div>
  )
}
