import { Search, SquarePen } from 'lucide-react'
import { useMemo, useRef, useState } from 'react'
import type { KeyboardEvent } from 'react'
import { useOrchestrator } from '../OrchestratorContext'
import { useLayout } from '../LayoutContext'
import { matchesThread, sortThreads } from '../threads'
import { useFocusRequest } from '../useFocusRequest'
import { useListNav } from '../useListNav'
import { useScrollCursorIntoView } from '../useScrollCursorIntoView'
import { ThreadRow } from './ThreadRow'

// A request, from the keyboard, that the row of thread id opens its menu; n counts the requests.
export type MenuRequest = { id: string; n: number }

export function ThreadSidebar() {
  const { projectThreads, threadTree, selectedId, select, focus, setComposerMode } = useOrchestrator('projectThreads', 'threadTree', 'selectedId', 'select', 'focus', 'setComposerMode')
  const { widths, focusChat, showHome, focusPanel } = useLayout()
  const panel = useRef<HTMLElement>(null)
  const list = useRef<HTMLDivElement>(null)
  const search = useRef<HTMLInputElement>(null)
  const [query, setQuery] = useState('')
  const [menuRequest, setMenuRequest] = useState<MenuRequest | null>(null)

  useFocusRequest('sidebar', panel)
  useFocusRequest('thread-search', search)

  // A selected child is not in the list, so its top-level thread's row keeps the owner's place.
  const selectedRow = threadTree.top.get(selectedId ?? '')?.id
  const threads = useMemo(() => sortThreads(projectThreads.filter((s) => matchesThread(s, query)), threadTree), [projectThreads, threadTree, query])

  const nav = useListNav(
    threads.map((s) => s.id),
    {
      onEnter(index) {
        select(threads[index].id)
        focusPanel('centre')
      },
    },
  )

  useScrollCursorIntoView(list, nav.cursorKey)

  const onKeyDown = (e: KeyboardEvent) => {
    if (e.key === 'Escape') {
      e.preventDefault()
      focusChat()
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
      focusChat()
      return
    }
    if (e.key !== 'Enter') return
    const first = threads.find((s) => s.state !== 'settled')
    if (!first) return
    select(first.id)
    focusPanel('centre')
  }

  return (
    <aside
      ref={panel}
      data-panel="sidebar"
      tabIndex={-1}
      className="panel flex min-h-0 flex-none flex-col overflow-hidden"
      style={{ width: `${widths.sidebar}rem` }}
      onKeyDown={onKeyDown}
      onFocus={(e) => {
        if (e.target === e.currentTarget && selectedRow) nav.setCursorKey(selectedRow)
      }}
    >
      <div className="flex flex-none items-center gap-1.5 px-1.5 pt-2 pb-2.5">
        <label className="thread-search flex h-[1.875rem] min-w-0 flex-1 cursor-text items-center gap-2 rounded-lg px-2.5 text-dim">
          <Search size={14} aria-hidden="true" />
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
          <SquarePen size={14} aria-hidden="true" />
        </button>
      </div>
      <div ref={list} className="flex min-h-0 flex-1 flex-col gap-0.5 overflow-y-auto pb-1">
        {threads.map((s) => (
          <ThreadRow key={s.id} thread={s} needs={threadTree.attention.get(s.id)} selected={s.id === selectedRow} cursor={nav.cursorKey === s.id} menuRequest={menuRequest?.id === s.id ? menuRequest.n : 0} />
        ))}
        {threads.length === 0 && <p className="px-4 py-4 text-small text-dim">{query.trim() ? 'No thread matches.' : 'No threads'}</p>}
      </div>
    </aside>
  )
}
