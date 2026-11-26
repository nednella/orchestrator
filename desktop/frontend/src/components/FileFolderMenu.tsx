import { ChevronLeft, ChevronRight, FileText, Folder, RefreshCw } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import type { KeyboardEvent } from 'react'
import { createPortal } from 'react-dom'
import { baseName, childrenOf, parentOf } from '../fileTree'
import { useFloatingAt } from '../useFloatingAt'
import type { Point } from '../useFloatingAt'
import type { FileEntry } from '../types'

type FileFolderMenuProps = {
  at: Point
  folder: string
  entries: FileEntry[]
  onOpen(path: string): void
  onRefresh(): void
  onClose(): void
}

// FileFolderMenu lists what a folder holds, to open a file of it or to look into a folder of it, with a row that goes
// back out of a folder looked into and a row that reads the files again.
export function FileFolderMenu({ at, folder, entries, onOpen, onRefresh, onClose }: FileFolderMenuProps) {
  const menu = useRef<HTMLDivElement>(null)
  const style = useFloatingAt(menu, at, onClose)
  const [looking, setLooking] = useState(folder)
  const children = childrenOf(entries, looking)

  useEffect(() => {
    menu.current?.querySelector<HTMLElement>('[role="menuitem"]')?.focus()
  }, [looking])

  const onKeyDown = (e: KeyboardEvent<HTMLDivElement>) => {
    e.stopPropagation()
    if (e.key === 'Tab') {
      e.preventDefault()
      onClose()
      return
    }
    const items = [...e.currentTarget.querySelectorAll<HTMLElement>('[role="menuitem"]')]
    const at = items.indexOf(document.activeElement as HTMLElement)
    const moves: Record<string, number | undefined> = { ArrowDown: at + 1, ArrowUp: at - 1, Home: 0, End: items.length - 1 }
    const to = moves[e.key]
    if (to === undefined) return
    e.preventDefault()
    items[(to + items.length) % items.length]?.focus()
  }

  return createPortal(
    <div ref={menu} role="menu" aria-label={looking || 'Files'} className="popup fade-in max-h-80 w-64 overflow-y-auto" style={style} onKeyDown={onKeyDown}>
      {looking !== '' && (
        <button role="menuitem" tabIndex={-1} className="composer-option" title="Back" onMouseEnter={(e) => e.currentTarget.focus()} onClick={() => setLooking(parentOf(looking))}>
          <ChevronLeft size={14} className="flex-none text-soft" aria-hidden="true" />
          <span className="min-w-0 flex-1 truncate">{baseName(looking)}</span>
        </button>
      )}
      <button role="menuitem" tabIndex={-1} className="composer-option" onMouseEnter={(e) => e.currentTarget.focus()} onClick={onRefresh}>
        <RefreshCw size={14} className="flex-none text-soft" aria-hidden="true" />
        <span className="min-w-0 flex-1 truncate">Refresh</span>
      </button>
      {children.length === 0 && <p className="px-2 py-1.5 text-small text-dim">This folder is empty.</p>}
      {children.map((e) => (
        <button
          key={e.path}
          role="menuitem"
          tabIndex={-1}
          className="composer-option"
          title={e.path}
          onMouseEnter={(ev) => ev.currentTarget.focus()}
          onClick={() => (e.dir ? setLooking(e.path) : onOpen(e.path))}
        >
          {e.dir ? <Folder size={14} className="flex-none text-soft" aria-hidden="true" /> : <FileText size={14} className="flex-none text-soft" aria-hidden="true" />}
          <span className="min-w-0 flex-1 truncate">{baseName(e.path)}</span>
          {e.dir && <ChevronRight size={14} className="flex-none text-dim" aria-hidden="true" />}
        </button>
      ))}
    </div>,
    document.body,
  )
}
