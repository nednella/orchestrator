import { useLayoutEffect, useRef } from 'react'
import { formatWorked } from '../chatTools'
import { onTick } from '../time'

type WorkingTimeProps = { since: number }

// WorkingTime is how long something has worked since then. It writes the time straight into the page at each tick of
// the clock, so the row that holds it never renders for it.
export function WorkingTime({ since }: WorkingTimeProps) {
  const time = useRef<HTMLSpanElement>(null)
  useLayoutEffect(() => {
    const show = (now: number) => {
      if (time.current) time.current.textContent = formatWorked(now - since)
    }
    show(Date.now())
    return onTick(show)
  }, [since])
  return <span ref={time} />
}
