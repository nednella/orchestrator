import { useEffect, useState } from 'react'
import { createToastTimers } from './toastTimers'
import type { Toast } from './toastPile'

// Dismisses each toast of the pile when its time is over; while paused, no toast loses time.
export function useToastTimers(toasts: Toast[], paused: boolean, dismiss: (key: number) => void) {
  const [timers] = useState(() => createToastTimers(dismiss))
  useEffect(() => () => timers.dispose(), [timers])
  useEffect(() => timers.sync(toasts), [timers, toasts])
  useEffect(() => timers.setPaused(paused), [timers, paused])
}
