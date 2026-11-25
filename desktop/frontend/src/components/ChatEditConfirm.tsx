import { useCallback, useEffect, useState } from 'react'
import { api, errorMessage } from '../api'
import { changesLabel } from '../chatEdit'
import { relative } from '../chatTools'
import type { FilesRewind } from '../types'

// message is the uuid of the owner's message the thread goes back to before. draftEmpty says whether the composer
// holds no text; onRewound gets the message's text, and whether it replaces the composer's.
type ChatEditConfirmProps = {
  thread: string
  message: string
  folder: string
  draftEmpty(): boolean
  onRewound(text: string, replace: boolean): void
  onClose(): void
}

type Step = { at: 'choose' } | { at: 'files' } | { at: 'draft'; files: boolean }

export function ChatEditConfirm({ thread, message, folder, draftEmpty, onRewound, onClose }: ChatEditConfirmProps) {
  const [preview, setPreview] = useState<FilesRewind | null>(null)
  const [step, setStep] = useState<Step>({ at: 'choose' })
  const [running, setRunning] = useState(false)
  const [error, setError] = useState('')

  // The dry run also wakes a settled thread whose files may be rewound. When it fails, the files are not offered for the
  // reason it gives.
  const load = useCallback(() => {
    setPreview(null)
    api.rewindPreview(thread, message).then(setPreview, (err) => setPreview({ offered: true, reason: errorMessage(err), files: [], insertions: 0, deletions: 0 }))
  }, [thread, message])
  useEffect(load, [load])

  const run = (files: boolean, replace: boolean) => {
    setRunning(true)
    setError('')
    api.rewindTo(thread, message, files).then(
      (text) => onRewound(text, replace),
      (err) => {
        // A message may have come in since the dry run, so what it showed is read again.
        setRunning(false)
        setError(errorMessage(err))
        setStep({ at: 'choose' })
        load()
      },
    )
  }
  const choose = (files: boolean) => (draftEmpty() ? run(files, true) : setStep({ at: 'draft', files }))

  const filesOff = preview === null ? 'Checking the files' : preview.reason || (preview.files.length === 0 ? "Claude's turns changed no file since this message" : '')

  return (
    <div
      className="chat-card mt-2 ml-auto flex max-w-[80%] flex-col gap-2 text-small"
      role="alertdialog"
      aria-label="Edit from here"
      onKeyDown={(e) => {
        if (e.key !== 'Escape') return
        e.stopPropagation()
        onClose()
      }}
    >
      {error && <p className="text-danger">{error}</p>}
      {step.at === 'choose' && (
        <>
          <p className="text-soft">Go back to before this message? The chat drops it and everything after it, and its text comes back to the composer.</p>
          {preview?.offered && preview.reason && <p className="text-dim">{preview.reason}</p>}
          <div className="flex flex-wrap justify-end gap-1.5">
            <button className="btn h-6 px-2" disabled={running} onClick={onClose}>
              Cancel
            </button>
            {(preview === null || preview.offered) && (
              <button className="btn h-6 px-2" disabled={running || filesOff !== ''} title={filesOff || undefined} onClick={() => setStep({ at: 'files' })}>
                Revert files too
              </button>
            )}
            <button className="btn btn-accent h-6 px-2" autoFocus disabled={running} aria-busy={running || undefined} onClick={() => choose(false)}>
              Revert and keep changes
            </button>
          </div>
        </>
      )}
      {step.at === 'files' && preview && (
        <>
          <p className="text-ink">{changesLabel(preview)}</p>
          <ul className="mono max-h-32 overflow-y-auto text-dim select-text">
            {preview.files.map((file) => (
              <li key={file} className="truncate" title={file}>
                {relative(file, folder)}
              </li>
            ))}
          </ul>
          <p className="text-dim">Files changed by commands are not reverted. Edits made outside Claude since then are overwritten.</p>
          <div className="flex flex-wrap justify-end gap-1.5">
            <button className="btn h-6 px-2" disabled={running} onClick={() => setStep({ at: 'choose' })}>
              Back
            </button>
            <button className="btn btn-danger h-6 px-2" autoFocus disabled={running} aria-busy={running || undefined} onClick={() => choose(true)}>
              Revert files and chat
            </button>
          </div>
        </>
      )}
      {step.at === 'draft' && (
        <>
          <p className="text-soft">Replace your draft? The message's text goes in its place.</p>
          <div className="flex flex-wrap justify-end gap-1.5">
            <button className="btn h-6 px-2" disabled={running} onClick={onClose}>
              Cancel
            </button>
            <button className="btn h-6 px-2" disabled={running} onClick={() => run(step.files, false)}>
              Keep my draft
            </button>
            <button className="btn btn-accent h-6 px-2" autoFocus disabled={running} aria-busy={running || undefined} onClick={() => run(step.files, true)}>
              Replace it
            </button>
          </div>
        </>
      )}
    </div>
  )
}
