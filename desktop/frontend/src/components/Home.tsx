import type { KeyboardEvent } from 'react'
import { useAgentos } from '../AgentosContext'
import { isTyping } from '../useListNav'
import { useRefreshQueueOnShow } from '../useRefreshQueueOnShow'
import { HomeComposer } from './HomeComposer'

export function Home() {
  const { project, focus } = useAgentos('project', 'focus')

  useRefreshQueueOnShow()

  const onKeyDown = (e: KeyboardEvent) => {
    if (e.key !== 'Escape' || isTyping(e.target)) return
    e.preventDefault()
    focus('composer')
  }

  return (
    <aside data-panel="home" tabIndex={-1} className="panel flex h-full min-h-0 min-w-0 flex-col overflow-y-auto" onKeyDown={onKeyDown}>
      <div className="m-auto flex w-full max-w-(--chat-column) flex-col px-4 pt-6 pb-[12vh]">
        <h1 className="mb-5 text-center text-[1.375rem] font-semibold text-balance">{project ? `What should we build in ${project.name}?` : 'What should we build?'}</h1>
        <HomeComposer key={project?.key} />
      </div>
    </aside>
  )
}
