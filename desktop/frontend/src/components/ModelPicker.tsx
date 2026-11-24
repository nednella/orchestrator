import { useRef, useState } from 'react'
import type { KeyboardEvent } from 'react'
import { CLAUDE_MODELS, EFFORTS, modelShortName } from '../model'
import type { ModelChoice } from '../types'
import { Icon } from './Icon'

// model and effort are the user's picks, '' until they pick; defaults are what '' resolves to, its effort for the
// model shown. Picking the default goes back to '' so the issue's labels and the action still choose.
type ModelPickerProps = {
  model: string
  effort: string
  defaults: ModelChoice
  onModel(model: string): void
  onEffort(effort: string): void
  onDone(): void
}

const head = 'flex h-8 items-center gap-2 border-b border-line text-small'

export function ModelPicker({ model, effort, defaults, onModel, onEffort, onDone }: ModelPickerProps) {
  const shownModel = model || defaults.model
  const shownEffort = effort || defaults.effort
  const [query, setQuery] = useState('')
  const [more, setMore] = useState(() => CLAUDE_MODELS[shownModel]?.more === true)
  const needle = query.trim().toLowerCase()
  const ids = Object.keys(CLAUDE_MODELS).filter((id) =>
    needle ? id.includes(needle) || CLAUDE_MODELS[id].name.toLowerCase().includes(needle) : more || !CLAUDE_MODELS[id].more,
  )
  const [highlight, setHighlight] = useState(() => Math.max(ids.indexOf(shownModel), 0))
  const at = Math.min(highlight, ids.length - 1)
  const levels = useRef<HTMLDivElement>(null)
  const search = useRef<HTMLInputElement>(null)

  const pickModel = (id: string) => {
    onModel(id === defaults.model ? '' : id)
    onDone()
  }

  const focusLevels = () => (levels.current?.querySelector<HTMLElement>('[aria-checked="true"]') ?? levels.current?.querySelector<HTMLElement>('button'))?.focus()

  const onSearchKey = (e: KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
      e.preventDefault()
      setHighlight(Math.max(0, Math.min(at + (e.key === 'ArrowDown' ? 1 : -1), ids.length - 1)))
    } else if (e.key === 'Enter' && ids[at]) {
      e.preventDefault()
      pickModel(ids[at])
    } else if (e.key === 'ArrowRight' && e.currentTarget.selectionStart === query.length) {
      e.preventDefault()
      focusLevels()
    }
  }

  const onLevelsKey = (e: KeyboardEvent<HTMLDivElement>) => {
    const rows = [...e.currentTarget.querySelectorAll<HTMLElement>('button')]
    const i = rows.indexOf(document.activeElement as HTMLElement)
    if (e.key === 'ArrowLeft') {
      e.preventDefault()
      search.current?.focus()
    } else if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
      e.preventDefault()
      rows[Math.max(0, Math.min(i + (e.key === 'ArrowDown' ? 1 : -1), rows.length - 1))]?.focus()
    }
  }

  const defaultTag = <span className="text-label text-dim">Default</span>
  const tabStop = shownEffort in EFFORTS ? shownEffort : Object.keys(EFFORTS)[0]

  return (
    <div className="-m-1 flex w-[min(26rem,calc(100vw-2rem))] overflow-hidden rounded-[0.75rem] max-sm:w-72 max-sm:flex-col">
      <div className="flex min-w-0 flex-1 flex-col">
        <div className="px-3 pt-1.5">
          <label className={`${head} focus-within:border-line-strong`}>
            <Icon name="search" size={13} />
            <input
              ref={search}
              autoFocus
              value={query}
              placeholder="Search models…"
              aria-label="Search models"
              className="min-w-0 flex-1 bg-transparent outline-none"
              onMouseDown={(e) => e.stopPropagation()}
              onChange={(e) => {
                setQuery(e.target.value)
                setHighlight(0)
              }}
              onKeyDown={onSearchKey}
            />
          </label>
        </div>
        <div role="listbox" aria-label="Models" className="flex flex-col gap-0.5 p-1.5">
          {ids.map((id, i) => (
            <button
              key={id}
              role="option"
              aria-selected={id === shownModel}
              tabIndex={-1}
              className="composer-option h-8"
              data-selected={i === at}
              onMouseEnter={() => setHighlight(i)}
              onClick={() => pickModel(id)}
            >
              <span className="min-w-0 flex-1 truncate">{CLAUDE_MODELS[id].name}</span>
              {id === defaults.model && defaultTag}
              {id === shownModel && <Icon name="check" size={13} />}
            </button>
          ))}
          {ids.length === 0 && <p className="px-2 py-1.5 text-small text-dim">No models found</p>}
          {!needle && (
            <button tabIndex={-1} className="composer-option h-8 text-soft hover:text-ink" onClick={() => setMore(!more)}>
              {more ? 'Show less' : 'Show more'}
            </button>
          )}
        </div>
      </div>
      <div className="flex w-44 flex-none flex-col border-l border-line max-sm:w-full max-sm:border-t max-sm:border-l-0">
        <div className="px-3 pt-1.5">
          <div className={head}>
            <span className="text-soft">Reasoning</span>
            <span className="ml-auto min-w-0 truncate text-label text-dim">{shownModel && modelShortName(shownModel)}</span>
          </div>
        </div>
        <div ref={levels} role="radiogroup" aria-label="Reasoning" className="flex flex-col gap-0.5 p-1.5" onKeyDown={onLevelsKey}>
          {Object.entries(EFFORTS).map(([id, name]) => (
            <button
              key={id}
              role="radio"
              aria-checked={id === shownEffort}
              tabIndex={id === tabStop ? 0 : -1}
              className="composer-option h-8"
              onClick={() => {
                onEffort(id === defaults.effort ? '' : id)
                onDone()
              }}
            >
              <span className="min-w-0 flex-1 truncate">{name}</span>
              {id === defaults.effort && defaultTag}
              {id === shownEffort && <Icon name="check" size={13} />}
            </button>
          ))}
        </div>
      </div>
    </div>
  )
}
