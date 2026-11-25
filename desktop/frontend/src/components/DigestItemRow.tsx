import { BookmarkCheck, ExternalLink, FileText, NotebookPen, X } from 'lucide-react'
import { useAgentos } from '../AgentosContext'
import { api } from '../api'
import { itemDate, itemHost } from '../digestFeed'
import { useClock } from '../time'
import type { DigestItem } from '../types'
import { DigestIconButton } from './DigestIconButton'

type DigestItemRowProps = { item: DigestItem; fresh: boolean }

export function DigestItemRow({ item, fresh }: DigestItemRowProps) {
  const { digestToNote, dismissDigestItem, report } = useAgentos('digestToNote', 'dismissDigestItem', 'report')
  const date = useClock((now) => itemDate(item.at, now))
  const saved = item.noteId !== ''
  const host = itemHost(item.url)

  return (
    <li className="group grid grid-cols-[1.5rem_minmax(0,1fr)_auto] gap-x-2 rounded-lg px-2 py-2.5 transition-colors duration-[120ms] focus-within:bg-[var(--bg-row-hover)] hover:bg-[var(--bg-row-hover)] @max-[40rem]:grid-cols-[1.5rem_minmax(0,1fr)]">
      <span className="grid h-5 place-items-center text-dim">
        <FileText size={16} aria-hidden />
      </span>
      <p className="col-start-2 flex min-h-5 flex-wrap items-center gap-x-2 gap-y-0.5 text-small text-soft">
        {fresh && (
          <span className="inline-flex items-center gap-1 font-medium text-ink">
            <span className="h-1.5 w-1.5 rounded-full bg-accent" aria-hidden="true" />
            New
          </span>
        )}
        {saved && (
          <span className="inline-flex items-center gap-1">
            <BookmarkCheck size={12} className="text-finished" aria-hidden />
            Saved as note
          </span>
        )}
        {(fresh || saved) && host && <span aria-hidden="true">·</span>}
        {host && <span>{host}</span>}
      </p>
      <h3 className="col-start-2 mt-1 text-body leading-[1.45] font-medium">{item.title}</h3>
      {item.why && <p className="col-start-2 mt-0.5 leading-[1.625] text-soft">{item.why}</p>}
      <div className="col-start-3 row-start-1 flex h-5 items-center gap-2 @max-[40rem]:col-start-2 @max-[40rem]:row-start-auto @max-[40rem]:mt-1.5 @max-[40rem]:h-auto @max-[40rem]:flex-row-reverse @max-[40rem]:justify-end">
        <div className="flex gap-0.5 opacity-0 transition-opacity duration-200 group-focus-within:opacity-100 group-hover:opacity-100 @max-[40rem]:opacity-100 [@media(hover:none)]:opacity-100">
          <DigestIconButton label={host ? `Open the source on ${host}` : 'Open the source'} tip="Open source" onClick={() => report(() => api.openURL(item.url))}>
            <ExternalLink size={12} aria-hidden />
          </DigestIconButton>
          {!saved && (
            <DigestIconButton label="Save as note" onClick={() => report(() => digestToNote(item.id))}>
              <NotebookPen size={12} aria-hidden />
            </DigestIconButton>
          )}
          <DigestIconButton label="Dismiss" onClick={() => report(() => dismissDigestItem(item.id))}>
            <X size={12} aria-hidden />
          </DigestIconButton>
        </div>
        <time className="text-small whitespace-nowrap text-soft tabular-nums" dateTime={new Date(item.at).toISOString()}>
          {date}
        </time>
      </div>
    </li>
  )
}
