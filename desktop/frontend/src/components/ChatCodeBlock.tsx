import { useCopied } from '../useCopied'

type ChatCodeBlockProps = { language: string; code: string }

// Each line is its own text node, so a line a streaming reply adds is appended without touching the lines before it.
export function ChatCodeBlock({ language, code }: ChatCodeBlockProps) {
  const [copied, copy] = useCopied(code)

  return (
    <div className="chat-code">
      <div className="chat-code-bar">
        <span>{language}</span>
        <button className="btn btn-ghost" onClick={copy}>
          {copied ? 'Copied' : 'Copy'}
        </button>
      </div>
      <pre>
        <code>{code.split('\n').map((line, i, lines) => (i < lines.length - 1 ? `${line}\n` : line))}</code>
      </pre>
    </div>
  )
}
