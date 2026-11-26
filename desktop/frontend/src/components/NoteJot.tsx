import { useRef, useState } from 'react'
import { useOrchestrator } from '../OrchestratorContext'
import { useFocusRequest } from '../useFocusRequest'
import { usePendingImages } from '../usePendingImages'
import { AutoTextarea } from './AutoTextarea'
import { NoteImages } from './NoteImages'

export function NoteJot() {
  const { addNote, report } = useOrchestrator('addNote', 'report')
  const pending = usePendingImages()
  const input = useRef<HTMLTextAreaElement>(null)
  const [text, setText] = useState('')
  const sending = useRef(false)
  const canFile = text.trim() !== '' || pending.images.length > 0

  useFocusRequest('note-jot', input)

  const file = () => {
    if (sending.current || !canFile) return
    sending.current = true
    report(async () => {
      try {
        await addNote(text, pending.images)
        setText('')
        pending.clear()
      } finally {
        sending.current = false
      }
    })
  }

  return (
    <div
      className="flex flex-col gap-2 rounded-[0.625rem] border border-line-strong bg-raised px-2.5 py-1.5 focus-within:border-accent"
      onDragOver={(e) => e.dataTransfer.types.includes('Files') && e.preventDefault()}
      onDrop={(e) => pending.attach(e.dataTransfer) && e.preventDefault()}
    >
      <div className="flex items-start gap-2">
        <AutoTextarea
          taRef={input}
          value={text}
          placeholder="Jot a note…"
          aria-label="New note"
          className="min-h-6 py-0.5 text-body"
          onChange={(e) => setText(e.target.value)}
          onPaste={(e) => pending.attach(e.clipboardData) && e.preventDefault()}
          onKeyDown={(e) => {
            if (e.key === 'Escape') {
              e.stopPropagation()
              e.currentTarget.blur()
              return
            }
            if (e.key !== 'Enter' || e.shiftKey || e.nativeEvent.isComposing) return
            e.preventDefault()
            file()
          }}
        />
        <button className="mono h-6 flex-none text-small text-dim hover:text-ink disabled:hover:text-dim" title="Add the note (Enter)" aria-label="Add note" disabled={!canFile} onClick={file}>
          ⏎
        </button>
      </div>
      {pending.images.length > 0 && <NoteImages urls={pending.images.map((i) => i.preview)} onRemove={pending.remove} />}
    </div>
  )
}
