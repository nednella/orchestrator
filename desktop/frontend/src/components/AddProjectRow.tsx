import type { ReactNode } from 'react'

type AddProjectRowProps = {
  id?: string
  icon: ReactNode
  title: string
  description?: string
  trailing?: ReactNode
  active?: boolean
  checked?: boolean
  onHover?(): void
  onRun?(): void
}

// A row of the add-project list. Without onRun it only shows something, as the summary of a new project does.
export function AddProjectRow({ id, icon, title, description, trailing, active, checked, onHover, onRun }: AddProjectRowProps) {
  return (
    <div
      id={id}
      role={onRun ? 'option' : undefined}
      aria-selected={onRun ? !!active : undefined}
      aria-checked={checked}
      data-row={id}
      className={`flex min-h-8 items-center gap-2 rounded-sm px-2 py-1.5 ${onRun ? 'cursor-pointer' : ''} ${active ? 'bg-(--bg-row-active)' : ''}`}
      onMouseDown={onRun ? (e) => e.preventDefault() : undefined}
      onMouseMove={onHover}
      onClick={onRun}
    >
      <span className="flex h-5 flex-none items-center self-start pt-0.5 text-soft">{icon}</span>
      <span className="flex min-w-0 flex-1 flex-col">
        <span className="truncate text-body text-ink">{title}</span>
        {description && <span className="truncate text-small text-soft/85">{description}</span>}
      </span>
      {trailing}
    </div>
  )
}
