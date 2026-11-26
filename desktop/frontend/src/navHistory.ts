// View is what the main view shows: a project's home, a thread's chat, or the view of a subagent Claude started in a
// thread's chat, in place of that chat.
export type View = { kind: 'home'; project: string } | { kind: 'thread'; id: string } | { kind: 'agent'; thread: string; agent: string }

// History is the views the owner went through, oldest first, and the one shown now, which Back and Forward move.
export type History = { views: View[]; at: number }

const MOST = 50

export const NO_HISTORY: History = { views: [], at: -1 }

function same(a: View | undefined, b: View): boolean {
  if (a?.kind !== b.kind) return false
  if (b.kind === 'home') return a.kind === 'home' && a.project === b.project
  if (b.kind === 'thread') return a.kind === 'thread' && a.id === b.id
  return a.kind === 'agent' && a.thread === b.thread && a.agent === b.agent
}

export const current = (h: History): View | undefined => h.views[h.at]

// push shows the view next: the views after the one shown, left by Back, go, and so do the oldest past fifty.
export function push(h: History, view: View): History {
  if (same(current(h), view)) return h
  const views = [...h.views.slice(0, h.at + 1), view].slice(-MOST)
  return { views, at: views.length - 1 }
}

export const back = (h: History): History => (h.at > 0 ? { ...h, at: h.at - 1 } : h)

export const forward = (h: History): History => (h.at < h.views.length - 1 ? { ...h, at: h.at + 1 } : h)

// stepTo is the history at the first view step reaches whose thread or project is still there, h itself when there is
// none.
export function stepTo(h: History, step: (h: History) => History, live: (view: View) => boolean): History {
  for (let from = h, next = step(h); next !== from; from = next, next = step(next)) {
    if (live(current(next)!)) return next
  }
  return h
}

// prune drops the views whose thread or project is gone, so they take no room among the fifty. Neighbours that become
// the same view merge, and the view shown is the nearest one kept at or before it.
export function prune(h: History, live: (view: View) => boolean): History {
  const views: View[] = []
  let at = -1
  h.views.forEach((view, i) => {
    if (!live(view)) return
    if (!same(views[views.length - 1], view)) views.push(view)
    if (i <= h.at) at = views.length - 1
  })
  if (views.length === h.views.length) return h
  return { views, at: at < 0 && views.length > 0 ? 0 : at }
}

// threadOf is the thread the view shows, '' for a home.
export const threadOf = (view: View): string => (view.kind === 'thread' ? view.id : view.kind === 'agent' ? view.thread : '')

// viewedAgent is the subagent whose view shows in place of the thread's chat, null while the thread shows its chat or is
// not on screen.
export function viewedAgent(h: History, thread: string): string | null {
  const view = current(h)
  return view?.kind === 'agent' && view.thread === thread ? view.agent : null
}

// closeAgent takes the thread from its subagent's view back to its chat, which takes the view's place, so Back does not
// open a view that closed. Another thread's view is left as it is.
export function closeAgent(h: History, thread: string): History {
  if (viewedAgent(h, thread) === null) return h
  const chat: View = { kind: 'thread', id: thread }
  if (same(h.views[h.at - 1], chat)) return { views: h.views.filter((_, i) => i !== h.at), at: h.at - 1 }
  return { views: h.views.map((v, i) => (i === h.at ? chat : v)), at: h.at }
}

// goneAgent closes the thread's subagent view once its loaded chat no longer holds the subagent.
export const goneAgent = (h: History, thread: string, loaded: boolean, holds: boolean): History => (loaded && !holds ? closeAgent(h, thread) : h)
