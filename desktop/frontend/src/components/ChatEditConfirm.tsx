import { useEffect, useState } from 'react'
import { api, errorMessage } from '../api'
import { changesLabel } from '../chatEdit'
import { relative } from '../chatTools'
import type { FilesRewind } from '../types'

// message is the uuid of the owner's message the thread goes back to before. worktree is the thread's own worktree,
// '' for a thread in the project folder, whose files are never rewound. draftEmpty says whether the composer holds no
// text; onRewound gets the message's text, and whether it replaces the composer's.
type ChatEditConfirmProps = {
  thread: string
  message: string
  worktree: string
  draftEmpty(): boolean
  onRewound(text: string, replace: boolean): void
  onClose(): void
}

type Step = { at: 'choose' } | { at: 'files' } | { at: 'draft'; files: boolean }

const noFiles: FilesRewind = { offered: false, reason: '', files: [], insertions: 0, deletions: 0 }

export function ChatEditConfirm({ thread, message, worktree, draftEmpty, onRewound, onClose }: ChatEditConfirmProps) {
  // offer says whether the files may be rewound, with no dry run; preview is the dry run, read once the owner picks
  // the files.
  const [offer, setOffer] = useState<FilesRewind | null>(worktree ? null : noFiles)
  const [preview, setPreview] = useState<FilesRewind | null>(null)
  const [step, setStep] = useState<Step>({ at: 'choose' })
  const [running, setRunning] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    if (worktree) api.rewindFilesOffer(thread).then(setOffer, (err) => setOffer({ ...noFiles, offered: true, reason: errorMessage(err) }))
  }, [thread, worktree])

  // The dry run also wakes a settled thread. When it fails, the files are not rewound for the reason it gives.
  const showFiles = () => {
    setStep({ at: 'files' })
    setPreview(null)
    api.rewindPreview(thread, message).then(setPreview, (err) => setPreview({ ...noFiles, offered: true, reason: errorMessage(err) }))
  }

  const run = (files: boolean, replace: boolean) => {
    setRunning(true)
    setError('')
    api.rewindTo(thread, message, files).then(
      (text) => onRewound(text, replace),
      (err) => {
        setRunning(false)
        setError(errorMessage(err))
        setStep({ at: 'choose' })
      },
    )
  }
  const choose = (files: boolean) => (draftEmpty() ? run(files, true) : setStep({ at: 'draft', files }))

  const filesOff = offer === null ? 'Checking the files' : offer.reason
  const previewOff = preview === null ? 'Checking the files' : preview.reason || (preview.files.length === 0 ? "Claude's turns changed no file since this message" : '')

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
          {offer?.offered && offer.reason && <p className="text-dim">{offer.reason}</p>}
          <div className="flex flex-wrap justify-end gap-1.5">
            <button className="btn h-6 px-2" disabled={running} onClick={onClose}>
              Cancel
            </button>
            {worktree && offer?.offered !== false && (
              <button className="btn h-6 px-2" disabled={running || filesOff !== ''} title={filesOff || undefined} onClick={showFiles}>
                Revert files too
              </button>
            )}
            <button className="btn btn-accent h-6 px-2" autoFocus disabled={running} aria-busy={running || undefined} onClick={() => choose(false)}>
              Revert and keep changes
            </button>
          </div>
        </>
      )}
      {step.at === 'files' && (
        <>
          {preview === null && <p className="text-dim">Checking what Claude's turns changed since this message…</p>}
          {preview !== null && previewOff !== '' && <p className="text-soft">{previewOff}</p>}
          {preview !== null && previewOff === '' && (
            <>
              <p className="text-ink">{changesLabel(preview)}</p>
              <ul className="mono max-h-32 overflow-y-auto text-dim select-text">
                {preview.files.map((file) => (
                  <li key={file} className="truncate" title={file}>
                    {relative(file, worktree)}
                  </li>
                ))}
              </ul>
              <p className="text-dim">Files changed by commands are not reverted. Edits made outside Claude since then are overwritten.</p>
            </>
          )}
          <div className="flex flex-wrap justify-end gap-1.5">
            <button className="btn h-6 px-2" disabled={running} onClick={() => setStep({ at: 'choose' })}>
              Back
            </button>
            <button className="btn btn-danger h-6 px-2" autoFocus disabled={running || previewOff !== ''} aria-busy={running || undefined} onClick={() => choose(true)}>
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
