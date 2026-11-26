// A shell whose start returned exists in Go, yet an announce sent before the start can still come after it. So a
// started shell keeps its pane until an announce lists it, its project goes, or the grace passes with no announce
// listing it.
export const START_GRACE_MS = 5000

export type Started = Map<string, { thread: string; project: string; at: number }>

// stillStarted is the started shells left after an announce of project's shells (listed) at time now.
export function stillStarted(started: Started, project: string, listed: Set<string>, now: number): Started {
  const left: Started = new Map()
  for (const [id, s] of started) if (s.project === project && !listed.has(id) && now - s.at < START_GRACE_MS) left.set(id, s)
  return left
}
