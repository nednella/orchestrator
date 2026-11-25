import { Check, LoaderCircle } from 'lucide-react'
import { itemHost } from '../digestFeed'
import type { DigestItem } from '../types'

// found is what the run has handed in so far; the last line stands for the reading still going on.
type DigestRunLinesProps = { found: DigestItem[] }

export function DigestRunLines({ found }: DigestRunLinesProps) {
  return (
    <ul className="flex flex-col pt-1.5 pb-1" aria-label="What the run has found so far">
      {found.map((item) => (
        <li key={item.id} className="flex min-h-6 min-w-0 items-center gap-1.5 p-0.5 leading-[1.625] text-soft">
          <span className="grid w-6 flex-none place-items-center text-finished">
            <Check size={16} aria-hidden />
          </span>
          <span className="min-w-0 truncate">Found {item.title}</span>
          <span className="flex-none font-mono text-small">{itemHost(item.url)}</span>
        </li>
      ))}
      <li className="flex min-h-6 items-center gap-1.5 p-0.5 leading-[1.625] text-soft">
        <span className="grid w-6 flex-none place-items-center text-dim">
          <LoaderCircle size={16} className="animate-spin" aria-hidden />
        </span>
        <span>Reading what changed</span>
      </li>
    </ul>
  )
}
