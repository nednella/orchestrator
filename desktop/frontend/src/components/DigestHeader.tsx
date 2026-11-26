import { Newspaper, RefreshCw } from 'lucide-react'
import { DigestTile } from './DigestTile'
import { Spinner } from './Spinner'

// button is the header's own: Run now once the digest has items, Checking while a run goes, none otherwise,
// since the empty and failed states carry their own.
type DigestHeaderProps = {
  project: string
  tools: number
  automatic: boolean
  button: 'run' | 'checking' | null
  onRun(): void
}

export function DigestHeader({ project, tools, automatic, button, onRun }: DigestHeaderProps) {
  const checked = tools === 0 ? 'the tools' : `the ${tools} ${tools === 1 ? 'tool' : 'tools'}`
  return (
    <header className="flex flex-wrap items-center gap-3">
      <DigestTile size="md">
        <Newspaper size={16} aria-hidden />
      </DigestTile>
      <div className="flex min-w-48 flex-1 flex-col">
        <h1 className="text-title leading-[1.375] font-semibold">News</h1>
        <p className="text-small text-soft">
          What changed in {checked} {project} depends on, picked by an agent {automatic ? 'each week' : 'when you run it'}
        </p>
      </div>
      {button === 'checking' && (
        <button className="btn" disabled>
          <Spinner size={14} />
          Checking
        </button>
      )}
      {button === 'run' && (
        <button className="btn" onClick={onRun}>
          <RefreshCw size={14} aria-hidden />
          Run now
        </button>
      )}
    </header>
  )
}
