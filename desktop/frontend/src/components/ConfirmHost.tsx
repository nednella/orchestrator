import { useEffect, useId, useRef } from 'react'
import { useAgentos } from '../AgentosContext'
import { answerConfirm, checkConfirm, useConfirm } from '../confirm'
import { answerFor } from '../confirmQueue'

// keepFocusIn wraps Tab and Shift+Tab around the dialog's controls, so the focus never leaves it.
function keepFocusIn(dialog: HTMLElement | null, e: KeyboardEvent) {
  const controls = [...(dialog?.querySelectorAll<HTMLElement>('button:not(:disabled), input:not(:disabled)') ?? [])]
  if (controls.length === 0) return
  const at = controls.indexOf(document.activeElement as HTMLElement)
  const next = e.shiftKey ? (at <= 0 ? controls.length - 1 : at - 1) : (at === -1 || at === controls.length - 1 ? 0 : at + 1)
  e.preventDefault()
  controls[next].focus()
}

// ConfirmHost shows the confirm askConfirm asks, one at a time: the next waits until this one is answered and gone.
// Esc cancels and the confirm button has the focus; the element that had it before the first gets it back after the last.
export function ConfirmHost() {
  const { focus } = useAgentos('focus')
  const { request, leaving } = useConfirm()
  const dialog = useRef<HTMLDivElement>(null)
  const yes = useRef<HTMLButtonElement>(null)
  const titleId = useId()
  const bodyId = useId()
  const shown = request !== undefined
  const id = request?.id

  useEffect(() => {
    if (!shown) return
    const opener = document.activeElement
    return () => {
      // What the answer did may have put the focus somewhere, as an edit does in the composer.
      const now = document.activeElement
      if (now !== null && now !== document.body) return
      if (opener instanceof HTMLElement && opener.isConnected && opener !== document.body) opener.focus()
      else focus('thread')
    }
  }, [shown, focus])

  useEffect(() => {
    if (id === undefined) return
    yes.current?.focus()
    const onKey = (e: KeyboardEvent) => {
      if (answerFor(e.key) === false) {
        e.preventDefault()
        e.stopPropagation()
        answerConfirm(id, false)
      } else if (e.key === 'Tab') keepFocusIn(dialog.current, e)
    }
    window.addEventListener('keydown', onKey, true)
    return () => window.removeEventListener('keydown', onKey, true)
  }, [id])

  if (!request) return null
  const { title, body = [], list = [], checkbox: box } = request

  return (
    // A press on the backdrop keeps the focus in the dialog; only Cancel or Esc says no.
    <div className="confirm-back" data-leaving={leaving || undefined} onMouseDown={(e) => e.target === e.currentTarget && e.preventDefault()}>
      <div ref={dialog} className="confirm-dialog" role="alertdialog" aria-modal="true" aria-labelledby={titleId} aria-describedby={bodyId}>
        <div className="confirm-head">
          <h2 id={titleId} className="confirm-title">
            {title}
          </h2>
          <div id={bodyId} className="confirm-body empty:hidden">
            {/* A confirm's paragraphs and list never change while it shows. */}
            {body.map((p, i) => (
              <p key={i}>{p}</p>
            ))}
            {list.length > 0 && (
              <ul className="confirm-list">
                {list.map((x, i) => (
                  <li key={i} className="truncate" title={x}>
                    {x}
                  </li>
                ))}
              </ul>
            )}
            {box && (
              <label className="confirm-check" data-disabled={box.disabled ? '' : undefined}>
                <input type="checkbox" checked={box.checked} disabled={!!box.disabled} onChange={(e) => checkConfirm(request.id, e.target.checked)} />
                <span>
                  <span>{box.label}</span>
                  {(box.disabled || box.hint) && <small>{box.disabled || box.hint}</small>}
                </span>
              </label>
            )}
          </div>
        </div>
        <div className="confirm-foot">
          <button className="btn" onClick={() => answerConfirm(request.id, false)}>
            Cancel
          </button>
          <button ref={yes} className={`btn ${request.tone === 'danger' ? 'btn-destructive' : 'btn-primary'}`} onClick={() => answerConfirm(request.id, true)}>
            {request.confirm}
          </button>
        </div>
      </div>
    </div>
  )
}
