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
    <header className="grid grid-cols-[auto_minmax(0,1fr)_auto] items-center gap-x-3 gap-y-1 @max-[40rem]:gap-x-2">
      <DigestTile size="md">
        <Newspaper size={16} aria-hidden />
      </DigestTile>
      <h1 className="col-start-2 text-title leading-[1.375] font-semibold">News</h1>
      <p className="col-start-2 row-start-2 text-small text-soft @max-[40rem]:col-span-full @max-[40rem]:col-start-1 @max-[40rem]:mt-1">
        What changed in {checked} {project} depends on, picked by an agent {automatic ? 'each week' : 'when you run it'}
      </p>
      <div className="col-start-3 row-span-2 row-start-1 @max-[40rem]:row-span-1">
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
      </div>
    </header>
  )
}
