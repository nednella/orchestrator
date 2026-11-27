import type { Toast } from './toastPile'

// A toast of the pile that has gone from the list but is still on screen while it fades out.
export type ShownToast = Toast & { leaving?: boolean }

export const TOAST_EXIT_MS = 200

// withLeaving is the pile to draw: the toasts in their order, and each toast that left since the last draw kept
// where it was, after the toast that was before it.
export function withLeaving(shown: ShownToast[], toasts: Toast[]): ShownToast[] {
  const live = new Set(toasts.map((toast) => toast.key))
  const next: ShownToast[] = [...toasts]
  shown.forEach((toast, i) => {
    if (live.has(toast.key)) return
    const before = shown.slice(0, i).reverse().find((t) => next.some((n) => n.key === t.key))
    const at = before ? next.findIndex((n) => n.key === before.key) + 1 : 0
    next.splice(at, 0, { ...toast, leaving: true })
  })
  return next
}

export const withoutLeaving = (shown: ShownToast[]): ShownToast[] => shown.filter((toast) => !toast.leaving)
