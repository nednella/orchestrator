import { CircleAlert, CircleCheck, CircleDashed, LoaderCircle } from 'lucide-react'
import { STATE_LABEL } from '../stateMeta'
import type { State } from '../types'

type StateIconProps = { state: State }

const ICON = { waiting: CircleAlert, working: LoaderCircle, idle: CircleDashed, settled: CircleCheck }

export function StateIcon({ state }: StateIconProps) {
  const Icon = ICON[state]
  return (
    <span role="img" aria-label={STATE_LABEL[state]} data-state={state} className="flex flex-none" style={{ color: 'var(--c)' }}>
      <Icon size={16} aria-hidden="true" className={state === 'working' ? 'thread-working' : undefined} />
    </span>
  )
}
