import { CLAUDE_MODELS, effortsOf } from './model.ts'

export type ModelTab = 'favorites' | 'claude'

// The models a tab lists for a search on the name: the starred ones in the Favourites tab, every model in
// the Claude tab with the starred ones first.
export function listedModels(tab: ModelTab, query: string, favorites: string[]): string[] {
  const needle = query.trim().toLowerCase()
  const ids = Object.keys(CLAUDE_MODELS).filter((id) => needle === '' || CLAUDE_MODELS[id].name.toLowerCase().includes(needle))
  const starred = ids.filter((id) => favorites.includes(id))
  if (tab === 'favorites') return starred
  return [...starred, ...ids.filter((id) => !favorites.includes(id))]
}

// The tab the menu opens on: Favourites when a starred model is on offer, else the Claude tab.
export function firstTab(favorites: string[]): ModelTab {
  return favorites.some((id) => id in CLAUDE_MODELS) ? 'favorites' : 'claude'
}

export function toggled(favorites: string[], id: string): string[] {
  return favorites.includes(id) ? favorites.filter((x) => x !== id) : [...favorites, id]
}

// The levels the Reasoning column lists. Ultracode is the toggle beside them, xhigh underneath, so xhigh shows as chosen.
export const reasoningLevels = (model: string) => effortsOf(model).filter((id) => id !== 'ultracode')

export const reasoningChosen = (effort: string) => (effort === 'ultracode' ? 'xhigh' : effort)

// The effort the app saves for a pick: '' for the default, so the issue's labels and the action still choose.
export const effortPick = (id: string, defaultEffort: string) => (id === defaultEffort ? '' : id)

// The effort that turns ultracode on or off.
export const ultracodeEffort = (on: boolean) => (on ? 'ultracode' : 'xhigh')
