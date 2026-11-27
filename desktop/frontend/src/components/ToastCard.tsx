import { CircleAlert, CircleCheck, GitPullRequest, Info, TriangleAlert, WifiOff } from 'lucide-react'
import type { ReactNode } from 'react'
import type { Toast } from '../toastPile'
import { Icon } from './Icon'
import { Spinner } from './Spinner'
import { StateDot } from './StateDot'

const GLYPH_SIZE = 16

const GLYPH: Record<Toast['tone'], { color: string; glyph: ReactNode }> = {
  error: { color: 'text-danger', glyph: <CircleAlert size={GLYPH_SIZE} /> },
  pr: { color: 'text-danger', glyph: <GitPullRequest size={GLYPH_SIZE} /> },
  network: { color: 'text-idle', glyph: <WifiOff size={GLYPH_SIZE} /> },
  warning: { color: 'text-idle', glyph: <TriangleAlert size={GLYPH_SIZE} /> },
  success: { color: 'text-finished', glyph: <CircleCheck size={GLYPH_SIZE} /> },
  done: { color: 'text-finished', glyph: <CircleCheck size={GLYPH_SIZE} /> },
  info: { color: 'text-accent', glyph: <Info size={GLYPH_SIZE} /> },
  evidence: { color: 'text-accent', glyph: <Info size={GLYPH_SIZE} /> },
  waiting: { color: 'text-waiting', glyph: <StateDot state="waiting" /> },
  replied: { color: 'text-idle', glyph: <StateDot state="idle" /> },
  opened: { color: 'text-idle', glyph: <StateDot state="idle" /> },
}

const JUMP_LABEL: Partial<Record<Toast['tone'], string>> = { replied: 'Open', opened: 'Open', evidence: 'View' }

type ToastCardProps = {
  toast: Toast
  // How far back in the pile the toast is: 0 is the front.
  depth: number
  onDismiss: (key: number) => void
  onJump: (toast: Toast) => void
}

export function ToastCard({ toast, depth, onDismiss, onJump }: ToastCardProps) {
  const { color, glyph } = GLYPH[toast.tone]
  return (
    <div className="toast" data-depth={depth}>
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
        <Icon name="close" size={12} />
      </button>
    </div>
  )
}
