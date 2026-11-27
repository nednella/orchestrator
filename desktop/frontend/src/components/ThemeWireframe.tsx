import type { ThemeName } from '../types'
import { WireframePane } from './WireframePane'

type ThemeWireframeProps = { panes: { theme: ThemeName; clip?: 'left' | 'right' }[] }

// Two panes split the frame in half with a 2px gap: the System tile's light half beside its dark half.
export function ThemeWireframe({ panes }: ThemeWireframeProps) {
  return (
    <span aria-hidden="true" className="relative block h-35 w-full overflow-hidden rounded-lg border border-(--border-rule)">
      {panes.map((pane) => (
        <WireframePane key={pane.clip ?? 'whole'} theme={pane.theme} clip={pane.clip} />
      ))}
    </span>
  )
}
