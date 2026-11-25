import { Children, isValidElement, memo } from 'react'
import type { ReactElement, ReactNode } from 'react'
import Markdown from 'react-markdown'
import type { Components } from 'react-markdown'
import { useAgentos } from '../AgentosContext'
import { api } from '../api'
import { codeLanguage, markdownProps } from '../chatMarkdown'
import { ChatCodeBlock } from './ChatCodeBlock'

type ChatMarkdownProps = { text: string }

const textOf = (node: ReactNode): string =>
  Children.toArray(node)
    .map((child) => (isValidElement(child) ? textOf((child as ReactElement<{ children?: ReactNode }>).props.children) : String(child)))
    .join('')

export const ChatMarkdown = memo(function ChatMarkdown({ text }: ChatMarkdownProps) {
  const { report } = useAgentos('report')

  const components: Components = {
    a: ({ href, children }) =>
      href ? (
        <a
          href={href}
          onClick={(e) => {
            e.preventDefault()
            report(() => api.openURL(href))
          }}
          onAuxClick={(e) => e.preventDefault()}
        >
          {children}
        </a>
      ) : (
        <>{children}</>
      ),
    pre: ({ children }) => {
      const code = Children.toArray(children)[0]
      const className = isValidElement(code) ? (code as ReactElement<{ className?: string }>).props.className : undefined
      return <ChatCodeBlock language={codeLanguage(className)} code={textOf(children).replace(/\n$/, '')} />
    },
  }

  return (
    <div className="markdown text-body">
      <Markdown {...markdownProps} components={components}>
        {text}
      </Markdown>
    </div>
  )
})
