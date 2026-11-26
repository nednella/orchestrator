import { ChevronRight } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import { crumbsOf } from '../fileTree'
import type { Point } from '../useFloatingAt'
import type { FileEntry } from '../types'
import { FileFolderMenu } from './FileFolderMenu'

type FileCrumbsProps = {
  project: string
  path: string
  entries: FileEntry[]
  onOpen(path: string): void
  onReveal(path: string): void
  onRefresh(): void
}

// FileCrumbs is the way to the open file: the project's name and each folder's, with a button after each that lists
// what the folder holds, and the file's name.
export function FileCrumbs({ project, path, entries, onOpen, onReveal, onRefresh }: FileCrumbsProps) {
  const [menu, setMenu] = useState<{ folder: string; at: Point } | null>(null)
  const nav = useRef<HTMLElement>(null)
  // A press on a crumb's button while its menu is open closes the menu on mousedown; the click that follows must not
  // open it again.
  const closing = useRef<string | null>(null)

  useEffect(() => {
    nav.current?.scrollTo({ left: nav.current.scrollWidth })
  }, [path])

  const crumbs = crumbsOf(path)
  const chevron = (folder: string, label: string) => (
    <button
      className="btn btn-ghost h-6 w-5 flex-none justify-center px-0 text-dim"
      aria-label={`Files in ${label}`}
      aria-haspopup="menu"
      aria-expanded={menu?.folder === folder}
      onMouseDown={() => {
        closing.current = menu?.folder === folder ? folder : null
      }}
      onClick={(e) => {
        if (closing.current === folder) {
          closing.current = null
          return
        }
        const box = e.currentTarget.getBoundingClientRect()
        setMenu({ folder, at: { x: box.left, y: box.bottom + 4 } })
      }}
    >
      <ChevronRight size={14} aria-hidden="true" />
    </button>
  )

  return (
    <nav ref={nav} aria-label="Path" className="file-crumbs flex min-w-0 flex-1 items-center text-small">
      <button className="btn btn-ghost h-6 flex-none px-1.5 text-soft" title={project} onClick={() => onReveal('')}>
        <span className="max-w-32 truncate">{project}</span>
      </button>
      {chevron('', project)}
      {crumbs.map((c, i) => (
        <span key={`${i}:${c.name}`} className="flex flex-none items-center">
          <button className={`btn btn-ghost h-6 px-1.5 ${c.folder === '' ? 'font-medium text-ink' : 'text-soft'}`} onClick={() => onReveal(c.folder === '' ? path : c.folder)}>
            {c.name}
          </button>
          {c.folder !== '' && chevron(c.folder, c.name)}
        </span>
      ))}
      {menu && (
        <FileFolderMenu
          at={menu.at}
          folder={menu.folder}
          entries={entries}
          onOpen={(p) => {
            setMenu(null)
            onOpen(p)
          }}
          onRefresh={onRefresh}
          onClose={() => setMenu(null)}
        />
      )}
    </nav>
  )
}
