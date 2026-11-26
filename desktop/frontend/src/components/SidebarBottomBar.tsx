import { ChartNoAxesColumn, CircleDot, Ellipsis, GitPullRequestArrow, Settings } from 'lucide-react'
import { useLayoutEffect, useRef, useState } from 'react'
import type { ReactNode } from 'react'
import { useOrchestrator, useLatestOrchestrator } from '../OrchestratorContext'
import { actionsFor } from '../actions'
import { barLayout } from '../bottomBar'
import type { BarButtonId } from '../bottomBar'
import { useLayout } from '../LayoutContext'
import { openUsage, useUsageOpen } from '../usageOpen'
import type { Point } from '../useFloatingAt'
import { ContextMenu } from './ContextMenu'
import { Icon } from './Icon'
import { SidebarBarButton } from './SidebarBarButton'
import { UpdateButton } from './UpdateButton'

const ICONS: Record<BarButtonId, ReactNode> = {
  settings: <Settings size={16} aria-hidden="true" />,
  pulls: <GitPullRequestArrow size={16} aria-hidden="true" />,
  issues: <CircleDot size={16} aria-hidden="true" />,
  usage: <ChartNoAxesColumn size={16} aria-hidden="true" />,
  stats: <Icon name="chart" fixed />,
  news: <Icon name="digest" fixed />,
}

// SidebarBottomBar sits under the thread list: the pages and views the owner opens from anywhere, and the update
// button at the right. Buttons that do not fit beside it go into the More menu, the last ones first.
export function SidebarBottomBar() {
  const { report, digestUnseenCount } = useOrchestrator('report', 'digestUnseenCount')
  const layout = useLayout()
  const latestOrchestrator = useLatestOrchestrator()
  const usageOpen = useUsageOpen()
  const bar = useRef<HTMLDivElement>(null)
  const [width, setWidth] = useState(Infinity)
  const [menuAt, setMenuAt] = useState<Point | null>(null)
  // A press on More closes its open menu before the click: the click must not open it again.
  const menuClosedAt = useRef(0)

  useLayoutEffect(() => {
    const el = bar.current
    if (!el) return
    setWidth(el.getBoundingClientRect().width)
    const observer = new ResizeObserver(([entry]) => setWidth(entry.borderBoxSize[0]?.inlineSize ?? entry.contentRect.width))
    observer.observe(el)
    return () => observer.disconnect()
  }, [])

  const runAction = (id: string) => report(() => actionsFor(latestOrchestrator(), layout).find((a) => a.id === id)!.run())
  const open: Record<BarButtonId, () => void> = {
    settings: () => runAction('settings'),
    pulls: () => report(() => layout.showPanelView('pulls', { expanded: true })),
    issues: () => report(() => layout.showPanelView('issues', { expanded: true })),
    usage: openUsage,
    stats: () => runAction('stats'),
    news: () => runAction('digest'),
  }
  const pressed: Partial<Record<BarButtonId, boolean>> = { stats: layout.statsOpen, usage: usageOpen, news: layout.digestOpen }
  const unseen: Partial<Record<BarButtonId, number>> = { news: digestUnseenCount }
  const { shown, more } = barLayout(width)

  // Its contents never widen the sidebar: the bar takes the thread list's width, and what does not fit goes to More.
  return (
    <div ref={bar} className="panel flex flex-none items-center gap-[4px] px-[8px] pt-[4px] pb-[8px] [contain:inline-size]">
      {shown.map((b) => (
        <SidebarBarButton key={b.id} label={b.label} pressed={pressed[b.id]} unseen={unseen[b.id]} onClick={open[b.id]}>
          {ICONS[b.id]}
        </SidebarBarButton>
      ))}
      {more.length > 0 && (
        <SidebarBarButton
          label="More"
          expanded={menuAt !== null}
          unseen={more.reduce((n, b) => n + (unseen[b.id] ?? 0), 0)}
          onClick={(e) => {
            if (performance.now() - menuClosedAt.current < 300) return
            const r = e.currentTarget.getBoundingClientRect()
            setMenuAt({ x: r.left, y: r.top })
          }}
        >
          <Ellipsis size={16} aria-hidden="true" />
        </SidebarBarButton>
      )}
      {menuAt && (
        <ContextMenu
          label="More"
          at={menuAt}
          items={more.map((b) => ({ label: b.label, run: open[b.id] }))}
          onClose={() => {
            menuClosedAt.current = performance.now()
            setMenuAt(null)
          }}
        />
      )}
      <UpdateButton />
    </div>
  )
}
