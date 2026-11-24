import { useAgentos } from '../AgentosContext'
import { splitThreads } from '../threads'
import type { Project, Session } from '../types'
import { Icon } from './Icon'
import { ThreadRow } from './ThreadRow'

export const headerKey = (project: Project) => `project:${project.key}`
export const settledKey = (project: Project) => `settled:${project.key}`

type ProjectGroupProps = {
  project: Project
  sessions: Session[]
  folded: boolean
  settledOpen: boolean
  cursorKey: string | null
  onToggleFold(): void
  onToggleSettled(): void
}

export function ProjectGroup({ project, sessions, folded, settledOpen, cursorKey, onToggleFold, onToggleSettled }: ProjectGroupProps) {
  const { selectedId, showHome, report } = useAgentos('selectedId', 'showHome', 'report')
  const { live, settled } = splitThreads(sessions)
  const needsYou = live.some((s) => s.state === 'waiting')

  const row = (s: Session) => <ThreadRow key={s.id} session={s} selected={s.id === selectedId} cursor={cursorKey === s.id} />

  return (
    <section className="flex flex-col pb-1">
      <div className="row group h-8 items-center pr-1" data-cursor={cursorKey === headerKey(project)}>
        <button className="flex h-full min-w-0 flex-1 items-center gap-1.5 pl-3 text-left" aria-expanded={!folded} onClick={onToggleFold}>
          <span data-chevron className="flex-none text-dim transition-transform duration-150" style={{ transform: folded ? 'rotate(-90deg)' : 'none' }}>
            <Icon name="chevron" size={12} />
          </span>
          <span className="min-w-0 truncate text-body font-semibold">{project.name}</span>
          <span className="mono flex-none text-small text-dim">{live.length}</span>
          {folded && needsYou && <span className="dot" data-state="waiting" title="A thread needs you" />}
        </button>
        <button
          className="btn btn-ghost h-6 w-6 flex-none justify-center px-0"
          title={`${project.name} home`}
          aria-label={`${project.name} home`}
          onClick={() => report(() => showHome(project.key))}
        >
          <Icon name="home" size={13} />
        </button>
      </div>
      {!folded && (
        <>
          {live.map(row)}
          {sessions.length === 0 && <p className="px-3 py-1.5 pl-7 text-small text-dim">No threads yet</p>}
          {settled.length > 0 && (
            <button
              className="row row-inset h-7 items-center gap-1.5 pl-3 text-small text-dim"
              aria-expanded={settledOpen}
              data-cursor={cursorKey === settledKey(project)}
              onClick={onToggleSettled}
            >
              <span data-chevron className="flex-none transition-transform duration-150" style={{ transform: settledOpen ? 'none' : 'rotate(-90deg)' }}>
                <Icon name="chevron" size={12} />
              </span>
              Settled · {settled.length}
            </button>
          )}
          {settledOpen && settled.map(row)}
        </>
      )}
    </section>
  )
}
