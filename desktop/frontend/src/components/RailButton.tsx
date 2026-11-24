import { useLatestAgentos } from '../AgentosContext'
import { actionsFor, formatShortcut } from '../actions'
import { useLayout } from '../LayoutContext'
import { Icon } from './Icon'
import type { IconName } from './Icon'

type RailButtonProps = { id: string; label: string; icon: IconName }

export function RailButton({ id, label, icon }: RailButtonProps) {
  const layout = useLayout()
  const latestAgentos = useLatestAgentos()
  const action = actionsFor(latestAgentos(), layout).find((a) => a.id === id)!
  return (
    <button
      className="btn btn-ghost h-9 w-9 flex-none justify-center px-0"
      title={action.shortcut ? `${label} (${formatShortcut(action.shortcut)})` : label}
      aria-label={label}
      data-rail-item
      onClick={() => action.run([])}
    >
      <Icon name={icon} />
    </button>
  )
}
