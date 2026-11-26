import type { TermGroup } from '../terminalGroups'
import { TerminalPane } from './TerminalPane'

type TerminalSplitProps = { group: TermGroup; onActivate(id: string): void }

// TerminalSplit shows the group's terminals in equal columns, or stacked in equal rows. The first sits on the edge of
// its own, so only the panes after it draw a divider.
export function TerminalSplit({ group, onActivate }: TerminalSplitProps) {
  const row = group.dir === 'row'
  const tracks = `repeat(${group.panes.length}, minmax(0, 1fr))`
  return (
    <div className="grid min-h-0 min-w-0 flex-1" style={row ? { gridTemplateColumns: tracks } : { gridTemplateRows: tracks }}>
      {group.panes.map((id, i) => (
        <TerminalPane key={id} id={id} edge={row ? 'left' : 'top'} divider={i === 0 ? 'none' : id === group.active ? 'active' : 'idle'} onActivate={() => onActivate(id)} />
      ))}
    </div>
  )
}
