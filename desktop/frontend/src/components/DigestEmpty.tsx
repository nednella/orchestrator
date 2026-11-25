import { Newspaper, RefreshCw } from 'lucide-react'
import { DigestTile } from './DigestTile'

type DigestEmptyProps = { onRun(): void }

export function DigestEmpty({ onRun }: DigestEmptyProps) {
  return (
    <section className="mt-6 flex flex-col items-center gap-2 rounded-xl bg-[var(--bg-card)] px-6 py-10 text-center shadow-[inset_0_0_0_1px_var(--border)]" aria-labelledby="digest-empty">
      <DigestTile size="lg">
        <Newspaper size={20} aria-hidden />
      </DigestTile>
      <h2 id="digest-empty" className="text-body font-semibold">
        Nothing here yet
      </h2>
      <p className="max-w-md leading-[1.625] text-soft">
        Once a week an agent reads what changed in the tools this project depends on and keeps the few items that matter here. Run it now to see how it reads.
      </p>
      <button className="btn btn-primary mt-3" onClick={onRun}>
        <RefreshCw size={14} aria-hidden />
        Run now
      </button>
    </section>
  )
}
