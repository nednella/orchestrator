import type { TimelineItem } from './chatTimeline'
import type { Session } from './types'

type Thread = Pick<Session, 'id' | 'forkOf' | 'forkAt'>
type Fork = Thread & Pick<Session, 'title'>

// forksKey is the forks of the thread that forkDividers reads, as text, so that a change in no fork of it is no change.
export function forksKey(id: string, sessions: Session[]): string {
  return JSON.stringify(sessions.filter((x) => x.forkOf === id && x.forkAt).map(({ id, forkOf, forkAt, title }): Fork => ({ id, forkOf, forkAt, title })))
}

// forkDividers adds the marks of a fork to the thread's timeline: in a fork, where its chat was cut from its source,
// and in a thread, where each fork of it was cut. A mark goes after the cut reply and the tool rows that follow it. The
// fork's mark goes to the top when its cut reply is not in the timeline; a fork's mark is left out of the source then.
export function forkDividers(s: Thread, forks: Fork[], items: TimelineItem[]): TimelineItem[] {
  if (!s.forkOf && forks.length === 0) return items
  const at = new Map<string, Fork[]>()
  for (const fork of forks) at.set(fork.forkAt, [...(at.get(fork.forkAt) ?? []), fork])
  const from: TimelineItem = { kind: 'forkedFrom', key: 'forked-from', source: s.forkOf }
  const out: TimelineItem[] = []
  let marks: TimelineItem[] = []
  let found = false
  for (const item of items) {
    if (item.kind !== 'work') {
      out.push(...marks)
      marks = []
    }
    out.push(item)
    const uuid = 'uuid' in item ? item.uuid : ''
    if (!uuid) continue
    if (s.forkOf && uuid === s.forkAt) {
      found = true
      marks.push(from)
    }
    for (const fork of at.get(uuid) ?? []) marks.push({ kind: 'forked', key: `forked-${fork.id}`, fork: fork.id, title: fork.title })
  }
  out.push(...marks)
  return s.forkOf && !found ? [from, ...out] : out
}
