import { useRef } from 'react'
import type { KeyboardEvent, MutableRefObject } from 'react'
import { useAgentos } from '../AgentosContext'
import type { SidebarTab } from '../AgentosContext'
import { useLayout } from '../LayoutContext'
import { isTyping } from '../useListNav'
import { useFocusRequest } from '../useFocusRequest'
import { useRefreshQueueOnShow } from '../useRefreshQueueOnShow'
import { Notes } from './Notes'
import { Queue } from './Queue'

export type NavHandler = (e: KeyboardEvent) => boolean
export type NavRef = MutableRefObject<NavHandler | null>

type TabButtonProps = { tab: SidebarTab; label: string; count: number }

function TabButton({ tab, label, count }: TabButtonProps) {
  const { sidebarTab } = useAgentos('sidebarTab')
  const { showSidebarTab } = useLayout()
  const active = sidebarTab === tab
  return (
    <button
      role="tab"
      aria-selected={active}
      className="flex h-full items-center gap-2 border-b-2 px-3 text-body font-medium"
      title={`${label} (${count})`}
      style={{ borderColor: active ? 'var(--accent)' : 'transparent', color: active ? 'var(--text)' : 'var(--text-soft)' }}
      onClick={() => showSidebarTab(tab)}
    >
      {label}
      <span className="mono text-small text-dim">{count}</span>
    </button>
  )
}

const TABS: SidebarTab[] = ['queue', 'notes']

export function QueueNotes() {
  const { sidebarTab, issues, notes } = useAgentos('sidebarTab', 'issues', 'notes')
  const { mode, returnToTerminal, showSidebarTab } = useLayout()
  const panel = useRef<HTMLElement>(null)
  const nav: NavRef = useRef(null)
  const narrow = mode === 'narrow'

  useFocusRequest('home', panel)
  useFocusRequest('terminal', panel)
  useRefreshQueueOnShow()

  const onKeyDown = (e: KeyboardEvent) => {
    if (isTyping(e.target)) return
    if (e.key === 'Escape') {
      e.preventDefault()
      returnToTerminal()
      return
    }
    if (nav.current?.(e)) {
      e.preventDefault()
      return
    }
    if (e.metaKey || e.ctrlKey || e.altKey) return
    if (['ArrowLeft', 'ArrowRight', 'a', 'd'].includes(e.key)) {
      e.preventDefault()
      showSidebarTab(TABS[(TABS.indexOf(sidebarTab) + 1) % TABS.length])
    }
  }

  return (
    <aside ref={panel} data-panel="home" data-tab={sidebarTab} tabIndex={-1} className="panel flex h-full min-h-0 min-w-0 flex-col overflow-hidden" onKeyDown={onKeyDown}>
      <div role="tablist" className="panel-head flex h-10 flex-none items-center border-b border-line">
        {narrow ? (
          <span className="label px-4">{sidebarTab}</span>
        ) : (
          <>
            <TabButton tab="queue" label="Queue" count={issues.length} />
            <TabButton tab="notes" label="Notes" count={notes.filter((n) => !n.archived).length} />
          </>
        )}
      </div>
      {sidebarTab === 'queue' ? <Queue nav={nav} /> : <Notes nav={nav} />}
    </aside>
  )
}
