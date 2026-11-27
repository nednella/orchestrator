import type { LucideIcon } from 'lucide-react'
import { useLatestOrchestrator } from '../OrchestratorContext'
import { actionsFor, formatShortcut } from '../actions'
import { useLayout } from '../LayoutContext'

type RailButtonProps = { id: string; label: string; icon: LucideIcon }

export function RailButton({ id, label, icon: Icon }: RailButtonProps) {
  const layout = useLayout()
  const latestOrchestrator = useLatestOrchestrator()
  const action = actionsFor(latestOrchestrator(), layout).find((a) => a.id === id)!
  return (
    <button
      className="btn btn-ghost h-9 w-9 flex-none justify-center px-0"
      title={action.shortcut ? `${label} (${formatShortcut(action.shortcut)})` : label}
      aria-label={label}
      data-rail-item
      onClick={() => action.run()}
    >
      <Icon size={14} aria-hidden="true" />
    </button>
  )
}
