import type { Session } from '../types'
import { ThreadRow } from './ThreadRow'
import type { MenuRequest } from './ThreadSidebar'

type ThreadSectionProps = {
  label: string
  sessions: Session[]
  cursorKey: string | null
  selectedId: string | null
  menuRequest: MenuRequest | null
}

export function ThreadSection({ label, sessions, cursorKey, selectedId, menuRequest }: ThreadSectionProps) {
  if (sessions.length === 0) return null
  return (
    <section className="flex flex-col pb-1">
      <h2 className="label px-3 py-1.5">{label}</h2>
      {sessions.map((s) => (
        <ThreadRow key={s.id} session={s} selected={s.id === selectedId} cursor={cursorKey === s.id} menuRequest={menuRequest?.id === s.id ? menuRequest.n : 0} />
      ))}
    </section>
  )
}
