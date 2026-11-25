import { CircleAlert, RefreshCw } from 'lucide-react'

type DigestFailureProps = { error: string; onRun(): void }

export function DigestFailure({ error, onRun }: DigestFailureProps) {
  return (
    <section className="mt-3 rounded-xl bg-[var(--bg-danger)] shadow-[inset_0_0_0_1px_var(--border-danger)]" role="alert" aria-labelledby="digest-failure">
      <header className="flex min-h-11 items-center border-b border-[var(--border-danger)] px-3.5 py-1.5">
        <h2 id="digest-failure" className="flex min-w-0 items-center gap-2 text-body font-medium break-words">
          <CircleAlert size={14} className="flex-none text-danger" aria-hidden />
          {error || 'The last run did not finish'}
        </h2>
      </header>
      <div className="px-3.5 pt-3 pb-3.5">
        <button className="btn btn-primary" onClick={onRun}>
          <RefreshCw size={14} aria-hidden />
          Run again
        </button>
      </div>
    </section>
  )
}
