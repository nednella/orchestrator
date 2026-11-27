import { ChevronDown } from 'lucide-react'

type SectionHeaderProps = { name: string; count: number; open: boolean; locked?: boolean; onToggle(): void }

export function SectionHeader({ name, count, open, locked = false, onToggle }: SectionHeaderProps) {
  return (
    <button className="section-head" aria-expanded={open} disabled={locked} onClick={onToggle}>
      <span data-section-name className="text-small font-medium">
        {name}
      </span>
      <span className="text-small text-dim">{count}</span>
      <span data-chevron className="ml-auto flex transition-transform duration-150 motion-reduce:transition-none" style={{ transform: open ? 'none' : 'rotate(-90deg)' }}>
        <ChevronDown size={14} aria-hidden="true" />
      </span>
    </button>
  )
}
