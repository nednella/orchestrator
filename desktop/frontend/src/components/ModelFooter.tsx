import { Workflow, Zap } from 'lucide-react'
import { EFFORT_HINTS, supportsFast } from '../model'
import { effortPick, ultracodeEffort } from '../modelMenu'
import type { Access } from '../types'
import { PermissionsMenu } from './PermissionsMenu'
import { SegmentedControl } from './SegmentedControl'

type ModelFooterProps = {
  model: string
  effort: string
  fast: boolean
  defaultEffort: string
  access: Access
  accessOff: string
  hasUltracode: boolean
  onEffort?(effort: string): void
  onFast?(fast: boolean): void
  onAccess(access: Access): void
}

const SPEEDS = [
  { value: 'standard', label: 'Standard' },
  { value: 'fast', label: 'Fast', icon: <Zap size={12} aria-hidden="true" className="flex-none" /> },
]

export function ModelFooter({ model, effort, fast, defaultEffort, access, accessOff, hasUltracode, onEffort, onFast, onAccess }: ModelFooterProps) {
  const ultracode = effort === 'ultracode'
  return (
    <div className="flex flex-wrap items-center gap-2 border-t border-line bg-[var(--bg-segment)] px-3 py-2">
      {supportsFast(model) && <SegmentedControl label="Speed" variant="track" options={SPEEDS} value={fast ? 'fast' : 'standard'} disabled={!onFast} onChange={(value) => onFast?.(value === 'fast')} />}
      {hasUltracode && (
        <div className="segments-track">
          <button
            aria-pressed={ultracode}
            disabled={!onEffort}
            title={EFFORT_HINTS.ultracode}
            className="inline-flex items-center gap-1.5 disabled:opacity-64"
            onMouseDown={(e) => e.preventDefault()}
            onClick={() => onEffort?.(effortPick(ultracodeEffort(!ultracode), defaultEffort))}
          >
            <Workflow size={12} aria-hidden="true" className="flex-none" />
            Ultracode
          </button>
        </div>
      )}
      <PermissionsMenu access={access} accessOff={accessOff} onAccess={onAccess} />
    </div>
  )
}
