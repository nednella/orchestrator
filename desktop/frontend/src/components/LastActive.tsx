import { shortAgo } from '../threads'
import { useClock } from '../time'

type LastActiveProps = { at: number }

export function LastActive({ at }: LastActiveProps) {
  return <>{useClock((now) => shortAgo(at, now))}</>
}
