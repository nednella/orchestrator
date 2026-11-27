// Deleting is the threads whose delete has started and not ended, by the id the delete was asked for, each with the ids
// of its family: the thread, its descendants and its side chats. Their rows leave the list at once, but what the app
// keeps of them, a draft and a panel, goes only once the delete is done: a refused one brings the rows back.
export type Deleting = Map<string, string[]>

// forgettable is the threads of gone that nothing keeps for a delete that runs.
export function forgettable(gone: string[], deleting: Deleting): string[] {
  const held = new Set([...deleting.values()].flat())
  return gone.filter((id) => !held.has(id))
}
