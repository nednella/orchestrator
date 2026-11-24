import { useMemo, useRef, useState } from 'react'
import type { KeyboardEvent } from 'react'
import { useAgentos } from '../AgentosContext'
import { useLayout } from '../LayoutContext'
import { readStored, writeStored } from '../storage'
import { matchesThread, splitThreads } from '../threads'
import { useFocusRequest } from '../useFocusRequest'
import { useListNav } from '../useListNav'
import { useScrollCursorIntoView } from '../useScrollCursorIntoView'
import { Icon } from './Icon'
import { Keycap } from './Keycap'
import { SETTLED_KEY, SettledSection } from './SettledSection'
import { SidebarFooter } from './SidebarFooter'
import { ThreadSection } from './ThreadSection'

const SETTLED_OPEN_KEY = 'agentos.sidebar.settled-open'

// A request, from the keyboard, that the row of thread id opens its menu; n counts the requests.
export type MenuRequest = { id: string; n: number }

export function ThreadSidebar() {
  const { projectSessions, selectedId, select, focus, setOverlay } = useAgentos('projectSessions', 'selectedId', 'select', 'focus', 'setOverlay')
  const { mode, widths, returnToTerminal } = useLayout()
  const panel = useRef<HTMLElement>(null)
  const list = useRef<HTMLDivElement>(null)
  const search = useRef<HTMLInputElement>(null)
  const [query, setQuery] = useState('')
  const [settledOpen, setSettledOpen] = useState(() => readStored<boolean>(SETTLED_OPEN_KEY, false))
  const [menuRequest, setMenuRequest] = useState<MenuRequest | null>(null)
  const narrow = mode === 'narrow'
  const searching = query.trim() !== ''

  useFocusRequest('sidebar', panel)
  useFocusRequest('thread-search', search)

  const sections = useMemo(() => splitThreads(projectSessions.filter((s) => matchesThread(s, query))), [projectSessions, query])
  const { settled } = sections
  const live = [...sections.waiting, ...sections.working, ...sections.idle]
  const settledShown = searching || settledOpen

  const items = [
    ...live.map((s) => ({ key: s.id, session: s })),
    ...(settled.length > 0 ? [{ key: SETTLED_KEY, session: null }] : []),
    ...(settledShown ? settled.map((s) => ({ key: s.id, session: s })) : []),
  ]

  const toggleSettled = () =>
    setSettledOpen((open) => {
      writeStored(SETTLED_OPEN_KEY, !open)
      return !open
    })

  // Opens or closes the Settled row at `index`; `to` limits it to one direction.
  const toggleAt = (index: number, to?: boolean) => {
    const item = items[index]
    if (!item || item.session) return false
    if (to !== undefined && settledShown === to) return false
    toggleSettled()
    return true
  }

  const nav = useListNav(
    items.map((i) => i.key),
    {
      onEnter(index) {
        const session = items[index].session
        if (!session) {
          toggleAt(index)
          return
        }
        if (session.state === 'ended') return
        select(session.id)
        focus('terminal')
      },
      onSpace: (index) => toggleAt(index),
      onLeft: (index) => toggleAt(index, false),
      onRight: (index) => toggleAt(index, true),
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
    if ((e.key === 'ContextMenu' || (e.shiftKey && e.key === 'F10')) && id && id !== SETTLED_KEY) {
      e.preventDefault()
      setMenuRequest((last) => ({ id, n: (last?.n ?? 0) + 1 }))
      return
    }
    if (nav.handle(e)) e.preventDefault()
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
    const first = items.find((i) => i.session && i.session.state !== 'ended')?.session
    if (!first) return
    select(first.id)
    focus('terminal')
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
      <div className="flex flex-none items-center gap-2 border-b border-line px-3 py-2">
        <Icon name="search" />
        <input
          ref={search}
          value={query}
          placeholder="Search threads"
          aria-label="Search threads"
          className="min-w-0 flex-1 bg-transparent py-0.5 outline-none"
          onChange={(e) => setQuery(e.target.value)}
          onKeyDown={onSearchKeyDown}
        />
        <button className="flex-none" title="Command palette" aria-label="Command palette" onClick={() => setOverlay('palette')}>
          <Keycap>⌘K</Keycap>
        </button>
      </div>
      <div ref={list} className="min-h-0 flex-1 overflow-y-auto py-1">
        <ThreadSection label="Attention" sessions={sections.waiting} cursorKey={nav.cursorKey} selectedId={selectedId} menuRequest={menuRequest} />
        <ThreadSection label="Working" sessions={sections.working} cursorKey={nav.cursorKey} selectedId={selectedId} menuRequest={menuRequest} />
        <ThreadSection label="Idle" sessions={sections.idle} cursorKey={nav.cursorKey} selectedId={selectedId} menuRequest={menuRequest} />
        <SettledSection
          sessions={settled}
          open={settledShown}
          cursorKey={nav.cursorKey}
          selectedId={selectedId}
          menuRequest={menuRequest}
          onToggle={() => {
            nav.setCursorKey(SETTLED_KEY)
            toggleSettled()
          }}
        />
        {live.length === 0 && settled.length === 0 && <p className="px-4 py-4 text-small text-dim">{searching ? 'No thread matches.' : 'No threads'}</p>}
      </div>
      <SidebarFooter />
    </aside>
  )
}
