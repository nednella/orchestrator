import { useRef } from 'react'
import type { KeyboardEvent } from 'react'
import { useAgentos, useAgentosSelector } from '../AgentosContext'
import { isTyping } from '../useListNav'
import { useFocusRequest } from '../useFocusRequest'
import { useRefreshQueueOnShow } from '../useRefreshQueueOnShow'
import { HomeComposer } from './HomeComposer'
import { SetupOffer } from './SetupOffer'

export function Home() {
  const { project, focus } = useAgentos('project', 'focus')
  const offersSetup = useAgentosSelector((a) => Boolean(a.project?.needsSetup && !a.project.setupDismissed))
  const panel = useRef<HTMLElement>(null)

  useFocusRequest('home', panel)
  useRefreshQueueOnShow()

  const onKeyDown = (e: KeyboardEvent) => {
    if (isTyping(e.target)) return
    if (e.key !== 'Escape') return
    e.preventDefault()
    focus('composer')
  }

  return (
    <aside ref={panel} data-panel="home" tabIndex={-1} className="panel flex h-full min-h-0 min-w-0 flex-col overflow-hidden" onKeyDown={onKeyDown}>
      <div className="min-h-0 flex-1 overflow-y-auto">
        <div className="mx-auto flex w-full max-w-3xl flex-col pb-6">
          <div className="flex min-w-0 items-baseline gap-3 px-4 pt-5">
            <h1 className="min-w-0 truncate text-title font-semibold">{project?.name}</h1>
            {project?.repo && <span className="mono min-w-0 truncate text-small text-dim">{project.repo}</span>}
          </div>
          {offersSetup && <SetupOffer />}
        </div>
      </div>
      <HomeComposer key={project?.key} />
    </aside>
  )
}
