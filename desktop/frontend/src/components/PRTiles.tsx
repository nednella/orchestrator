import { CircleCheck, CircleDot, CircleX, Eye, FileDiff, MessageSquare, MessagesSquare } from 'lucide-react'
import type { LucideIcon } from 'lucide-react'
import type { ReactNode } from 'react'
import type { PR } from '../types'

type PRTilesProps = { pr: PR }

const REVIEW: Record<PR['review'], { icon: LucideIcon; color: string; label: string }> = {
  approved: { icon: CircleCheck, color: 'var(--finished)', label: 'Approved' },
  changes: { icon: FileDiff, color: 'var(--danger)', label: 'Changes requested' },
  commented: { icon: MessageSquare, color: 'var(--text-dim)', label: 'Commented' },
  '': { icon: Eye, color: 'var(--text-dim)', label: 'No review yet' },
}

export function PRTiles({ pr }: PRTilesProps) {
  const failing = pr.failedChecks.length
  const checks: { icon: LucideIcon; color: string; n: number } =
    failing > 0
      ? { icon: CircleX, color: 'var(--danger)', n: failing }
      : pr.runningChecks > 0
        ? { icon: CircleDot, color: 'var(--idle)', n: pr.runningChecks }
        : { icon: CircleCheck, color: 'var(--finished)', n: pr.passedChecks }
  const review = REVIEW[pr.review]

  return (
    <div className="flex flex-col gap-2 px-4 pb-3">
      <div className="grid grid-cols-3 gap-2">
        <Tile label="Checks" icon={checks.icon} color={checks.color}>
          {checks.n}
        </Tile>
        <Tile label="Review" icon={review.icon} color={review.color} title={review.label}>
          {review.label}
        </Tile>
        <Tile label="Comments" icon={MessagesSquare} color={pr.newComments > 0 ? 'var(--idle)' : 'var(--text-dim)'}>
          {pr.newComments}
        </Tile>
      </div>
      {failing > 0 && (
        <ul className="flex flex-wrap gap-1.5" aria-label="Failed checks">
          {pr.failedChecks.map((name) => (
            <li key={name} className="inline-flex h-6 items-center gap-1.5 rounded-full border border-(--border-rule) bg-(--bg-card) pr-2.5 pl-2 text-label text-soft">
              <span className="flex text-danger">
                <CircleX size={12} aria-hidden="true" />
              </span>
              {name}
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}

type TileProps = { label: string; icon: LucideIcon; color: string; title?: string; children: ReactNode }

function Tile({ label, icon: Glyph, color, title, children }: TileProps) {
  return (
    <div className="flex min-w-0 flex-col gap-0.5 rounded-xl border border-(--border-rule) bg-(--bg-card) px-3 py-2" title={title}>
      <span className="text-label text-dim">{label}</span>
      <span className="flex min-w-0 items-center gap-1.5 text-small font-medium text-ink tabular-nums">
        <span className="flex flex-none" style={{ color }}>
          <Glyph size={14} aria-hidden="true" />
        </span>
        <span className="truncate">{children}</span>
      </span>
    </div>
  )
}
