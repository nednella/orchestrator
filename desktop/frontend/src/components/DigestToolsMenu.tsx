import { ChevronDown, ListFilter } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import type { DigestTool } from '../digestFeed'

type DigestToolsMenuProps = { tools: DigestTool[]; onPick(key: string): void }

export function DigestToolsMenu({ tools, onPick }: DigestToolsMenuProps) {
  const [open, setOpen] = useState(false)
  const root = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!open) return
    const outside = (e: MouseEvent) => {
      if (!root.current?.contains(e.target as Node)) setOpen(false)
    }
    // Ahead of the page's own Escape, which closes the page.
    const escape = (e: KeyboardEvent) => {
      if (e.key !== 'Escape') return
      e.preventDefault()
      e.stopPropagation()
      setOpen(false)
      root.current?.querySelector('button')?.focus()
    }
    window.addEventListener('mousedown', outside)
    window.addEventListener('keydown', escape, true)
    root.current?.querySelector<HTMLElement>('[role="menuitem"]')?.focus()
    return () => {
      window.removeEventListener('mousedown', outside)
      window.removeEventListener('keydown', escape, true)
    }
  }, [open])

  return (
    <div ref={root} className="relative flex-none">
      <button
        className="inline-flex h-7 items-center gap-1.5 rounded-lg px-2 text-small font-medium whitespace-nowrap text-[var(--text-chip)] hover:bg-hover hover:text-ink aria-expanded:bg-hover aria-expanded:text-ink"
        aria-haspopup="menu"
        aria-expanded={open}
        onClick={() => setOpen(!open)}
      >
        <ListFilter size={14} className="text-soft" aria-hidden />
        <span className="@max-[40rem]:sr-only">{tools.length === 1 ? '1 tool' : `${tools.length} tools`}</span>
        <ChevronDown size={12} className="text-soft" aria-hidden />
      </button>
      {open && (
        <div
          className="fade-in absolute top-[calc(100%+0.375rem)] right-0 max-h-72 min-w-48 overflow-y-auto rounded-xl bg-[var(--bg-composer)] p-1 shadow-[0_0_0_1px_var(--border-strong),var(--shadow-popup)]"
          style={{ zIndex: 'var(--z-popup)' }}
          role="menu"
          aria-label="Go to a tool"
        >
          {tools.map((tool) => (
            <button
              key={tool.key}
              role="menuitem"
              className="flex h-7 w-full items-center gap-3 rounded-lg px-2 text-left text-small outline-none hover:bg-hover focus:bg-hover"
              onClick={() => {
                setOpen(false)
                onPick(tool.key)
              }}
            >
              <span className="min-w-0 truncate">{tool.name}</span>
              <span className="ml-auto text-soft tabular-nums">{tool.items.length}</span>
            </button>
          ))}
        </div>
      )}
    </div>
  )
}
