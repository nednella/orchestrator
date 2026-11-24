import { useEffect, useRef, useState } from 'react'
import type { KeyboardEvent } from 'react'
import { useAgentos } from '../AgentosContext'
import { waitingByProject } from '../threads'
import { Icon } from './Icon'

type WaitingBadgeProps = { count: number; title: string }

function WaitingBadge({ count, title }: WaitingBadgeProps) {
  if (count === 0) return null
  return (
    <span className="flex flex-none items-center gap-1" title={title}>
      <span className="dot" data-state="waiting" />
      <span className="mono text-small text-dim">{count}</span>
    </span>
  )
}

export function ProjectPicker() {
  const { project, projects, sessions, showHome, addProject, report } = useAgentos('project', 'projects', 'sessions', 'showHome', 'addProject', 'report')
  const [open, setOpen] = useState(false)
  const root = useRef<HTMLDivElement>(null)
  const trigger = useRef<HTMLButtonElement>(null)
  const menu = useRef<HTMLDivElement>(null)
  const waiting = waitingByProject(sessions)
  const elsewhere = [...waiting].reduce((n, [key, count]) => (key === project?.key ? n : n + count), 0)

  useEffect(() => {
    if (!open) return
    const close = (e: MouseEvent) => {
      if (!root.current?.contains(e.target as Node)) setOpen(false)
    }
    window.addEventListener('mousedown', close)
    return () => window.removeEventListener('mousedown', close)
  }, [open])

  useEffect(() => {
    if (!open) return
    const items = menu.current?.querySelectorAll<HTMLElement>('[role="menuitem"]')
    ;(menu.current?.querySelector<HTMLElement>('[aria-current="true"]') ?? items?.[0])?.focus()
  }, [open])

  const choose = (run: () => Promise<void> | void) => {
    setOpen(false)
    trigger.current?.focus()
    report(async () => run())
  }

  const onMenuKeyDown = (e: KeyboardEvent) => {
    e.stopPropagation()
    if (e.key === 'Escape') {
      e.preventDefault()
      setOpen(false)
      trigger.current?.focus()
      return
    }
    const step = e.key === 'ArrowDown' ? 1 : e.key === 'ArrowUp' ? -1 : 0
    if (step === 0) return
    e.preventDefault()
    const items = [...(menu.current?.querySelectorAll<HTMLElement>('[role="menuitem"]') ?? [])]
    const at = items.indexOf(document.activeElement as HTMLElement)
    items[(at + step + items.length) % items.length]?.focus()
  }

  return (
    <div ref={root} className="relative flex min-w-0 flex-1 items-center gap-1">
      <button
        ref={trigger}
        className="btn btn-ghost h-7 min-w-0 flex-1 justify-start gap-1.5 px-2"
        aria-haspopup="menu"
        aria-expanded={open}
        aria-label={`Project ${project?.name ?? ''}`}
        title="Switch project"
        onClick={() => setOpen(!open)}
      >
        <span className="flex-none text-dim">
          <Icon name="chevron" size={12} />
        </span>
        <span className="min-w-0 flex-1 truncate text-left text-body font-semibold">{project?.name}</span>
        <WaitingBadge count={elsewhere} title={`${elsewhere} in other projects need you`} />
      </button>
      {project && (
        <button
          className="btn btn-ghost h-7 w-7 flex-none justify-center px-0"
          title={`${project.name} home`}
          aria-label={`${project.name} home`}
          onClick={() => report(() => showHome(project.key))}
        >
          <Icon name="home" size={13} />
        </button>
      )}
      {open && (
        <div
          ref={menu}
          className="fade-in absolute top-8 left-0 max-h-80 w-full min-w-48 overflow-y-auto rounded-md border border-line-strong bg-raised py-1"
          style={{ zIndex: 'var(--z-popup)' }}
          role="menu"
          onKeyDown={onMenuKeyDown}
        >
          {projects.map((p) => (
            <button
              key={p.key}
              role="menuitem"
              className="row h-8 items-center gap-2 px-3"
              data-selected={p.key === project?.key}
              aria-current={p.key === project?.key}
              onClick={() => choose(() => showHome(p.key))}
            >
              <span className="min-w-0 flex-1 truncate text-left">{p.name}</span>
              <WaitingBadge count={waiting.get(p.key) ?? 0} title="Threads waiting for you" />
            </button>
          ))}
          <button role="menuitem" className="row h-8 items-center gap-2 border-t border-line px-3 text-soft" onClick={() => choose(() => addProject())}>
            <Icon name="plus" size={12} />
            Add project…
          </button>
        </div>
      )}
    </div>
  )
}
