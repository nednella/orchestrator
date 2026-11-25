import { useMemo, useRef, useState } from 'react'
import type { KeyboardEvent } from 'react'
import { useAgentos } from '../AgentosContext'
import { useLayout } from '../LayoutContext'
import { matchesThread, sortThreads } from '../threads'
import { useFocusRequest } from '../useFocusRequest'
import { useListNav } from '../useListNav'
import { useScrollCursorIntoView } from '../useScrollCursorIntoView'
import { Icon } from './Icon'
import { ThreadRow } from './ThreadRow'

// A request, from the keyboard, that the row of thread id opens its menu; n counts the requests.
export type MenuRequest = { id: string; n: number }

export function ThreadSidebar() {
  const { projectSessions, selectedId, select, focus, setComposerMode } = useAgentos('projectSessions', 'selectedId', 'select', 'focus', 'setComposerMode')
  const { mode, widths, returnToTerminal, showHome } = useLayout()
  const panel = useRef<HTMLElement>(null)
  const list = useRef<HTMLDivElement>(null)
  const search = useRef<HTMLInputElement>(null)
  const [query, setQuery] = useState('')
  const [menuRequest, setMenuRequest] = useState<MenuRequest | null>(null)
  const narrow = mode === 'narrow'

  useFocusRequest('sidebar', panel)
  useFocusRequest('thread-search', search)

  const threads = useMemo(() => sortThreads(projectSessions.filter((s) => matchesThread(s, query))), [projectSessions, query])

  const nav = useListNav(
    threads.map((s) => s.id),
    {
      onEnter(index) {
        const session = threads[index]
        if (session.state === 'settled') return
        select(session.id)
        focus('thread')
      },
    },
  )

  useScrollCursorIntoView(list, nav.cursorKey)

  const onKeyDown = (e: KeyboardEvent) => {
    if (e.key === 'Escape') {
      e.preventDefault()
      returnToTerminal()
      return
    }
    const id = nav.cursorKey
    if ((e.key === 'ContextMenu' || (e.shiftKey && e.key === 'F10')) && id) {
      e.preventDefault()
      setMenuRequest((last) => ({ id, n: (last?.n ?? 0) + 1 }))
      return
    }
    if (nav.handle(e)) e.preventDefault()
  }

  const newThread = () => {
    showHome()
    setComposerMode('thread')
    focus('composer')
  }

  const onSearchKeyDown = (e: KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'Escape') {
      e.stopPropagation()
      setQuery('')
      e.currentTarget.blur()
      returnToTerminal()
      return
    }
    if (e.key !== 'Enter') return
    const first = threads.find((s) => s.state !== 'settled')
    if (!first) return
    select(first.id)
    focus('thread')
  }

  return (
    <aside
      ref={panel}
      data-panel="sidebar"
      tabIndex={-1}
      className={`panel flex min-h-0 flex-col overflow-hidden ${narrow ? 'min-w-0 flex-1' : 'flex-none'}`}
      style={narrow ? undefined : { width: `${widths.sidebar}rem` }}
      onKeyDown={onKeyDown}
      onFocus={(e) => {
        if (e.target === e.currentTarget && selectedId) nav.setCursorKey(selectedId)
      }}
    >
      <div className="flex flex-none items-center gap-1.5 px-1.5 pt-2 pb-2.5">
        <label className="thread-search flex h-[1.875rem] min-w-0 flex-1 cursor-text items-center gap-2 rounded-lg px-2.5 text-dim">
          <Icon name="search" />
          <input
            ref={search}
            value={query}
            placeholder="Search threads"
            aria-label="Search threads"
            className="min-w-0 flex-1 bg-transparent text-ink outline-none"
            onChange={(e) => setQuery(e.target.value)}
            onKeyDown={onSearchKeyDown}
          />
        </label>
        <button className="btn btn-ghost h-[1.875rem] w-[1.875rem] justify-center border-line px-0 hover:text-ink" title="New thread" aria-label="New thread" onClick={newThread}>
          <Icon name="compose" />
        </button>
      </div>
      <div ref={list} className="flex min-h-0 flex-1 flex-col gap-0.5 overflow-y-auto pb-1">
        {threads.map((s) => (
          <ThreadRow key={s.id} session={s} selected={s.id === selectedId} cursor={nav.cursorKey === s.id} menuRequest={menuRequest?.id === s.id ? menuRequest.n : 0} />
        ))}
        {threads.length === 0 && <p className="px-4 py-4 text-small text-dim">{query.trim() ? 'No thread matches.' : 'No threads'}</p>}
      </div>
    </aside>
  )
}
