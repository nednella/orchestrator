import { useEffect, useRef } from 'react'

// The issue asked to be read in the Issues view, kept until the view takes it, so an ask made as the view opens holds
// while it mounts.
let asked: number | null = null
const listeners = new Set<() => void>()

export function askToReadIssue(number: number) {
  asked = number
  listeners.forEach((listener) => listener())
}

// useIssueReadRequest calls read with each issue asked for while the view is mounted, and with one asked just before.
export function useIssueReadRequest(read: (number: number) => void) {
  const latest = useRef(read)
  latest.current = read
  useEffect(() => {
    const take = () => {
      if (asked === null) return
      const number = asked
      asked = null
      latest.current(number)
    }
    take()
    listeners.add(take)
    return () => {
      listeners.delete(take)
    }
  }, [])
}
