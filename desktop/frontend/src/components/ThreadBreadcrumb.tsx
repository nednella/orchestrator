import { useRef } from 'react'
import type { CSSProperties } from 'react'
import { useFitsWithin } from '../useFitsWithin'

const noDrag = { '--wails-draggable': 'no-drag' } as CSSProperties
const crumb = 'max-w-[12rem] min-w-8 shrink truncate'
const separator = (
  <span className="flex-none text-dim" aria-hidden="true">
    /
  </span>
)

// parent is the title of the view above the one shown, null when none is.
type ThreadBreadcrumbProps = { project: string; onShowProject(): void; parent: string | null; onOpenParent(): void; title: string }

// ThreadBreadcrumb is project / parent / the view shown. The parent gives way to "..." when the bar cannot show the
// three in full, measured on a hidden copy of them, so the collapse does not change what it measures.
export function ThreadBreadcrumb({ project, onShowProject, parent, onOpenParent, title }: ThreadBreadcrumbProps) {
  const nav = useRef<HTMLElement>(null)
  const full = useRef<HTMLSpanElement>(null)
  const fits = useFitsWithin(nav, full)

  return (
    <nav ref={nav} aria-label="Breadcrumb" className="relative flex min-w-0 flex-1 items-center gap-2 text-body">
      <button className={`${crumb} rounded-sm text-soft hover:text-ink`} style={noDrag} title={`Show ${project} home`} onClick={onShowProject}>
        {project}
      </button>
      {separator}
      {parent !== null && (
        <>
          <button className={`${fits ? crumb : 'flex-none'} rounded-sm text-soft hover:text-ink`} style={noDrag} title={parent} aria-label={`Open parent thread: ${parent}`} onClick={onOpenParent}>
            {fits ? parent : '...'}
          </button>
          {separator}
        </>
      )}
      <span className="min-w-0 truncate" aria-current="page" title={title}>
        {title}
      </span>
      {/* The copy lies in a box the bar's size that clips it, so a copy wider than the bar overflows nothing. */}
      <span aria-hidden="true" className="pointer-events-none invisible absolute inset-0 overflow-hidden">
        <span ref={full} className="flex w-max items-center gap-2">
          <span className={crumb}>{project}</span>
          {separator}
          {parent !== null && (
            <>
              <span className={crumb}>{parent}</span>
              {separator}
            </>
          )}
          <span>{title}</span>
        </span>
      </span>
    </nav>
  )
}
