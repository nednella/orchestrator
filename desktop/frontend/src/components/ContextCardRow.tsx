import { Square } from 'lucide-react'
import type { MouseEvent, ReactNode } from 'react'
import type { State } from '../types'
import { StateDot } from './StateDot'

// label wraps to two lines when wrap is set; meta is a muted line under it. dot is a thread's state on the icon, and
// stop names what the row's stop button stops.
type ContextCardRowProps = {
  icon: ReactNode
  label: ReactNode
  end?: ReactNode
  meta?: ReactNode
  wrap?: boolean
  title?: string
  dot?: State
  onClick?(): void
  onContextMenu?(e: MouseEvent): void
  stop?: { label: string; run(): void }
}

export function ContextCardRow({ icon, label, end, meta, wrap = false, title, dot, onClick, onContextMenu, stop }: ContextCardRowProps) {
  const body = (
    <>
      <span className={`flex min-w-0 gap-2.5 text-[0.8125rem] leading-5 font-medium text-(--text-chip) ${wrap ? 'items-start' : 'items-center'}`}>
        <span className="relative flex h-5 flex-none items-center text-soft">
          {icon}
          {dot && (
            <span className="absolute -right-1 bottom-0 flex rounded-full bg-(--bg-composer) p-[2px]">
              <StateDot state={dot} />
            </span>
          )}
        </span>
        <span className={`min-w-0 flex-1 ${wrap ? 'line-clamp-2' : 'flex'}`}>{typeof label === 'string' && !wrap ? <span className="truncate">{label}</span> : label}</span>
        {end && <span className="flex flex-none items-center gap-1 text-small font-normal text-soft">{end}</span>}
      </span>
      {meta && <span className="flex min-w-0 items-center gap-1.5 pl-[26px] text-small text-soft">{meta}</span>}
    </>
  )
  const inner = 'flex min-h-8 min-w-0 flex-1 flex-col justify-center gap-0.5 rounded-lg px-2.5 py-1.5 text-left'
  return (
    <div className={`flex min-h-8 items-center rounded-lg ${onClick ? 'hover:bg-(--bg-row-hover)' : ''}`} title={title} onContextMenu={onContextMenu}>
      {onClick ? (
        <button className={`${inner} focus-visible:outline-2 focus-visible:-outline-offset-2 active:scale-[0.97] motion-reduce:active:scale-100`} onClick={onClick}>
          {body}
        </button>
      ) : (
        <div className={inner}>{body}</div>
      )}
      {stop && (
        <button className="btn btn-ghost mr-1 h-7 w-7 flex-none justify-center px-0 text-danger" title={stop.label} aria-label={stop.label} onClick={stop.run}>
          <Square size={12} fill="currentColor" aria-hidden="true" />
        </button>
      )}
    </div>
  )
}
