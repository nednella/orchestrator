import { ChevronRight, Search } from 'lucide-react'
import { useLayoutEffect, useRef, useState } from 'react'
import type { KeyboardEvent } from 'react'
import { createPortal } from 'react-dom'
import { rowValue, shownOptions, toggled } from '../listOptions'
import type { CheckRow, RadioRow } from '../listOptions'
import { LIST_ITEM } from '../listStyle'
import { moveFocus } from './ListMenu'
import { ListMenuItem } from './ListMenuItem'

const EDGE = 8
// A row shows a search field once it has more options than this.
const SEARCH_FROM = 6

type FilterRowItemProps = {
  row: RadioRow | CheckRow
  menuId: string
  open: boolean
  onOpen(open: boolean): void
}

// FilterRowItem is one row of the Filters menu: what it filters, what it holds, and a submenu beside it with the options.
export function FilterRowItem({ row, menuId, open, onOpen }: FilterRowItemProps) {
  const trigger = useRef<HTMLButtonElement>(null)
  const sub = useRef<HTMLDivElement>(null)
  const [at, setAt] = useState<{ left: number; top: number; maxHeight: number } | null>(null)
  const [search, setSearch] = useState('')
  const searchable = row.options.length > SEARCH_FROM

  useLayoutEffect(() => {
    if (!open || !sub.current || !trigger.current) {
      setAt(null)
      return
    }
    const from = trigger.current.getBoundingClientRect()
    const { width, height } = sub.current.getBoundingClientRect()
    const beside = from.right + 4 + width <= window.innerWidth - EDGE ? from.right + 4 : Math.max(EDGE, from.left - 4 - width)
    const top = Math.max(EDGE, Math.min(from.top - 5, window.innerHeight - height - EDGE))
    setAt({ left: beside, top, maxHeight: window.innerHeight - 2 * EDGE })
  }, [open, search])

  useLayoutEffect(() => {
    if (!open) setSearch('')
  }, [open])

  const pick = (value: string) => {
    if (row.kind === 'radio') {
      onOpen(false)
      row.onChange(value)
      trigger.current?.focus()
    } else row.onChange(toggled(row.value, value))
  }

  const options = shownOptions(row, search)
  const onKeyDown = (e: KeyboardEvent<HTMLDivElement>) => {
    e.stopPropagation()
    if (e.key === 'ArrowLeft') {
      e.preventDefault()
      onOpen(false)
      trigger.current?.focus()
    } else if (e.key === 'Escape') {
      e.preventDefault()
      onOpen(false)
      trigger.current?.focus()
    } else moveFocus(e, { ArrowDown: 1, ArrowUp: -1, Home: 'first', End: 'last' })
  }

  return (
    <>
      <button
        ref={trigger}
        role="menuitem"
        aria-haspopup="menu"
        aria-expanded={open}
        tabIndex={-1}
        className={`${LIST_ITEM} text-ink`}
        onMouseEnter={(e) => {
          e.currentTarget.focus()
          onOpen(true)
        }}
        onClick={() => onOpen(true)}
        onKeyDown={(e) => {
          if (e.key === 'ArrowRight' || e.key === 'Enter' || e.key === ' ') {
            e.preventDefault()
            e.stopPropagation()
            onOpen(true)
            requestAnimationFrame(() => sub.current?.querySelector<HTMLElement>('input, [role^="menuitem"]')?.focus())
          }
        }}
      >
        <span className="flex w-3.5 flex-none justify-center text-soft">{row.icon}</span>
        <span className="min-w-0 flex-1 truncate">{row.label}</span>
        <span className="max-w-32 min-w-0 truncate text-label text-dim">{rowValue(row)}</span>
        <ChevronRight size={14} className="flex-none text-soft" />
      </button>
      {open &&
        createPortal(
          <div
            ref={sub}
            role="menu"
            aria-label={row.label}
            data-menu-layer
            data-menu-of={menuId}
            className="popup fade-in flex w-56 flex-col overflow-y-auto"
            style={at ? { left: at.left, top: at.top, maxHeight: at.maxHeight } : { left: 0, top: 0, opacity: 0 }}
            onKeyDown={onKeyDown}
          >
            <div className="px-2 py-1.5 text-label font-medium text-dim">{row.label}</div>
            {searchable && (
              <div className="relative mx-1 mb-1">
                <span className="pointer-events-none absolute inset-y-0 left-2 flex items-center text-dim">
                  <Search size={13} />
                </span>
                <input
                  autoFocus
                  value={search}
                  spellCheck={false}
                  placeholder={row.searchLabel}
                  aria-label={row.searchLabel}
                  className="field w-full pl-7 text-small"
                  onChange={(e) => setSearch(e.target.value)}
                  onKeyDown={(e) => {
                    if (e.key === 'ArrowDown') {
                      e.preventDefault()
                      e.stopPropagation()
                      sub.current?.querySelector<HTMLElement>('[role^="menuitem"]')?.focus()
                    }
                  }}
                />
              </div>
            )}
            {options.map((o) => (
              <ListMenuItem
                key={o.value}
                role={row.kind === 'radio' ? 'menuitemradio' : 'menuitemcheckbox'}
                checked={row.kind === 'radio' ? o.value === row.value : row.value.includes(o.value)}
                icon={o.icon}
                disabled={o.unavailable !== undefined}
                title={o.unavailable}
                onClick={() => pick(o.value)}
              >
                {o.label}
                {o.count !== undefined && <span className="ml-2 text-label text-dim">{o.count}</span>}
                {o.unavailable !== undefined && <span className="text-label text-dim"> · Unavailable</span>}
              </ListMenuItem>
            ))}
            {options.length === 0 && <div className="px-2 py-1.5 text-small text-dim">{row.emptyText ?? 'Nothing found'}</div>}
          </div>,
          document.body,
        )}
    </>
  )
}
