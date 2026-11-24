import type { ReactNode } from 'react'
import { Icon } from './Icon'
import type { IconName } from './Icon'

type ThreadCardLineProps = { icon: IconName; iconColor?: string; children: ReactNode }

export function ThreadCardLine({ icon, iconColor, children }: ThreadCardLineProps) {
  return (
    <div className="flex min-w-0 items-start gap-2">
      <span className="flex h-5 flex-none items-center" style={{ color: iconColor ?? 'var(--text-dim)' }}>
        <Icon name={icon} size={14} />
      </span>
      <span className="min-w-0 break-words">{children}</span>
    </div>
  )
}
