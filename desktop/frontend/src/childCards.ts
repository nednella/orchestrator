import type { Access, ChatEvent, Session, State } from './types'

// ChildFacts is the live line of a child's card: what the sessions list says of the child now. A child gone from the
// list keeps the title its card was made with, and says no more.
export type ChildFacts = { title: string; project: string; access: Access | ''; where: string; state: State | 'gone' }

// childFacts is the card's live line for the child. where is the branch of the child's own worktree, or the worktree's
// folder before it has one, else Folder: the child shares a folder.
export function childFacts(child: Session | undefined, fallbackTitle: string): ChildFacts {
  if (!child) return { title: fallbackTitle, project: '', access: '', where: '', state: 'gone' }
  const where = child.worktree ? child.branch || (child.worktree.split('/').at(-1) ?? '') : 'Folder'
  return { title: child.title || fallbackTitle, project: child.project, access: child.access, where, state: child.state }
}

// approvalInput is the input of the tool the approval with the request id asks to run, from the thread's log; undefined
// until the log holds it.
export function approvalInput(events: ChatEvent[], request: string): unknown {
  return events.findLast((e) => e.kind === 'approval' && e.id === request)?.input
}
