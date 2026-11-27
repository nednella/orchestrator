import type { WorkStep } from '../chatTimeline'
import { ChatMarkdown } from './ChatMarkdown'
import { ChatThoughtRow } from './ChatThoughtRow'
import { ChatToolRow } from './ChatToolRow'

type ChatStepsProps = { thread: string; steps: WorkStep[]; running: boolean; folder: string }

// ChatSteps is the lines of a fold or a subagent: its tool calls, its thinking and the text it wrote between them.
export function ChatSteps({ thread, steps, running, folder }: ChatStepsProps) {
  return (
    <>
      {steps.map((s) => {
        if (s.kind === 'note')
          return (
            <div key={s.key} className="py-1 text-small text-soft">
              <ChatMarkdown text={s.text} thread={thread} />
            </div>
          )
        if (s.kind === 'thought') return <ChatThoughtRow key={s.key} text={s.text} live={s.live} />
        return <ChatToolRow key={s.call.id} thread={thread} id={s.call.id} tool={s.call.tool} input={s.call.input} result={s.call.result} error={s.call.error} denied={s.call.answer === 'deny'} running={running} folder={folder} />
      })}
    </>
  )
}
