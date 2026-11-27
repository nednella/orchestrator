import { CircleAlert, CircleCheck, GitPullRequest, Info, TriangleAlert, WifiOff, X } from 'lucide-react'
import { useLayoutEffect, useRef } from 'react'
import type { CSSProperties, ReactNode } from 'react'
import type { ShownToast } from '../toastLeaving'
import { Spinner } from './Spinner'
import { StateDot } from './StateDot'

const GLYPH_SIZE = 16

const GLYPH: Record<ShownToast['tone'], { color: string; glyph: ReactNode }> = {
  error: { color: 'text-danger', glyph: <CircleAlert size={GLYPH_SIZE} /> },
  pr: { color: 'text-danger', glyph: <GitPullRequest size={GLYPH_SIZE} /> },
  network: { color: 'text-danger', glyph: <WifiOff size={GLYPH_SIZE} /> },
  warning: { color: 'text-idle', glyph: <TriangleAlert size={GLYPH_SIZE} /> },
  success: { color: 'text-finished', glyph: <CircleCheck size={GLYPH_SIZE} /> },
  info: { color: 'text-accent', glyph: <Info size={GLYPH_SIZE} /> },
  evidence: { color: 'text-accent', glyph: <Info size={GLYPH_SIZE} /> },
  waiting: { color: 'text-waiting', glyph: <StateDot state="waiting" /> },
  replied: { color: 'text-idle', glyph: <StateDot state="idle" /> },
  opened: { color: 'text-idle', glyph: <StateDot state="idle" /> },
}

const JUMP_LABEL: Partial<Record<ShownToast['tone'], string>> = { replied: 'Open', opened: 'Open', evidence: 'View' }

type ToastCardProps = {
  toast: ShownToast
  // How far back in the pile the toast is: 0 is the front.
  depth: number
  onDismiss: (key: number) => void
  onJump: (toast: ShownToast) => void
}

export function ToastCard({ toast, depth, onDismiss, onJump }: ToastCardProps) {
  const { color, glyph } = GLYPH[toast.tone]
  // A leaving toast gives back the room it took in the open pile, so the toasts after it slide into place.
  const card = useRef<HTMLDivElement>(null)
  const height = useRef(0)
  useLayoutEffect(() => {
    if (!toast.leaving && card.current) height.current = card.current.offsetHeight
  })
  const leaving = toast.leaving ? ({ '--toast-h': `${height.current}px` } as CSSProperties) : undefined
  return (
    <div ref={card} className="toast" data-depth={depth} data-leaving={toast.leaving || undefined} aria-hidden={toast.leaving || undefined} style={leaving}>
      <span className={`flex h-5 w-5 flex-none items-center justify-center ${color}`}>{toast.busy ? <Spinner size={16} /> : glyph}</span>
      <span className="flex min-w-0 flex-1 flex-col">
        <span className="text-body font-medium">{toast.text}</span>
        {toast.detail && <span className="text-small break-words text-soft">{toast.detail}</span>}
      </span>
      {toast.threadId && (
        <button className="btn btn-primary h-7 rounded-full" onClick={() => onJump(toast)}>
          {JUMP_LABEL[toast.tone] ?? 'Jump'}
        </button>
      )}
      {toast.action && (
        <button className="btn btn-primary h-7 rounded-full" onClick={toast.action.run}>
          {toast.action.label}
        </button>
      )}
      <button className="toast-x btn btn-ghost h-7 w-7 flex-none justify-center rounded-full px-0" aria-label="Dismiss" onClick={() => onDismiss(toast.key)}>
        <X size={12} aria-hidden="true" />
      </button>
    </div>
  )
}
