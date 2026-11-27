import { useEffect, useState } from 'react'
import { useOrchestrator } from '../OrchestratorContext'
import { api, errorMessage } from '../api'
import type { IssueType, Release } from '../types'
import { Keycap } from './Keycap'
import { Overlay } from './Overlay'
import { TypeMark } from './TypeMark'

const SECTION_TYPE: Record<string, IssueType> = { Features: 'feature', 'Bug Fixes': 'bug' }

export function WhatsNew() {
  const { overlay, setOverlay, startNotes } = useOrchestrator('overlay', 'setOverlay', 'startNotes')
  if (overlay !== 'changelog') return null
  return <WhatsNewDialog opened={startNotes[0]} onClose={() => setOverlay(null)} />
}

type WhatsNewDialogProps = { opened?: Release; onClose(): void }

function WhatsNewDialog({ opened, onClose }: WhatsNewDialogProps) {
  const { report } = useOrchestrator('report')
  const [release, setRelease] = useState(opened)
  const [error, setError] = useState('')

  useEffect(() => {
    if (opened) return
    let current = true
    api.releaseNotes().then(
      (r) => current && setRelease(r),
      (e) => current && setError(errorMessage(e)),
    )
    return () => {
      current = false
    }
  }, [opened])

  return (
    <Overlay size="reading" label="What's new" onClose={onClose}>
      <div className="flex flex-none items-center gap-3 border-b border-line px-4 py-2.5">
        <h2 className="flex-none text-title font-semibold">{release ? `What's new in ${release.version}` : "What's new"}</h2>
        <span className="flex-1" />
        <Keycap>Esc</Keycap>
      </div>
      <div className={`min-h-0 overflow-y-auto px-5 py-3 ${error ? '' : 'h-[min(26rem,60vh)]'}`}>
        {error && <p className="text-body text-dim">{error}</p>}
        {release?.sections.map((section) => (
          <section key={section.title} className="pb-4">
            <h3 className="label pb-1">{section.title}</h3>
            <ul>
              {section.changes.map((change, i) => (
                <li key={i} className="flex items-baseline gap-3 border-b border-line py-1.5 last:border-b-0">
                  <span className="flex-none self-center">
                    <TypeMark type={SECTION_TYPE[section.title] ?? 'chore'} />
                  </span>
                  <span className="mono w-16 flex-none truncate text-label text-dim">{change.scope}</span>
                  <span className="min-w-0 flex-1 text-body">{change.text}</span>
                  {change.url && (
                    <button className="link mono flex-none text-label" onClick={() => report(() => api.openURL(change.url))}>
                      #{change.issue}
                    </button>
                  )}
                </li>
              ))}
            </ul>
          </section>
        ))}
      </div>
    </Overlay>
  )
}
