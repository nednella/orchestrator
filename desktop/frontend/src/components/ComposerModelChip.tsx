import { ComposerMenuChip } from './ComposerMenuChip'
import { Icon } from './Icon'

const MODELS = ['best', 'fable', 'opus', 'sonnet', 'haiku', 'opusplan']
const EFFORTS = ['low', 'medium', 'high', 'xhigh', 'max']

const NAMES: Record<string, string> = { '': 'Default', opusplan: 'Opus plan', xhigh: 'Extra high' }

const named = (value: string) => NAMES[value] ?? value[0].toUpperCase() + value.slice(1)

type ComposerModelChipProps = {
  model: string
  effort: string
  onModel(model: string): void
  onEffort(effort: string): void
}

type ChoicesProps = { title: string; values: string[]; value: string; onPick(value: string): void }

function Choices({ title, values, value, onPick }: ChoicesProps) {
  return (
    <div role="group" aria-label={title} className="flex flex-col gap-0.5">
      <div className="px-2 pt-1 pb-0.5 text-label text-dim">{title}</div>
      {['', ...values].map((v) => (
        <button key={v} role="menuitemradio" aria-checked={v === value} className="composer-option" onClick={() => onPick(v)}>
          <span className="min-w-0 flex-1 truncate">{named(v)}</span>
          {v === value && <Icon name="check" size={13} />}
        </button>
      ))}
    </div>
  )
}

export function ComposerModelChip({ model, effort, onModel, onEffort }: ComposerModelChipProps) {
  return (
    <ComposerMenuChip
      label={`Model ${named(model)}, effort ${named(effort)}`}
      menu={(close) => (
        <div className="grid w-72 grid-cols-2 gap-1">
          <Choices
            title="Model"
            values={MODELS}
            value={model}
            onPick={(v) => {
              onModel(v)
              close()
            }}
          />
          <div className="flex flex-col">
            <Choices
              title="Effort"
              values={EFFORTS}
              value={effort}
              onPick={(v) => {
                onEffort(v)
                close()
              }}
            />
            <p className="px-2 pt-1.5 pb-1 text-label text-dim">More effort thinks longer. Default lets the issue or Claude’s settings choose.</p>
          </div>
        </div>
      )}
    >
      <span className="max-w-24 truncate">{named(model)}</span>
      {effort && <span className="max-w-24 truncate text-label text-soft">{named(effort)}</span>}
    </ComposerMenuChip>
  )
}
