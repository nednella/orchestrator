import { STATE_LABEL } from '../stateMeta'
import type { State } from '../types'
import { Icon } from './Icon'
import type { IconName } from './Icon'

type StateIconProps = { state: State }

const ICON: Record<State, IconName> = { waiting: 'needs-you', working: 'loader', idle: 'idle', settled: 'settled' }

export function StateIcon({ state }: StateIconProps) {
  return (
    <span role="img" aria-label={STATE_LABEL[state]} data-state={state} className="flex flex-none" style={{ color: 'var(--c)' }}>
      <Icon name={ICON[state]} size={16} />
    </span>
  )
}
