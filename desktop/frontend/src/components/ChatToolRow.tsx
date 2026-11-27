import { LoaderCircle } from 'lucide-react'
import { memo, useState } from 'react'
import { isOpened, setOpened } from '../chatOpen'
import { resultLimit, resultLines, toolSummary } from '../chatTools'
import { ChatToolInput } from './ChatToolInput'
import { ToolIcon } from './ToolIcon'

type ChatToolRowProps = { thread: string; id: string; tool: string; input: unknown; result: string | null; error: boolean; denied: boolean; running: boolean; folder: string }

export const ChatToolRow = memo(function ChatToolRow({ thread, id, tool, input, result, error, denied, running, folder }: ChatToolRowProps) {
  const [open, setOpen] = useState(() => isOpened(thread, id))
  const [all, setAll] = useState(false)
  const { icon, title, detail } = toolSummary(tool, input, folder)
  const waiting = running && result === null

  return (
    <div>
      <button className="flex w-full min-w-0 items-center gap-2 py-1 text-left text-small text-soft hover:text-ink" aria-expanded={open} onClick={() => {
          setOpened(thread, id, !open)
          setOpen(!open)
        }}>
        <span className={`flex flex-none ${error ? 'text-danger' : 'text-dim'}`}>
          <ToolIcon name={icon} size={12} />
        </span>
        <span className="flex-none">{title}</span>
        {detail && <span className="mono min-w-0 truncate text-dim">{detail}</span>}
        {denied && <span className="flex-none text-label text-danger">Denied</span>}
        {waiting && (
          <span className="flex flex-none animate-spin text-dim" role="img" aria-label="Running">
            <LoaderCircle size={12} aria-hidden="true" />
          </span>
        )}
      </button>
      {open && <ChatToolDetail tool={tool} input={input} result={result} error={error} all={all} onShowAll={() => setAll(true)} />}
    </div>
  )
})

type ChatToolDetailProps = { tool: string; input: unknown; result: string | null; error: boolean; all: boolean; onShowAll: () => void }

function ChatToolDetail({ tool, input, result, error, all, onShowAll }: ChatToolDetailProps) {
  const out = result === null ? null : resultLines(result, all)
  return (
    <div className="mb-2 ml-5 flex min-w-0 flex-col gap-1.5 select-text">
      <ChatToolInput tool={tool} input={input} />
      {out && out.shown !== '' && (
        <pre className={`chat-tool-pre ${error ? 'text-danger' : ''}`}>{out.shown}</pre>
      )}
      {out && !all && out.total > resultLimit && (
        <button className="btn btn-ghost self-start" onClick={onShowAll}>
          Show all {out.total} lines
        </button>
      )}
    </div>
  )
}
