import { Check, CircleDashed, X } from 'lucide-react'
import type { PR } from '../types'

type ChecksMarkProps = { checks: PR['checks'] }

const LABEL: Record<PR['checks'], string> = {
  none: '',
  pending: 'checks running',
  passing: 'checks passing',
  failing: 'checks failing',
}

const COLOR: Record<PR['checks'], string> = {
  none: 'currentColor',
  pending: 'var(--text-dim)',
  passing: 'var(--finished)',
  failing: 'var(--danger)',
}

const ICON = { none: CircleDashed, pending: CircleDashed, passing: Check, failing: X }

export function ChecksMark({ checks }: ChecksMarkProps) {
  if (checks === 'none') return null
  const Shape = ICON[checks]
  return (
    <span className="flex flex-none" role="img" aria-label={LABEL[checks]} title={LABEL[checks]}>
      <Shape size={14} strokeWidth={2.25} color={COLOR[checks]} aria-hidden="true" />
    </span>
  )
}
