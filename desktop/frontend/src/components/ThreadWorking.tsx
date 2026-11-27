import { WorkingTime } from './WorkingTime'

type ThreadWorkingProps = { since: number }

// ThreadWorking is how long the card's thread has worked; the state icon beside the title is what turns. It takes the
// age's place, so the selected card's close button replaces it the same way.
export function ThreadWorking({ since }: ThreadWorkingProps) {
  return (
    <span className="thread-age thread-dim">
      <span className="whitespace-nowrap text-label text-working tabular-nums">
        Working <WorkingTime since={since} />
      </span>
    </span>
  )
}
