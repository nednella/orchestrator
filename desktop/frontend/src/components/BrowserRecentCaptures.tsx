import { useOrchestrator } from '../OrchestratorContext'
import { useLayout } from '../LayoutContext'
import type { Thread } from '../types'

type BrowserRecentCapturesProps = { thread: Thread }

const SHOWN = 3

export function BrowserRecentCaptures({ thread }: BrowserRecentCapturesProps) {
  const { evidence } = useOrchestrator('evidence')
  const { showPanelView } = useLayout()
  const recent = (evidence[thread.id] ?? [])
    .filter((item) => item.kind === 'image')
    .sort((a, b) => b.at - a.at)
    .slice(0, SHOWN)

  return (
    <section>
      <h4 className="pb-2 text-small font-medium text-soft">Recent captures</h4>
      {recent.length === 0 ? (
        <p className="text-small text-dim">No captures yet</p>
      ) : (
        <ul className="grid grid-cols-3 gap-2">
          {recent.map((item) => (
            <li key={item.id} className="min-w-0">
              <button
                className="block aspect-[16/10] w-full overflow-hidden rounded-lg border border-(--border-rule) bg-term"
                title={item.caption || 'Open in Media'}
                onClick={() => showPanelView('media')}
              >
                <img src={item.url} alt={item.caption || 'Capture'} className="h-full w-full object-cover" />
              </button>
            </li>
          ))}
        </ul>
      )}
    </section>
  )
}
