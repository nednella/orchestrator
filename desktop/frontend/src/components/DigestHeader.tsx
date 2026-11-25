import { Newspaper, RefreshCw } from 'lucide-react'
import { DigestTile } from './DigestTile'

type DigestHeaderProps = {
  project: string
  automatic: boolean
  running: boolean
  onRun(): void
}

export function DigestHeader({ project, automatic, running, onRun }: DigestHeaderProps) {
  return (
    <header className="flex flex-wrap items-center gap-3">
      <DigestTile size="md">
        <Newspaper size={16} aria-hidden />
      </DigestTile>
      <div className="flex min-w-48 flex-1 flex-col">
        <h1 className="text-title leading-[1.375] font-semibold">News</h1>
        <p className="text-small text-soft">
          What changed in the tools {project} depends on, picked by an agent {automatic ? 'each week' : 'when you run it'}
        </p>
      </div>
      {running ? (
        <button className="btn" disabled>
          <span className="spinner" aria-hidden="true" />
          Checking
        </button>
      ) : (
        <button className="btn" onClick={onRun}>
          <RefreshCw size={14} aria-hidden />
          Run now
        </button>
      )}
    </header>
  )
}
