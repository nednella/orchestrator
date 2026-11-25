import { toolBody } from '../chatTools'

type ChatToolInputProps = { tool: string; input: unknown }

export function ChatToolInput({ tool, input }: ChatToolInputProps) {
  const body = toolBody(tool, input)
  return (
    <pre className="chat-tool-pre">
      {'diff' in body ? body.diff.map((l, n) => (
        <span key={n} className={`block ${l.sign === '+' ? 'chat-diff-add' : l.sign === '−' ? 'chat-diff-del' : 'text-dim'}`}>
          {l.sign} {l.text}
        </span>
      )) : body.text}
    </pre>
  )
}
