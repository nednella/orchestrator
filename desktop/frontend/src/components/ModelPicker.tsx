import { useRef, useState } from 'react'
import type { KeyboardEvent } from 'react'
import { effortsOf, optionsAfterModel } from '../model'
import { effortPick, firstTab, listedModels } from '../modelMenu'
import type { ModelTab } from '../modelMenu'
import type { Access, ModelChoice } from '../types'
import { useFavoriteModels } from '../useFavoriteModels'
import { useModelJump } from '../useModelJump'
import { ModelChoices } from './ModelChoices'
import { ModelFooter } from './ModelFooter'
import { ModelRail } from './ModelRail'
import { ModelReasoning } from './ModelReasoning'

// model and effort are the user's picks, '' until they pick; defaults are what '' resolves to, its effort for the
// model shown. Picking the default goes back to '' so the issue's labels and the action still choose. Without onModel
// the model is a running thread's, shown and not changed; without onEffort or onFast, so are the effort and fast mode.
// accessOff is why the access cannot change now, '' when it can.
type ModelPickerProps = {
  model: string
  effort: string
  fast: boolean
  defaults: ModelChoice
  access: Access
  accessOff: string
  onModel?(model: string): void
  onEffort?(effort: string): void
  onFast?(fast: boolean): void
  onAccess(access: Access): void
}

const FIXED = 'A running thread keeps the model it started with'

export function ModelPicker({ model, effort, fast, defaults, access, accessOff, onModel, onEffort, onFast, onAccess }: ModelPickerProps) {
  const [favorites, toggleFavorite] = useFavoriteModels()
  const [tab, setTab] = useState<ModelTab>(() => firstTab(favorites))
  const [query, setQuery] = useState('')
  const [highlight, setHighlight] = useState('')
  const root = useRef<HTMLDivElement>(null)
  const search = useRef<HTMLInputElement>(null)
  const shownModel = model || defaults.model
  const shownEffort = effort || defaults.effort
  const fixed = !onModel
  const list = listedModels(tab, query, favorites)
  const highlighted = list.includes(highlight) ? highlight : list.includes(shownModel) ? shownModel : (list[0] ?? '')

  // A model without fast mode or ultracode turns them off.
  const pickModel = (id: string) => {
    onModel?.(id === defaults.model ? '' : id)
    const next = optionsAfterModel(id, fast, shownEffort)
    if (next.fast !== fast) onFast?.(next.fast)
    if (next.effort !== shownEffort) onEffort?.(next.effort)
  }
  useModelJump((number) => {
    const id = list[number - 1]
    if (id && !fixed) pickModel(id)
  })

  const focus = (selector: string) => root.current?.querySelector<HTMLElement>(selector)?.focus()
  const onSearchKey = (e: KeyboardEvent<HTMLInputElement>) => {
    if (e.altKey || e.ctrlKey || e.metaKey || e.shiftKey) return
    const atEnd = e.currentTarget.selectionStart === query.length
    if (e.key === 'ArrowLeft' && query === '') {
      e.preventDefault()
      focus('[data-model-rail] [aria-pressed="true"]')
    } else if (e.key === 'ArrowRight' && atEnd) {
      e.preventDefault()
      if (!fixed && highlighted && highlighted !== shownModel) pickModel(highlighted)
      focus('[data-effort-level][aria-checked="true"]:not(:disabled)')
    }
  }

  return (
    <div ref={root} className="-m-1 flex w-[min(34rem,calc(100vw-2rem))] flex-col overflow-hidden rounded-[0.75rem]">
      <div className="flex h-72 max-sm:h-auto max-sm:flex-col">
        <ModelRail tab={tab} onTab={setTab} onSearch={() => search.current?.focus()} />
        <ModelChoices
          tab={tab}
          query={query}
          list={list}
          highlighted={highlighted}
          selected={shownModel}
          favorites={favorites}
          off={fixed ? FIXED : ''}
          searchRef={search}
          onQuery={setQuery}
          onHighlight={setHighlight}
          onPick={pickModel}
          onStar={toggleFavorite}
          onKey={onSearchKey}
        />
        <ModelReasoning model={shownModel} effort={shownEffort} defaultEffort={defaults.effort} disabled={!onEffort} onPick={(id) => onEffort?.(effortPick(id, defaults.effort))} onBack={() => search.current?.focus()} />
      </div>
      <ModelFooter
        model={shownModel}
        effort={shownEffort}
        fast={fast}
        defaultEffort={defaults.effort}
        access={access}
        accessOff={accessOff}
        hasUltracode={effortsOf(shownModel).includes('ultracode')}
        onEffort={onEffort}
        onFast={onFast}
        onAccess={onAccess}
      />
    </div>
  )
}
