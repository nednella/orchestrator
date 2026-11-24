import { useRef } from 'react'
import type { KeyboardEvent, MutableRefObject } from 'react'
import { useAgentos, useAgentosSelector } from '../AgentosContext'
import { isTyping } from '../useListNav'
import { useFocusRequest } from '../useFocusRequest'
import { useRefreshQueueOnShow } from '../useRefreshQueueOnShow'
import { HomeComposer } from './HomeComposer'
import { Queue } from './Queue'
import { SetupOffer } from './SetupOffer'

export type NavHandler = (e: KeyboardEvent) => boolean
export type NavRef = MutableRefObject<NavHandler | null>

export function Home() {
  const { project, issues, focus } = useAgentos('project', 'issues', 'focus')
  const offersSetup = useAgentosSelector((a) => Boolean(a.project?.needsSetup && !a.project.setupDismissed))
  const panel = useRef<HTMLElement>(null)
  const body = useRef<HTMLDivElement>(null)
  const nav: NavRef = useRef(null)

  useFocusRequest('home', panel)
  useRefreshQueueOnShow()

  const onKeyDown = (e: KeyboardEvent) => {
    if (isTyping(e.target)) return
    if (e.key === 'Escape') {
      e.preventDefault()
      focus('composer')
      return
    }
    if (nav.current?.(e)) e.preventDefault()
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
          <section className="flex flex-col">
            <h2 className="flex items-baseline gap-2 px-4 pt-5 pb-1">
              <span className="label">Queue</span>
              <span className="mono text-small text-dim">{issues.length}</span>
            </h2>
            <Queue nav={nav} scroller={body} />
          </section>
        </div>
      </div>
      <HomeComposer key={project?.key} />
    </aside>
  )
}
