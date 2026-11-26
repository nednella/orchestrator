import { Check, FileText } from 'lucide-react'
import { useEffect, useRef } from 'react'
import type { KeyboardEvent } from 'react'
import { createPortal } from 'react-dom'
import { fileLabel, LATEST_PLAN } from '../filesView'
import { useFloatingAt } from '../useFloatingAt'
import type { Point } from '../useFloatingAt'

// planWaiting says the latest plan still waits on the owner.
type FilePickerProps = { at: Point; choices: string[]; current: string; planWaiting: boolean; onChoose(choice: string): void; onClose(): void }

// FilePicker is the Files view's list of the files it can show, the one shown ticked.
export function FilePicker({ at, choices, current, planWaiting, onChoose, onClose }: FilePickerProps) {
  const menu = useRef<HTMLDivElement>(null)
  const style = useFloatingAt(menu, at, onClose)

  useEffect(() => {
    const items = menu.current?.querySelectorAll<HTMLElement>('[role="menuitemradio"]')
    ;(menu.current?.querySelector<HTMLElement>('[aria-checked="true"]') ?? items?.[0])?.focus()
  }, [])

  const onKeyDown = (e: KeyboardEvent<HTMLDivElement>) => {
    e.stopPropagation()
    if (e.key === 'Tab') {
      e.preventDefault()
      onClose()
      return
    }
    const items = [...e.currentTarget.querySelectorAll<HTMLElement>('[role="menuitemradio"]')]
    const at = items.indexOf(document.activeElement as HTMLElement)
    const moves: Record<string, number | undefined> = { ArrowDown: at + 1, ArrowUp: at - 1, Home: 0, End: items.length - 1 }
    const to = moves[e.key]
    if (to === undefined) return
    e.preventDefault()
    items[(to + items.length) % items.length]?.focus()
  }

  return createPortal(
    <div ref={menu} role="menu" aria-label="Files" className="popup fade-in max-h-80 w-72 overflow-y-auto" style={style} onKeyDown={onKeyDown}>
      {choices.map((choice) => {
        const { folders, name } = fileLabel(choice)
        const checked = choice === current
        return (
          <button key={choice} role="menuitemradio" aria-checked={checked} tabIndex={-1} className="composer-option" title={choice === LATEST_PLAN ? undefined : choice} onMouseEnter={(e) => e.currentTarget.focus()} onClick={() => onChoose(choice)}>
            <FileText size={14} className="flex-none text-soft" aria-hidden="true" />
            <span className="min-w-0 flex-1 truncate">
              {folders && <span className="text-soft">{folders} / </span>}
              <span className="text-ink">{name}</span>
            </span>
            {choice === LATEST_PLAN && planWaiting && <span className="flex-none text-waiting">Waiting for you</span>}
            {checked && <Check size={14} className="flex-none" aria-hidden="true" />}
          </button>
        )
      })}
    </div>,
    document.body,
  )
}
