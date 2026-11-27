import { activeView, addView, removeView, renameView, selectView, storedViews, updateView } from './listViews'
import type { SavedViews } from './listViews'
import { NO_VIEWS } from './listViews'
import { useProjectStored } from './useProjectStored'
import type { ViewsMenuProps } from './components/ViewsMenu'

type SavedViewsOptions<P> = {
  kind: ViewsMenuProps['kind']
  // storeAs is the name the project's views are kept under.
  storeAs: string
  current: P
  fix(value: unknown): P
  normal(prefs: P): unknown
  // apply puts every field of a view's prefs on the list.
  apply(prefs: P): void
  suggested: string
}

// useSavedViews is the props of the Views menu for a list: the views of the project, which one the list shows, and what
// each choice does to them.
export function useSavedViews<P>({ kind, storeAs, current, fix, normal, apply, suggested }: SavedViewsOptions<P>): ViewsMenuProps {
  const [stored, setStored] = useProjectStored<SavedViews<P>>(storeAs, NO_VIEWS)
  const saved = storedViews(stored, fix)
  const { view, modified } = activeView(saved, current, normal)

  return {
    kind,
    views: saved.views,
    active: view,
    modified,
    suggested,
    onApply: (id) => {
      const picked = saved.views.find((v) => v.id === id)
      if (!picked) return
      setStored(selectView(saved, id))
      apply(picked.prefs)
    },
    onSave: (name) => setStored(addView(saved, crypto.randomUUID(), name, current)),
    onUpdate: () => view && setStored(updateView(saved, view.id, current)),
    onRename: (name) => view && setStored(renameView(saved, view.id, name)),
    onDelete: () => view && setStored(removeView(saved, view.id)),
  }
}
