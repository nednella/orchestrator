import { useMemo, useRef, useState } from 'react'
import type { KeyboardEvent } from 'react'
import { useAgentos } from '../AgentosContext'
import { useLayout } from '../LayoutContext'
import { readStored, writeStored } from '../storage'
import { matchesThread, sidebarThreads } from '../threads'
import { useFocusRequest } from '../useFocusRequest'
import { useListNav } from '../useListNav'
import { useScrollCursorIntoView } from '../useScrollCursorIntoView'
import { headerKey, ProjectGroup } from './ProjectGroup'
import { Icon } from './Icon'
import { Keycap } from './Keycap'
import { SETTLED_KEY, SettledSection } from './SettledSection'
import { SidebarFooter } from './SidebarFooter'

const FOLDED_KEY = 'agentos.sidebar.folded'
const SETTLED_OPEN_KEY = 'agentos.sidebar.settled-open'

const toggled = (list: string[], key: string) => (list.includes(key) ? list.filter((k) => k !== key) : [...list, key])

export function ThreadSidebar() {
  const { projects, sessions, selectedId, select, focus, setOverlay } = useAgentos('projects', 'sessions', 'selectedId', 'select', 'focus', 'setOverlay')
  const { mode, widths, returnToTerminal } = useLayout()
  const panel = useRef<HTMLElement>(null)
  const list = useRef<HTMLDivElement>(null)
  const search = useRef<HTMLInputElement>(null)
  const [query, setQuery] = useState('')
  const [folded, setFolded] = useState(() => readStored<string[]>(FOLDED_KEY, []))
  const [settledOpen, setSettledOpen] = useState(() => readStored<boolean>(SETTLED_OPEN_KEY, false))
  const narrow = mode === 'narrow'
  const searching = query.trim() !== ''

  useFocusRequest('sidebar', panel)
  useFocusRequest('thread-search', search)

  const { groups, settled } = useMemo(() => {
    const found = sidebarThreads(
      projects,
      sessions.filter((s) => matchesThread(s, query)),
    )
    return { groups: found.groups.map((g) => ({ ...g, folded: !searching && folded.includes(g.project.key) })), settled: found.settled }
  }, [projects, sessions, query, searching, folded])
  const settledShown = searching || settledOpen

  const items = [
    ...groups.flatMap((g) => [{ key: headerKey(g.project), session: null }, ...(g.folded ? [] : g.sessions.map((s) => ({ key: s.id, session: s })))]),
    ...(settled.length > 0 ? [{ key: SETTLED_KEY, session: null }] : []),
    ...(settledShown ? settled.map((s) => ({ key: s.id, session: s })) : []),
  ]

  const toggleFold = (key: string) =>
    setFolded((current) => {
      const next = toggled(current, key)
      writeStored(FOLDED_KEY, next)
      return next
    })
  const toggleSettled = () =>
    setSettledOpen((open) => {
      writeStored(SETTLED_OPEN_KEY, !open)
      return !open
    })

  // Folds or unfolds the header or Settled row at `index`; `to` limits it to one direction.
  const toggleAt = (index: number, to?: boolean) => {
    const item = items[index]
    if (!item || item.session) return false
    const group = groups.find((g) => headerKey(g.project) === item.key)
    if (to !== undefined && (group ? !group.folded : settledShown) === to) return false
    if (group) toggleFold(group.project.key)
    else toggleSettled()
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
        {groups.map((g) => (
          <ProjectGroup
            key={g.project.key}
            project={g.project}
            sessions={g.sessions}
            folded={g.folded}
            cursorKey={nav.cursorKey}
            onToggleFold={() => {
              nav.setCursorKey(headerKey(g.project))
              toggleFold(g.project.key)
            }}
          />
        ))}
        <SettledSection
          sessions={settled}
          projects={projects}
          open={settledShown}
          cursorKey={nav.cursorKey}
          selectedId={selectedId}
          onToggle={() => {
            nav.setCursorKey(SETTLED_KEY)
            toggleSettled()
          }}
        />
        {groups.length === 0 && settled.length === 0 && <p className="px-4 py-4 text-small text-dim">{searching ? 'No thread matches.' : 'No threads'}</p>}
      </div>
      <SidebarFooter />
    </aside>
  )
}
