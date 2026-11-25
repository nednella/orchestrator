import { PLACE_HINT, PLACE_LABEL } from '../place'
import type { Place } from '../types'
import { ComposerMenuChip } from './ComposerMenuChip'
import { Icon } from './Icon'

const PLACES: Place[] = ['worktree', 'project']

// disabled is why the place cannot change now; the chip shows it and does not open.
type ComposerPlaceChipProps = { place: Place; disabled?: string; onPlace(place: Place): void }

export function ComposerPlaceChip({ place, disabled, onPlace }: ComposerPlaceChipProps) {
  return (
    <ComposerMenuChip
      label={`Runs in: ${PLACE_LABEL[place]}`}
      disabled={disabled}
      menu={(close) => (
        <div role="radiogroup" aria-label="Runs in" className="flex w-64 flex-col gap-0.5 p-1">
          {PLACES.map((p) => (
            <button
              key={p}
              role="radio"
              aria-checked={place === p}
              className="composer-option h-auto items-start py-1.5"
              onClick={() => {
                onPlace(p)
                close()
              }}
            >
              <span className="flex min-w-0 flex-1 flex-col">
                <span>{PLACE_LABEL[p]}</span>
                <span className="text-label text-soft">{PLACE_HINT[p]}</span>
              </span>
              {place === p && <Icon name="check" size={13} />}
            </button>
          ))}
        </div>
      )}
    >
      <Icon name={place === 'worktree' ? 'git-branch' : 'folder'} size={14} />
      {PLACE_LABEL[place]}
    </ComposerMenuChip>
  )
}
