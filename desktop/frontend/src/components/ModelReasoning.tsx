import { Check } from 'lucide-react'
import type { KeyboardEvent } from 'react'
import { EFFORTS, EFFORT_HINTS, modelShortName } from '../model'
import { reasoningChosen, reasoningLevels } from '../modelMenu'
import { moveFocus } from './moveFocus'

type ModelReasoningProps = {
  model: string
  effort: string
  defaultEffort: string
  disabled: boolean
  onPick(effort: string): void
  onBack(): void
}

// The first level whose name starts with the typed letter, after the focused one.
function levelStarting(e: KeyboardEvent<HTMLElement>): HTMLElement | undefined {
  if (e.key.length !== 1 || e.key.trim() === '' || e.ctrlKey || e.altKey || e.metaKey) return undefined
  const rows = [...e.currentTarget.querySelectorAll<HTMLElement>('[data-effort-level]:not(:disabled)')]
  const i = rows.indexOf(document.activeElement as HTMLElement)
  return [...rows.slice(i + 1), ...rows.slice(0, i + 1)].find((row) => row.dataset.effortLevel?.toLowerCase().startsWith(e.key.toLowerCase()))
}

export function ModelReasoning({ model, effort, defaultEffort, disabled, onPick, onBack }: ModelReasoningProps) {
  const levels = reasoningLevels(model)
  const chosen = levels.includes(reasoningChosen(effort)) ? reasoningChosen(effort) : levels[0]
  const onKeyDown = (e: KeyboardEvent<HTMLElement>) => {
    if (e.key === 'ArrowLeft') {
      e.preventDefault()
      onBack()
    } else if (!moveFocus(e, '[data-effort-level]')) levelStarting(e)?.focus()
  }
  return (
    <div className="flex w-48 flex-none flex-col border-l border-line max-sm:w-full max-sm:border-t max-sm:border-l-0">
      <div className="flex-none px-3 pt-2.5">
        <div className="flex h-8 items-center gap-2 border-b border-line pb-1.5 text-body">
          <span className="text-soft">Reasoning</span>
          <span className="ml-auto min-w-0 truncate text-label text-soft">{model && modelShortName(model)}</span>
        </div>
      </div>
      <div role="radiogroup" aria-label="Reasoning" className="flex min-h-0 flex-1 flex-col gap-0.5 overflow-y-auto px-2 pt-2.5 pb-1.5" onKeyDown={onKeyDown}>
        {levels.map((id) => (
          <button
            key={id}
            role="radio"
            aria-checked={id === chosen}
            data-effort-level={EFFORTS[id]}
            tabIndex={id === chosen ? 0 : -1}
            disabled={disabled}
            title={EFFORT_HINTS[id]}
            className="flex h-8 w-full flex-none items-center gap-2 rounded-md px-2 text-left text-body hover:bg-[var(--bg-row-hover)] focus-visible:bg-[var(--bg-row-hover)] focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-64"
            onClick={() => onPick(id)}
          >
            <span className="min-w-0 flex-1 truncate">{EFFORTS[id]}</span>
            {id === chosen ? (
              <>
                {id === defaultEffort && <span className="sr-only">Default</span>}
                <Check size={13} aria-hidden="true" />
              </>
            ) : (
              id === defaultEffort && <span className="text-label text-dim">Default</span>
            )}
          </button>
        ))}
      </div>
    </div>
  )
}
