import { Check } from 'lucide-react'
import type { ReactNode } from 'react'
import { LIST_ITEM } from '../listStyle'

type ListMenuItemProps = {
  role?: 'menuitem' | 'menuitemradio' | 'menuitemcheckbox'
  checked?: boolean
  icon?: ReactNode
  danger?: boolean
  disabled?: boolean
  title?: string
  onClick(): void
  children: ReactNode
}

export function ListMenuItem({ role = 'menuitem', checked, icon, danger, disabled, title, onClick, children }: ListMenuItemProps) {
  return (
    <button role={role} aria-checked={role === 'menuitem' ? undefined : !!checked} tabIndex={-1} className={`${LIST_ITEM} ${danger ? 'text-danger' : 'text-ink'}`} disabled={disabled} title={title} onMouseEnter={(e) => e.currentTarget.focus()} onClick={onClick}>
      {icon !== undefined && <span className="flex w-3.5 flex-none justify-center text-soft">{icon}</span>}
      <span className="min-w-0 flex-1 truncate">{children}</span>
      {role !== 'menuitem' && <span className="flex w-3.5 flex-none text-soft">{checked && <Check size={14} />}</span>}
    </button>
  )
}
