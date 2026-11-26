import type { TimelineItem } from './chatTimeline'
import type { Session } from './types'

// forkDividers adds the marks of a fork to the thread's timeline: in a fork, where its chat was cut from its source,
// and in a thread, where each fork of it was cut. A fork whose cut reply is not in the timeline gets no mark, except
// that the fork's own mark goes to the top.
export function forkDividers(s: Session, sessions: Session[], items: TimelineItem[]): TimelineItem[] {
  const forks = sessions.filter((x) => x.forkOf === s.id && x.forkAt)
  if (!s.forkOf && forks.length === 0) return items
  const out: TimelineItem[] = []
  const mark = (): TimelineItem => ({ kind: 'forkedFrom', key: 'forked-from', source: s.forkOf })
  if (s.forkOf && !items.some((i) => 'uuid' in i && i.uuid === s.forkAt && s.forkAt !== '')) out.push(mark())
  for (const item of items) {
    out.push(item)
    const uuid = 'uuid' in item ? item.uuid : ''
    if (!uuid) continue
    if (s.forkOf && uuid === s.forkAt) out.push(mark())
    for (const fork of forks) if (fork.forkAt === uuid) out.push({ kind: 'forked', key: `forked-${fork.id}`, fork: fork.id })
  }
  return out
}
