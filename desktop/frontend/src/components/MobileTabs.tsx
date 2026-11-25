import { useAgentos } from '../AgentosContext'
import { useLayout } from '../LayoutContext'
import { panelViewFor, shownPanelViews, usePanelContext } from '../panelViews'

type TabProps = { label: string; active: boolean; badge?: string; urgent?: boolean; onPick(): void }

function Tab({ label, active, badge, urgent, onPick }: TabProps) {
  return (
    <button
      role="tab"
      aria-selected={active}
      className="flex h-11 min-w-0 flex-1 flex-col short:h-8 items-center justify-center gap-0.5 border-t-2 text-small font-medium"
      style={{ borderColor: active ? 'var(--accent)' : 'transparent', color: active ? 'var(--text)' : 'var(--text-soft)' }}
      onClick={onPick}
    >
      <span className="flex items-center gap-1.5">
        {label}
        {urgent && <span className="dot" data-state="waiting" />}
      </span>
      {badge && <span className="mono short:hidden text-label text-dim">{badge}</span>}
    </button>
  )
}

export function MobileTabs() {
  const { sessions } = useAgentos('sessions')
  const { mobilePanel, setMobilePanel, panelView, showPanel } = useLayout()
  const ctx = usePanelContext()
  const shown = panelViewFor(panelView, ctx)
  const needsYou = sessions.some((s) => s.state === 'waiting')
  return (
    <nav role="tablist" aria-label="Panels" className="flex flex-none border-t border-line bg-app">
      <Tab label="Thread" active={mobilePanel === 'thread'} badge={ctx.session ? `#${ctx.session.n}` : '–'} onPick={() => setMobilePanel('thread')} />
      {shownPanelViews(ctx).map((view) => {
        const count = view.count?.(ctx)
        return <Tab key={view.id} label={view.label} active={mobilePanel === 'panel' && shown.id === view.id} badge={count === undefined ? undefined : String(count)} onPick={() => showPanel(view.id)} />
      })}
      <Tab label="Threads" active={mobilePanel === 'threads'} badge={String(sessions.length)} urgent={needsYou} onPick={() => setMobilePanel('threads')} />
      <Tab label="Shell" active={mobilePanel === 'shell'} badge="❯" onPick={() => setMobilePanel('shell')} />
    </nav>
  )
}
