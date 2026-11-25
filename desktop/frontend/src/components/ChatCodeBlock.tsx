import { useCopied } from '../useCopied'

type ChatCodeBlockProps = { language: string; code: string }

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
        <code>{code}</code>
      </pre>
    </div>
  )
}
