import { useEffect, useRef } from 'react'
import type { KeyboardEvent, RefObject } from 'react'
import { createPortal } from 'react-dom'
import { moveOption } from '../selectKeys'
import type { SelectOption } from './SelectMenu'

type SelectPopupProps<T extends string> = {
  anchor: DOMRect
  trigger: RefObject<HTMLButtonElement | null>
  label: string
  options: SelectOption<T>[]
  value: T
  onPick(value: T): void
  onClose(refocus: boolean): void
}

const GAP_PX = 4
const EDGE_PX = 8

// The list opens under its trigger with their right edges level, or over it when there is more room there.
export function SelectPopup<T extends string>({ anchor, trigger, label, options, value, onPick, onClose }: SelectPopupProps<T>) {
  const list = useRef<HTMLDivElement>(null)
  const below = window.innerHeight - anchor.bottom
  const above = anchor.top
  const opensAbove = below < 160 && above > below
  const place = opensAbove
    ? { bottom: window.innerHeight - anchor.top + GAP_PX, maxHeight: above - GAP_PX - EDGE_PX }
    : { top: anchor.bottom + GAP_PX, maxHeight: below - GAP_PX - EDGE_PX }

  useEffect(() => {
    list.current?.querySelector<HTMLElement>('[aria-selected="true"]')?.focus()
  }, [])

  useEffect(() => {
    const outside = (e: MouseEvent) => {
      const target = e.target as Node
      if (!list.current?.contains(target) && !trigger.current?.contains(target)) onClose(false)
    }
    const escape = (e: globalThis.KeyboardEvent) => {
      if (e.key !== 'Escape') return
      e.preventDefault()
      e.stopPropagation()
      onClose(true)
    }
    const scrolled = (e: Event) => {
      if (!list.current?.contains(e.target as Node)) onClose(false)
    }
    const resized = () => onClose(false)
    window.addEventListener('mousedown', outside)
    window.addEventListener('keydown', escape, true)
    window.addEventListener('scroll', scrolled, true)
    window.addEventListener('resize', resized)
    return () => {
      window.removeEventListener('mousedown', outside)
      window.removeEventListener('keydown', escape, true)
      window.removeEventListener('scroll', scrolled, true)
      window.removeEventListener('resize', resized)
    }
  }, [trigger, onClose])

  const onKeyDown = (e: KeyboardEvent<HTMLDivElement>) => {
    if (e.key === 'Tab') {
      e.preventDefault()
      onClose(true)
      return
    }
    const rows = [...e.currentTarget.querySelectorAll<HTMLElement>('[role="option"]')]
    const to = moveOption(e.key, rows.indexOf(document.activeElement as HTMLElement), rows.length)
    if (to < 0) return
    e.preventDefault()
    rows[to].focus()
  }

  return createPortal(
    <div
      ref={list}
      role="listbox"
      aria-label={label}
      className="fade-in fixed overflow-y-auto rounded-lg border border-(--border-glass) bg-(--bg-dropdown) bg-(image:--bg-dropdown-tint) p-1 shadow-(--shadow-popup) backdrop-blur-[16px] backdrop-saturate-[1.08]"
      style={{ zIndex: 'var(--z-select)', right: window.innerWidth - anchor.right, minWidth: anchor.width, ...place }}
      onKeyDown={onKeyDown}
    >
      {options.map((option) => (
        <button
          key={option.value}
          type="button"
          role="option"
          aria-selected={option.value === value}
          className={`flex min-h-7 w-full cursor-pointer items-center rounded-sm px-2 py-1 text-left text-body outline-none hover:bg-(--bg-hover) focus:bg-(--bg-hover) ${option.value === value ? 'bg-(--bg-row-active)' : ''} ${option.description ? 'min-w-64' : ''}`}
          onClick={() => onPick(option.value)}
        >
          <span className="grid min-w-0 gap-0.5">
            <span className={`inline-flex items-center gap-1.5 ${option.description ? 'font-medium' : ''}`}>
              {option.icon && <span className="flex-none text-soft">{option.icon}</span>}
              {option.label}
            </span>
            {option.description && <span className="text-small leading-4 text-dim">{option.description}</span>}
          </span>
        </button>
      ))}
    </div>,
    document.body,
  )
}
