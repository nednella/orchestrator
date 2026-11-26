import { CircleDot } from 'lucide-react'
import type { ReactNode } from 'react'
import type { CardIcon } from '../threadCard'
import { Icon } from './Icon'

type ThreadCardLineProps = { icon: CardIcon; iconColor?: string; children: ReactNode }

export function ThreadCardLine({ icon, iconColor, children }: ThreadCardLineProps) {
  return (
    <div className="flex min-w-0 items-start gap-2">
      <span className="flex h-5 flex-none items-center" style={{ color: iconColor ?? 'var(--text-dim)' }}>
        {icon === 'circle-dot' ? <CircleDot size={14} aria-hidden="true" /> : <Icon name={icon} size={14} />}
      </span>
      <span className="min-w-0 break-words">{children}</span>
    </div>
  )
}
