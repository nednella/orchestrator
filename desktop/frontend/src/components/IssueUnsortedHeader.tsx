import { Icon } from './Icon'

type IssueUnsortedHeaderProps = { count: number; open: boolean; locked: boolean; onToggle(): void }

export function IssueUnsortedHeader({ count, open, locked, onToggle }: IssueUnsortedHeaderProps) {
  return (
    <button className="section-head text-small" aria-expanded={open} disabled={locked} onClick={onToggle}>
      <span>{`${count} ${open ? '' : 'more '}${count === 1 ? 'issue matches' : 'issues match'} no section`}</span>
      <span className="ml-auto flex items-center gap-2.5">
        {open ? 'Hide' : 'Show'}
        <span data-chevron className="transition-transform duration-150" style={{ transform: open ? 'none' : 'rotate(-90deg)' }}>
          <Icon name="chevron" size={12} />
        </span>
      </span>
    </button>
  )
}
