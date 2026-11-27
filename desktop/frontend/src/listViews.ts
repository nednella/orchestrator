// A saved view is a name for the search, filters and sort of one list; a project keeps its own for each list.
export type SavedView<P> = { id: string; name: string; prefs: P }
export type SavedViews<P> = { views: SavedView<P>[]; selected: string }

export const VIEW_NAME_MAX = 40

export const NO_VIEWS: SavedViews<never> = { views: [], selected: '' }

export function viewNameError(name: string, views: { id: string; name: string }[], own = ''): string {
  const text = name.trim()
  if (text === '') return 'Enter a name.'
  if (text.length > VIEW_NAME_MAX) return `Use ${VIEW_NAME_MAX} characters or fewer.`
  if (views.some((v) => v.id !== own && v.name.toLowerCase() === text.toLowerCase())) return 'A view with this name exists.'
  return ''
}

// storedViews reads what was stored, so a value of another shape is no views at all.
export function storedViews<P>(value: unknown, fix: (prefs: unknown) => P): SavedViews<P> {
  const stored = value as Partial<SavedViews<unknown>> | null
  if (!stored || !Array.isArray(stored.views)) return NO_VIEWS
  const views = stored.views
    .filter((v): v is SavedView<unknown> => typeof v?.id === 'string' && typeof v.name === 'string')
    .map((v) => ({ id: v.id, name: v.name, prefs: fix(v.prefs) }))
  return { views, selected: views.some((v) => v.id === stored.selected) ? (stored.selected as string) : '' }
}

export function addView<P>(saved: SavedViews<P>, id: string, name: string, prefs: P): SavedViews<P> {
  return { views: [...saved.views, { id, name: name.trim(), prefs }], selected: id }
}

export function renameView<P>(saved: SavedViews<P>, id: string, name: string): SavedViews<P> {
  return { ...saved, views: saved.views.map((v) => (v.id === id ? { ...v, name: name.trim() } : v)) }
}

export function updateView<P>(saved: SavedViews<P>, id: string, prefs: P): SavedViews<P> {
  return { ...saved, views: saved.views.map((v) => (v.id === id ? { ...v, prefs } : v)) }
}

export function removeView<P>(saved: SavedViews<P>, id: string): SavedViews<P> {
  return { views: saved.views.filter((v) => v.id !== id), selected: saved.selected === id ? '' : saved.selected }
}

export function selectView<P>(saved: SavedViews<P>, id: string): SavedViews<P> {
  return { ...saved, selected: id }
}

// activeView is the view the list shows: the one picked, else the first whose prefs the list has now. modified says the
// list has moved off it.
export function activeView<P>(saved: SavedViews<P>, current: P, normal: (prefs: P) => unknown): { view: SavedView<P> | null; modified: boolean } {
  const key = JSON.stringify(normal(current))
  const picked = saved.views.find((v) => v.id === saved.selected)
  if (picked) return { view: picked, modified: JSON.stringify(normal(picked.prefs)) !== key }
  const same = saved.views.find((v) => JSON.stringify(normal(v.prefs)) === key)
  return { view: same ?? null, modified: false }
}
