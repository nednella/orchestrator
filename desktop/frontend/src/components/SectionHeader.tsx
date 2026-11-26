import { Icon } from './Icon'

type SectionHeaderProps = { name: string; count: number; open: boolean; locked?: boolean; onToggle(): void }

export function SectionHeader({ name, count, open, locked = false, onToggle }: SectionHeaderProps) {
  return (
    <button className="section-head" aria-expanded={open} disabled={locked} onClick={onToggle}>
      <span className="label">{name}</span>
      <span className="mono text-small text-dim">{count}</span>
      <span className="ml-auto flex items-center gap-2.5">
        <span data-chevron className="transition-transform duration-150" style={{ transform: open ? 'none' : 'rotate(-90deg)' }}>
          <Icon name="chevron" size={12} />
        </span>
      </span>
    </button>
  )
}
