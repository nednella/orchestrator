import { useId, type ReactNode } from 'react'
import type { LucideIcon } from 'lucide-react'

type StatsCardProps = { icon: LucideIcon; title: string; meta?: string; end?: ReactNode; children: ReactNode }

export function StatsCard({ icon: CardIcon, title, meta, end, children }: StatsCardProps) {
  const heading = useId()
  return (
    <section aria-labelledby={heading} className="min-w-0 rounded-xl bg-(--bg-card) shadow-[inset_0_0_0_1px_var(--border)]">
      <header className="flex min-h-11 flex-wrap items-center gap-x-2 gap-y-1 border-b border-line px-3.5 py-1.5">
        <h2 id={heading} className="flex items-center gap-2 text-body font-medium">
          <CardIcon size={14} className="flex-none text-dim" aria-hidden="true" />
          {title}
        </h2>
        {meta && <span className="text-small text-soft">{meta}</span>}
        {end && <div className="ml-auto flex items-center gap-1">{end}</div>}
      </header>
      <div className="px-3.5 pt-3 pb-3.5">{children}</div>
    </section>
  )
}
