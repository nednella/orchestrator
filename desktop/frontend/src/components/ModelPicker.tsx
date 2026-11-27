import { Zap } from 'lucide-react'
import type { KeyboardEvent } from 'react'
import { ACCESSES, ACCESS_HINT, ACCESS_NAME } from '../access'
import { CLAUDE_MODELS, EFFORTS, EFFORT_HINTS, effortsOf, modelShortName, optionsAfterModel, supportsFast } from '../model'
import type { Access, ModelChoice } from '../types'
import { AccessIcon } from './AccessIcon'
import { Icon } from './Icon'
import { SegmentedControl } from './SegmentedControl'
import { Switch } from './Switch'

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

const head = 'flex h-7 items-center gap-2 px-2 text-small font-semibold text-soft'

const ACCESS_OPTIONS = ACCESSES.map((value) => ({ value, label: ACCESS_NAME[value], icon: <AccessIcon access={value} /> }))

// moveFocus moves the focus up or down the options of a column with the arrow keys.
const moveFocus = (e: KeyboardEvent<HTMLDivElement>) => {
  if (e.key !== 'ArrowDown' && e.key !== 'ArrowUp') return
  e.preventDefault()
  const rows = [...e.currentTarget.querySelectorAll<HTMLElement>('button:not(:disabled)')]
  const i = rows.indexOf(document.activeElement as HTMLElement)
  rows[Math.max(0, Math.min(i + (e.key === 'ArrowDown' ? 1 : -1), rows.length - 1))]?.focus()
}

export function ModelPicker({ model, effort, fast, defaults, access, accessOff, onModel, onEffort, onFast, onAccess }: ModelPickerProps) {
  const shownModel = model || defaults.model
  const shownEffort = effort || defaults.effort
  const fixed = !onModel
  const efforts = effortsOf(shownModel)
  // A model without fast mode or ultracode turns them off.
  const pickModel = (id: string) => {
    onModel?.(id === defaults.model ? '' : id)
    const next = optionsAfterModel(id, fast, shownEffort)
    if (next.fast !== fast) onFast?.(next.fast)
    if (next.effort !== shownEffort) onEffort?.(next.effort)
  }
  const defaultTag = <span className="text-label text-dim">Default</span>
  const ids = Object.keys(CLAUDE_MODELS)
  const modelStop = ids.includes(shownModel) ? shownModel : ids[0]
  const effortStop = efforts.includes(shownEffort) ? shownEffort : efforts[0]

  return (
    <div className="-m-1 flex w-[min(28rem,calc(100vw-2rem))] flex-col overflow-hidden rounded-[0.75rem] max-sm:w-72">
      <div className="flex max-sm:flex-col" title={fixed ? 'A running thread keeps the model it started with' : undefined}>
        <div role="radiogroup" aria-label="Model" className="flex min-w-0 flex-1 flex-col gap-0.5 p-1.5" onKeyDown={moveFocus}>
          <p className={head}>Model</p>
          {ids.map((id) => (
            <button
              key={id}
              role="radio"
              aria-checked={id === shownModel}
              tabIndex={id === modelStop ? 0 : -1}
              autoFocus={!fixed && id === modelStop}
              disabled={fixed}
              className="composer-option h-8"
              onClick={() => pickModel(id)}
            >
              <span className="min-w-0 flex-1 truncate">{CLAUDE_MODELS[id].name}</span>
              {id === defaults.model && defaultTag}
              {id === shownModel && <Icon name="check" size={13} />}
            </button>
          ))}
        </div>
        <div role="radiogroup" aria-label="Effort" className="flex w-48 flex-none flex-col gap-0.5 border-l border-line bg-[var(--bg-segment)] p-1.5 max-sm:w-full max-sm:border-t max-sm:border-l-0" onKeyDown={moveFocus}>
          <p className={head}>
            Effort
            <span className="ml-auto min-w-0 truncate text-label font-normal text-dim">{shownModel && modelShortName(shownModel)}</span>
          </p>
          {efforts.map((id) => (
            <button
              key={id}
              role="radio"
              aria-checked={id === shownEffort}
              tabIndex={id === effortStop ? 0 : -1}
              disabled={!onEffort}
              title={EFFORT_HINTS[id]}
              className="composer-option h-8"
              onClick={() => onEffort?.(id === defaults.effort ? '' : id)}
            >
              <span className="min-w-0 flex-1 truncate">{EFFORTS[id]}</span>
              {id === defaults.effort && defaultTag}
              {id === shownEffort && <Icon name="check" size={13} />}
            </button>
          ))}
        </div>
      </div>
      {supportsFast(shownModel) && (
        <div className="flex h-11 items-center gap-2 border-t border-line px-3.5">
          <Zap size={14} aria-hidden="true" className="flex-none text-soft" />
          <span className="min-w-0 flex-1 truncate text-small font-semibold text-soft">Fast mode</span>
          <span className="flex-none text-label text-dim">Faster output</span>
          <Switch label="Fast mode" checked={fast} disabled={!onFast} onChange={(on) => onFast?.(on)} />
        </div>
      )}
      <div className="flex flex-col gap-1.5 border-t border-line p-2">
        <p className={`${head} h-auto px-0`}>Access</p>
        <SegmentedControl label="Access" variant="track" options={ACCESS_OPTIONS} value={access} disabled={Boolean(accessOff)} onChange={onAccess} />
        <p className="min-h-[1.25em] text-small leading-tight text-idle">{accessOff && `${accessOff}.`}</p>
        <p className="min-h-[2.5em] text-small leading-tight text-soft">{ACCESS_HINT[access]}.</p>
      </div>
    </div>
  )
}
