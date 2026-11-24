import { useMemo, useRef, useState } from 'react'
import type { KeyboardEvent } from 'react'
import { useAgentos } from '../AgentosContext'
import { useLayout } from '../LayoutContext'
import { readStored, writeStored } from '../storage'
import { matchesThread, splitThreads } from '../threads'
import { useFocusRequest } from '../useFocusRequest'
import { useListNav } from '../useListNav'
import { useScrollCursorIntoView } from '../useScrollCursorIntoView'
import { headerKey, ProjectGroup, settledKey } from './ProjectGroup'
import { Icon } from './Icon'
import { Keycap } from './Keycap'
import { NewSessionRow } from './NewSessionRow'
import { SidebarFooter } from './SidebarFooter'

const FOLDED_KEY = 'agentos.sidebar.folded'

const toggled = (list: string[], key: string) => (list.includes(key) ? list.filter((k) => k !== key) : [...list, key])

export function ThreadSidebar() {
  const { projects, sessions, selectedId, select, focus, setOverlay } = useAgentos('projects', 'sessions', 'selectedId', 'select', 'focus', 'setOverlay')
  const { mode, widths, returnToTerminal } = useLayout()
  const panel = useRef<HTMLElement>(null)
  const list = useRef<HTMLDivElement>(null)
  const search = useRef<HTMLInputElement>(null)
  const [query, setQuery] = useState('')
  const [folded, setFolded] = useState(() => readStored<string[]>(FOLDED_KEY, []))
  const [settledOpen, setSettledOpen] = useState<string[]>([])
  const narrow = mode === 'narrow'
  const searching = query.trim() !== ''

  useFocusRequest('sidebar', panel)
  useFocusRequest('thread-search', search)

  const groups = useMemo(
    () =>
      projects
        .map((project) => {
          const own = sessions.filter((s) => s.project === project.key && matchesThread(s, query))
          return { project, sessions: own, folded: !searching && folded.includes(project.key), settledOpen: searching || settledOpen.includes(project.key) }
        })
        .filter((g) => !searching || g.sessions.length > 0),
    [projects, sessions, query, searching, folded, settledOpen],
  )

  const items = groups.flatMap((g) => {
    const { live, settled } = splitThreads(g.sessions)
    if (g.folded) return [{ key: headerKey(g.project), session: null }]
    return [
      { key: headerKey(g.project), session: null },
      ...live.map((s) => ({ key: s.id, session: s })),
      ...(settled.length > 0 ? [{ key: settledKey(g.project), session: null }] : []),
      ...(g.settledOpen ? settled.map((s) => ({ key: s.id, session: s })) : []),
    ]
  })

  const toggleFold = (key: string) =>
    setFolded((current) => {
      const next = toggled(current, key)
      writeStored(FOLDED_KEY, next)
      return next
    })
  const toggleSettled = (key: string) => setSettledOpen((current) => toggled(current, key))

  const groupOf = (index: number) => groups.find((g) => items[index]?.key === headerKey(g.project) || items[index]?.key === settledKey(g.project))

  // Folds or unfolds the header or Settled row at `index`; `to` limits it to one direction.
  const toggleAt = (index: number, to?: boolean) => {
    const group = groupOf(index)
    if (!group || items[index].session) return false
    const header = items[index].key === headerKey(group.project)
    if (to !== undefined && (header ? !group.folded : group.settledOpen) === to) return false
    if (header) toggleFold(group.project.key)
    else toggleSettled(group.project.key)
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
            settledOpen={g.settledOpen}
            cursorKey={nav.cursorKey}
            onToggleFold={() => {
              nav.setCursorKey(headerKey(g.project))
              toggleFold(g.project.key)
            }}
            onToggleSettled={() => {
              nav.setCursorKey(settledKey(g.project))
              toggleSettled(g.project.key)
            }}
          />
        ))}
        {searching && groups.length === 0 && <p className="px-4 py-4 text-small text-dim">No thread matches.</p>}
      </div>
      <NewSessionRow cursor={false} compact={false} />
      <SidebarFooter />
    </aside>
  )
}
