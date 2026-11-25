import { useEffect, useRef, useState } from 'react'
import { useAgentos } from '../AgentosContext'

type ChatCodeBlockProps = { language: string; code: string }

export function ChatCodeBlock({ language, code }: ChatCodeBlockProps) {
  const { report } = useAgentos('report')
  const [copied, setCopied] = useState(false)

  const timer = useRef<ReturnType<typeof setTimeout>>()
  useEffect(() => () => clearTimeout(timer.current), [])

  const copy = () => {
    report(async () => {
      await navigator.clipboard.writeText(code)
      setCopied(true)
      clearTimeout(timer.current)
      timer.current = setTimeout(() => setCopied(false), 1500)
    })
  }

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
