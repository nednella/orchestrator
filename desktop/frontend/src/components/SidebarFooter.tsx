import { useAgentos, useLatestAgentos } from '../AgentosContext'
import { actionsFor, formatShortcut } from '../actions'
import { useLayout } from '../LayoutContext'
import { CleanupsMenu } from './CleanupsMenu'
import { Icon } from './Icon'
import type { IconName } from './Icon'
import { MemoryTotal } from './MemoryTotal'

type FooterButtonProps = { id: string; label: string; active?: boolean; unseen?: number; icon?: IconName }

function FooterButton({ id, label, active = false, unseen = 0, icon }: FooterButtonProps) {
  const layout = useLayout()
  const latestAgentos = useLatestAgentos()
  const action = actionsFor(latestAgentos(), layout).find((a) => a.id === id)!
  return (
    <button
      className={`btn btn-ghost h-7 gap-1.5 ${icon ? 'w-7 justify-center px-0' : 'px-2'}`}
      style={{ color: active ? 'var(--accent)' : undefined }}
      title={action.shortcut ? `${label} (${formatShortcut(action.shortcut)})` : label}
      aria-label={label}
      aria-pressed={icon ? undefined : active}
      onClick={() => action.run([])}
    >
      {icon ? <Icon name={icon} /> : label}
      {unseen > 0 && <span className="mono text-small text-accent">{unseen > 9 ? '9+' : unseen}</span>}
    </button>
  )
}

export function SidebarFooter() {
  const { digestUnseenCount, newsUnseen } = useAgentos('digestUnseenCount', 'newsUnseen')
  const { statsOpen, newsOpen, newsPage } = useLayout()
  return (
    <div className="flex flex-none flex-wrap items-center gap-x-0.5 gap-y-1 border-t border-line px-1.5 py-1.5">
      <FooterButton id="settings" label="Settings" icon="settings" />
      <FooterButton id="stats" label="Stats" active={statsOpen} />
      <FooterButton id="digest" label="Digest" active={newsOpen && newsPage === 'digest'} unseen={digestUnseenCount} />
      <FooterButton id="news" label="News" active={newsOpen && newsPage !== 'digest'} unseen={newsUnseen} />
      <CleanupsMenu />
      <span className="ml-auto">
        <MemoryTotal />
      </span>
    </div>
  )
}
