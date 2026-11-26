import type { TermGroup } from '../terminalGroups'
import { TerminalPane } from './TerminalPane'

type TerminalSplitProps = { group: TermGroup; onActivate(id: string): void }

// TerminalSplit shows the group's terminals side by side in equal columns. The first sits on the column's own edge,
// so only the panes after it draw a divider.
export function TerminalSplit({ group, onActivate }: TerminalSplitProps) {
  return (
    <div className="grid min-h-0 min-w-0 flex-1" style={{ gridTemplateColumns: `repeat(${group.panes.length}, minmax(0, 1fr))` }}>
      {group.panes.map((id, i) => (
        <TerminalPane key={id} id={id} divider={i === 0 ? 'none' : id === group.active ? 'active' : 'idle'} onActivate={() => onActivate(id)} />
      ))}
    </div>
  )
}
