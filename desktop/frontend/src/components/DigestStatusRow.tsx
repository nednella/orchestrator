import { CircleAlert, LoaderCircle } from 'lucide-react'
import { formatWorked } from '../chatTools'
import { runState, runStatus, runStatusEnd } from '../digestStatus'
import { useClock } from '../time'
import type { Digest } from '../types'

type DigestStatusRowProps = { digest: Digest }

export function DigestStatusRow({ digest }: DigestStatusRowProps) {
  const text = useClock((now) => runStatus(digest, now, formatWorked))
  const end = useClock((now) => runStatusEnd(digest, now))
  const state = runState(digest)

  return (
    <div className="mt-4 @max-[40rem]:mt-3 flex min-h-6 flex-wrap items-center gap-2 border-b border-[var(--border-rule)] px-1 pt-1 pb-2 text-body leading-[1.625] text-soft tabular-nums" role={state === 'running' ? 'status' : undefined}>
      {state === 'running' && <LoaderCircle size={16} className="animate-spin text-dim" aria-hidden />}
      {state === 'failed' && <CircleAlert size={16} className="text-danger" aria-hidden />}
      <span>{text}</span>
      <span className="ml-auto text-small whitespace-nowrap">{end}</span>
    </div>
  )
}
