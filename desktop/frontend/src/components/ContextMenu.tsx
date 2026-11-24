import { Fragment, useEffect, useRef } from 'react'
import type { KeyboardEvent, MouseEvent } from 'react'
import { createPortal } from 'react-dom'
import { useFloatingAt } from '../useFloatingAt'
import type { Point } from '../useFloatingAt'

export type MenuItem = { label: string; run(e: MouseEvent): void; danger?: boolean; separated?: boolean }

type ContextMenuProps = { label: string; at: Point; items: (MenuItem | false)[]; onClose(): void }

export function ContextMenu({ label, at, items, onClose }: ContextMenuProps) {
  const menu = useRef<HTMLDivElement>(null)
  const style = useFloatingAt(menu, at, onClose)
  const shown = items.filter((item): item is MenuItem => item !== false)

  useEffect(() => {
    menu.current?.querySelector<HTMLElement>('[role="menuitem"]')?.focus()
  }, [])

  // The menu renders in a portal, so React would still pass its keys and clicks up to the row and the list.
  const onKeyDown = (e: KeyboardEvent<HTMLDivElement>) => {
    e.stopPropagation()
    const buttons = [...e.currentTarget.querySelectorAll<HTMLElement>('[role="menuitem"]')]
    const at = buttons.indexOf(document.activeElement as HTMLElement)
    const moves: Record<string, number | undefined> = { ArrowDown: at + 1, ArrowUp: at - 1, Home: 0, End: buttons.length - 1 }
    const to = moves[e.key]
    if (e.key === 'Tab') {
      e.preventDefault()
      onClose()
      return
    }
    if (to === undefined) return
    e.preventDefault()
    buttons[(to + buttons.length) % buttons.length]?.focus()
  }

  return createPortal(
    <div
      ref={menu}
      role="menu"
      aria-label={label}
      className="popup fade-in min-w-44"
      style={style}
      onKeyDown={onKeyDown}
      onContextMenu={(e) => {
        e.preventDefault()
        e.stopPropagation()
      }}
    >
      {shown.map((item) => (
        <Fragment key={item.label}>
          {item.separated && <div role="separator" className="mx-2 my-1 h-px bg-line" />}
          <button
            role="menuitem"
            tabIndex={-1}
            className="composer-option"
            data-danger={item.danger}
            onMouseEnter={(e) => e.currentTarget.focus()}
            onClick={(e) => {
              onClose()
              item.run(e)
            }}
          >
            {item.label}
          </button>
        </Fragment>
      ))}
    </div>,
    document.body,
  )
}
