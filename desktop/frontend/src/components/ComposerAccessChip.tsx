import { ACCESS_HINT, ACCESS_LABEL, accessOf, choiceOf } from '../access'
import type { AccessChoice } from '../access'
import type { Access } from '../types'
import { ComposerMenuChip } from './ComposerMenuChip'
import { Icon } from './Icon'
import { SegmentedControl } from './SegmentedControl'

const LEVELS: { value: AccessChoice['level']; label: string }[] = [
  { value: 'read', label: 'Read' },
  { value: 'write', label: 'Write' },
]

const WRITE: { value: AccessChoice['write']; label: string }[] = [
  { value: 'supervised', label: 'Supervised' },
  { value: 'full', label: 'Full access' },
]

// disabled is why the access cannot change now; the chip shows it and does not open.
type ComposerAccessChipProps = { access: Access; disabled?: string; onAccess(access: Access): void }

export function ComposerAccessChip({ access, disabled, onAccess }: ComposerAccessChipProps) {
  const choice = choiceOf(access)
  const pick = (next: Partial<AccessChoice>) => onAccess(accessOf({ ...choice, ...next }))
  return (
    <ComposerMenuChip
      label={`Access: ${ACCESS_LABEL[access]}`}
      disabled={disabled}
      menu={() => (
        <div className="flex w-64 flex-col gap-1.5 p-1">
          <SegmentedControl label="Access" variant="track" options={LEVELS} value={choice.level} onChange={(level) => pick({ level })} />
          {choice.level === 'read' ? (
            <p className="px-1 text-label text-soft">{ACCESS_HINT.read}</p>
          ) : (
            <div role="radiogroup" aria-label="Write access" className="flex flex-col gap-0.5">
              {WRITE.map((w) => (
                <button key={w.value} role="radio" aria-checked={choice.write === w.value} className="composer-option h-auto items-start py-1.5" onClick={() => pick({ write: w.value })}>
                  <span className="flex min-w-0 flex-1 flex-col">
                    <span>{w.label}</span>
                    <span className="text-label text-soft">{ACCESS_HINT[w.value]}</span>
                  </span>
                  {choice.write === w.value && <Icon name="check" size={13} />}
                </button>
              ))}
            </div>
          )}
        </div>
      )}
    >
      <Icon name={access === 'read' ? 'eye' : 'git-branch'} size={14} />
      {ACCESS_LABEL[access]}
    </ComposerMenuChip>
  )
}
