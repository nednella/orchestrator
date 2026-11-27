import type { PanelViewId } from './panelTabs'

export type Toast = {
  key: number
  // A toast with the id of one shown replaces it; without one, the same tone, text and detail do.
  id?: string
  tone: 'waiting' | 'replied' | 'opened' | 'pr' | 'evidence' | 'error' | 'info' | 'done' | 'success' | 'warning' | 'network'
  text: string
  detail?: string
  threadId?: string
  view?: PanelViewId
  sticky?: boolean
  busy?: boolean
  action?: { label: string; run(): void }
}

export type ToastInput = Omit<Toast, 'key'>

export const MAX_TOASTS = 4
export const TOAST_MS = 6000

const identity = (toast: ToastInput) => toast.id ?? `${toast.tone}\n${toast.text}\n${toast.detail ?? ''}`

// addToast puts the toast in front of the pile, which is the end of the list. A repeat of a toast in the pile takes that
// toast's place and key; the pile then drops its oldest toast that is not sticky until it is at most MAX_TOASTS.
export function addToast(list: Toast[], input: ToastInput, newKey: number): { list: Toast[]; key: number } {
  const repeat = list.find((toast) => identity(toast) === identity(input))
  const key = repeat?.key ?? newKey
  const next = [...list.filter((toast) => toast !== repeat), { ...input, key }]
  while (next.length > MAX_TOASTS) {
    const oldest = next.findIndex((toast, i) => !toast.sticky && i < next.length - 1)
    next.splice(Math.max(oldest, 0), 1)
  }
  return { list: next, key }
}
