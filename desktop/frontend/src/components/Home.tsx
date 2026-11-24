import { useRef, useState } from 'react'
import type { KeyboardEvent, MutableRefObject, ReactNode } from 'react'
import { useAgentos, useAgentosSelector } from '../AgentosContext'
import { useLayout } from '../LayoutContext'
import { isTyping } from '../useListNav'
import { useFocusRequest } from '../useFocusRequest'
import { useRefreshQueueOnShow } from '../useRefreshQueueOnShow'
import { HomeComposer } from './HomeComposer'
import { Notes } from './Notes'
import { Queue } from './Queue'
import { SetupOffer } from './SetupOffer'

export type NavHandler = (e: KeyboardEvent) => boolean
export type NavRef = MutableRefObject<NavHandler | null>

type Part = 'queue' | 'notes'

type HomeSectionProps = { part: Part; title: string; count: number; onActivate(part: Part): void; children: ReactNode }

function HomeSection({ part, title, count, onActivate, children }: HomeSectionProps) {
  return (
    <section data-section={part} className="flex flex-col" onFocusCapture={() => onActivate(part)} onPointerDownCapture={() => onActivate(part)}>
      <h2 className="flex items-baseline gap-2 px-4 pt-5 pb-1">
        <span className="label">{title}</span>
        <span className="mono text-small text-dim">{count}</span>
      </h2>
      {children}
    </section>
  )
}

export function Home() {
  const { project, issues, notes, focus } = useAgentos('project', 'issues', 'notes', 'focus')
  const { mode, mobilePanel } = useLayout()
  const offersSetup = useAgentosSelector((a) => Boolean(a.project?.needsSetup && !a.project.setupDismissed))
  const panel = useRef<HTMLElement>(null)
  const body = useRef<HTMLDivElement>(null)
  const navs: Record<Part, NavRef> = { queue: useRef(null), notes: useRef(null) }
  const [part, setPart] = useState<Part>('queue')
  const narrowPart = mode === 'narrow' && (mobilePanel === 'queue' || mobilePanel === 'notes') ? mobilePanel : null

  useFocusRequest('home', panel)
  useRefreshQueueOnShow()

  const onKeyDown = (e: KeyboardEvent) => {
    if (isTyping(e.target)) return
    if (e.key === 'Escape') {
      e.preventDefault()
      focus('composer')
      return
    }
    if (navs[part].current?.(e)) {
      e.preventDefault()
      return
    }
    if (e.metaKey || e.ctrlKey || e.altKey) return
    if (!narrowPart && ['ArrowLeft', 'ArrowRight', 'a', 'd'].includes(e.key)) {
      e.preventDefault()
      setPart(part === 'queue' ? 'notes' : 'queue')
    }
  }

  return (
    <aside ref={panel} data-panel="home" tabIndex={-1} className="panel flex h-full min-h-0 min-w-0 flex-col overflow-hidden" onKeyDown={onKeyDown}>
      <div ref={body} className="min-h-0 flex-1 overflow-y-auto">
        <div className="mx-auto flex w-full max-w-3xl flex-col pb-6">
          <div className="flex min-w-0 items-baseline gap-3 px-4 pt-5">
            <h1 className="min-w-0 truncate text-title font-semibold">{project?.name}</h1>
            {project?.repo && <span className="mono min-w-0 truncate text-small text-dim">{project.repo}</span>}
          </div>
          {offersSetup && <SetupOffer />}
          {narrowPart !== 'notes' && (
            <HomeSection part="queue" title="Queue" count={issues.length} onActivate={setPart}>
              <Queue nav={navs.queue} scroller={body} />
            </HomeSection>
          )}
          {narrowPart !== 'queue' && (
            <HomeSection part="notes" title="Notes" count={notes.filter((n) => !n.archived).length} onActivate={setPart}>
              <Notes nav={navs.notes} />
            </HomeSection>
          )}
        </div>
      </div>
      <HomeComposer key={project?.key} />
    </aside>
  )
}
