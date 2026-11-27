import { ArrowDown, ArrowUp } from 'lucide-react'

type BranchBarProps = { ahead: number; behind: number }

const barWidth = (n: number) => (n > 0 ? Math.min(22, Math.round(5 + 8 * Math.log10(n + 1))) : 0)

export function BranchBar({ ahead, behind }: BranchBarProps) {
  return (
    <span
      className="inline-grid grid-cols-[auto_22px_1px_22px_auto] items-center gap-x-1 text-label text-soft tabular-nums"
      title={`${behind} behind, ${ahead} ahead`}
    >
      <span className="inline-flex items-center">
        <ArrowDown size={12} aria-hidden="true" />
        {behind}
      </span>
      <span className="h-[5px] justify-self-end rounded-l-[3px] bg-dim opacity-55" style={{ width: barWidth(behind) }} />
      <span className="h-3 w-px bg-(--border-rule)" />
      <span className="h-[5px] rounded-r-[3px] bg-dim opacity-55" style={{ width: barWidth(ahead) }} />
      <span className="inline-flex items-center">
        {ahead}
        <ArrowUp size={12} aria-hidden="true" />
      </span>
    </span>
  )
}
