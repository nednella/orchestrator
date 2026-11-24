import type { Project, Session } from '../types'
import { Icon } from './Icon'
import { ThreadRow } from './ThreadRow'

export const SETTLED_KEY = 'settled'

type SettledSectionProps = {
  sessions: Session[]
  projects: Project[]
  open: boolean
  cursorKey: string | null
  selectedId: string | null
  onToggle(): void
}

export function SettledSection({ sessions, projects, open, cursorKey, selectedId, onToggle }: SettledSectionProps) {
  if (sessions.length === 0) return null
  const nameOf = (key: string) => projects.find((p) => p.key === key)?.name

  return (
    <section className="flex flex-col pb-1">
      <button className="row row-inset h-7 items-center gap-1.5 pl-3 text-small text-dim" aria-expanded={open} data-cursor={cursorKey === SETTLED_KEY} onClick={onToggle}>
        <span data-chevron className="flex-none transition-transform duration-150" style={{ transform: open ? 'none' : 'rotate(-90deg)' }}>
          <Icon name="chevron" size={12} />
        </span>
        Settled · {sessions.length}
      </button>
      {open && sessions.map((s) => <ThreadRow key={s.id} session={s} projectName={nameOf(s.project)} selected={s.id === selectedId} cursor={cursorKey === s.id} />)}
    </section>
  )
}
