import type { MouseEvent, ReactNode } from 'react'
import { useDelayedTip } from '../useDelayedTip'
import { FloatingTip } from './FloatingTip'

export const sidebarBarButton =
  'relative inline-flex h-[32px] w-[32px] flex-none items-center justify-center rounded-[8px] text-[var(--sidebar-icon)] transition-colors duration-[120ms] hover:bg-[var(--bg-row-hover)] hover:text-ink aria-pressed:bg-[var(--bg-row-active)] aria-pressed:text-ink aria-expanded:bg-[var(--bg-row-active)] aria-expanded:text-ink'

type SidebarBarButtonProps = { label: string; pressed?: boolean; unseen?: number; expanded?: boolean; onClick(e: MouseEvent<HTMLButtonElement>): void; children: ReactNode }

export function SidebarBarButton({ label, pressed, unseen = 0, expanded, onClick, children }: SidebarBarButtonProps) {
  const { anchor, tipHandlers } = useDelayedTip()
  const name = unseen > 0 ? `${label}, ${unseen} new` : label
  return (
    <button
      className={sidebarBarButton}
      aria-label={name}
      aria-pressed={pressed}
      aria-haspopup={expanded === undefined ? undefined : 'menu'}
      aria-expanded={expanded}
      onClick={onClick}
      {...tipHandlers}
    >
      {children}
      {unseen > 0 && <span className="mono absolute top-[3px] right-[2px] text-[10px] leading-none text-accent">{unseen > 9 ? '9+' : unseen}</span>}
      {anchor && <FloatingTip anchor={anchor} text={name} />}
    </button>
  )
}
