import { useEffect, useState } from 'react'
import { TOAST_EXIT_MS, withLeaving, withoutLeaving } from './toastLeaving'
import type { ShownToast } from './toastLeaving'
import type { Toast } from './toastPile'

// The toasts to draw: those of the pile, and the ones that just went, until their exit is over.
export function useLeavingToasts(toasts: Toast[]): ShownToast[] {
  const [shown, setShown] = useState<ShownToast[]>(toasts)
  const [seen, setSeen] = useState(toasts)
  if (seen !== toasts) {
    setSeen(toasts)
    setShown(withLeaving(shown, toasts))
  }
  const leaving = shown.some((toast) => toast.leaving)
  useEffect(() => {
    if (!leaving) return
    const timer = setTimeout(() => setShown(withoutLeaving), TOAST_EXIT_MS)
    return () => clearTimeout(timer)
  }, [shown, leaving])
  return shown
}
