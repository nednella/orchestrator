import { WorkingTime } from './WorkingTime'

type ThreadWorkingProps = { since: number }

// ThreadWorking is the card's mark of a thread at work: a dashed ring that turns, and how long it has worked. It
// takes the age's place, so the selected card's close button replaces it the same way.
export function ThreadWorking({ since }: ThreadWorkingProps) {
  return (
    <span className="thread-age thread-dim">
      <span className="flex items-center gap-1.5 whitespace-nowrap text-label text-working tabular-nums">
        <svg viewBox="0 0 24 24" width={14} height={14} className="thread-working ui-icon" fill="none" stroke="currentColor" strokeWidth={2.5} strokeLinecap="round" strokeDasharray="3.5 4.1" aria-hidden="true">
          <circle cx="12" cy="12" r="9" />
        </svg>
        <span>
          Working <WorkingTime since={since} />
        </span>
      </span>
    </span>
  )
}
