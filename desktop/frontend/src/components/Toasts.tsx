import { useEffect, useRef, useState } from 'react'
import { useLayout } from '../LayoutContext'
import { useOrchestrator } from '../OrchestratorContext'
import type { Toast } from '../toastPile'
import { useLeavingToasts } from '../useLeavingToasts'
import { useToastTimers } from '../useToastTimers'
import { ToastCard } from './ToastCard'

export function Toasts() {
  const { toasts, select, dismissToast } = useOrchestrator('toasts', 'select', 'dismissToast')
  const { showPanelView } = useLayout()
  const stack = useRef<HTMLDivElement>(null)
  const [hovered, setHovered] = useState(false)
  const [focused, setFocused] = useState(false)
  const open = hovered || focused
  useToastTimers(toasts, open, dismissToast)
  const shown = useLeavingToasts(toasts)

  // A toast that goes while the pointer or the focus is on it sends no event of leaving.
  useEffect(() => {
    if (toasts.length === 0) setHovered(false)
    setFocused(stack.current?.contains(document.activeElement) ?? false)
  }, [toasts])

  const jump = ({ key, threadId, view }: Toast) => {
    if (!threadId) return
    select(threadId)
    if (view) showPanelView(view, { thread: threadId })
    dismissToast(key)
  }
  const clearAll = () => toasts.forEach((toast) => dismissToast(toast.key))

  return (
    <div className="toast-layer" aria-live="polite">
      {shown.length > 0 && (
        <div
          ref={stack}
          className="toast-stack"
          onMouseEnter={() => setHovered(true)}
          onMouseLeave={() => setHovered(false)}
          onFocus={() => setFocused(true)}
          onBlur={(e) => setFocused(e.currentTarget.contains(e.relatedTarget))}
        >
          <div className="toast-pile" data-open={open || undefined} data-count={shown.length}>
            {[...shown].reverse().map((toast, depth) => (
              <ToastCard key={toast.key} toast={toast} depth={depth} onDismiss={dismissToast} onJump={jump} />
            ))}
          </div>
          {open && toasts.length > 1 && (
            <div className="toast-foot">
              <span>{toasts.length} notifications</span>
              <button className="btn btn-ghost h-7 rounded-full" onClick={clearAll}>
                Clear all
              </button>
            </div>
          )}
        </div>
      )}
    </div>
  )
}
