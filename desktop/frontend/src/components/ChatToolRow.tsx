import { memo, useState } from 'react'
import { resultLines, toolBody, toolSummary } from '../chatTools'
import { Icon } from './Icon'

type ChatToolRowProps = { tool: string; input: unknown; result: string | null; error: boolean; running: boolean; folder: string }

export const ChatToolRow = memo(function ChatToolRow({ tool, input, result, error, running, folder }: ChatToolRowProps) {
  const [open, setOpen] = useState(false)
  const [all, setAll] = useState(false)
  const { icon, title, detail } = toolSummary(tool, input, folder)
  const waiting = running && result === null

  return (
    <div>
      <button className="flex w-full min-w-0 items-center gap-2 py-1 text-left text-small text-soft hover:text-ink" aria-expanded={open} onClick={() => setOpen(!open)}>
        <span className={`flex flex-none ${error ? 'text-danger' : 'text-dim'}`}>
          <Icon name={icon} size={12} />
        </span>
        <span className="flex-none">{title}</span>
        {detail && <span className="mono min-w-0 truncate text-dim">{detail}</span>}
        {waiting && (
          <span className="flex flex-none animate-spin text-dim" role="img" aria-label="Running">
            <Icon name="loader" size={12} />
          </span>
        )}
      </button>
      {open && <ChatToolDetail tool={tool} input={input} result={result} error={error} all={all} onShowAll={() => setAll(true)} />}
    </div>
  )
})

type ChatToolDetailProps = { tool: string; input: unknown; result: string | null; error: boolean; all: boolean; onShowAll: () => void }

function ChatToolDetail({ tool, input, result, error, all, onShowAll }: ChatToolDetailProps) {
  const body = toolBody(tool, input)
  const out = result === null ? null : resultLines(result, all)
  return (
    <div className="mb-2 ml-5 flex min-w-0 flex-col gap-1.5 select-text">
      <pre className="chat-tool-pre">
        {'diff' in body ? body.diff.map((l, n) => (
          <span key={n} className={`block ${l.sign === '+' ? 'chat-diff-add' : 'chat-diff-del'}`}>
            {l.sign} {l.text}
          </span>
        )) : body.text}
      </pre>
      {out && out.shown !== '' && (
        <pre className={`chat-tool-pre ${error ? 'text-danger' : ''}`}>{out.shown}</pre>
      )}
      {out && !all && out.total > 20 && (
        <button className="btn btn-ghost self-start" onClick={onShowAll}>
          Show all {out.total} lines
        </button>
      )}
    </div>
  )
}
