import { Children, isValidElement, memo } from 'react'
import type { ReactElement, ReactNode } from 'react'
import Markdown from 'react-markdown'
import type { Components } from 'react-markdown'
import { useAgentos } from '../AgentosContext'
import { api } from '../api'
import { codeLanguage, markdownProps, textCache } from '../chatMarkdown'
import { markdownBlocks } from '../markdownBlocks'
import { ChatCodeBlock } from './ChatCodeBlock'

type ChatMarkdownProps = { text: string; streaming?: boolean }

const textOf = (node: ReactNode): string =>
  Children.toArray(node)
    .map((child) => (isValidElement(child) ? textOf((child as ReactElement<{ children?: ReactNode }>).props.children) : String(child)))
    .join('')

const components: Components = {
  a: function MarkdownLink({ href, children }) {
    const { report } = useAgentos('report')
    if (!href) return <>{children}</>
    return (
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
    )
  },
  pre: ({ children }) => {
    const code = Children.toArray(children)[0]
    const className = isValidElement(code) ? (code as ReactElement<{ className?: string }>).props.className : undefined
    return <ChatCodeBlock language={codeLanguage(className)} code={textOf(children).replace(/\n$/, '')} />
  },
}

const parse = (text: string) => Markdown({ ...markdownProps, components, children: text })

// A row the virtualised list mounts again finds its blocks here instead of parsing them again.
const parsed = textCache<ReactElement>(500_000)

function parseCached(text: string) {
  const hit = parsed.get(text)
  if (hit) return hit
  const tree = parse(text)
  parsed.set(text, tree)
  return tree
}

type MarkdownBlockProps = { text: string; writing: boolean }

// The block still being written changes on every update, so it is neither cached nor kept.
const MarkdownBlock = memo(function MarkdownBlock({ text, writing }: MarkdownBlockProps) {
  return writing ? parse(text) : parseCached(text)
})

// A streaming reply renders block by block, so an update parses only its last block; once it ends it renders whole.
export const ChatMarkdown = memo(function ChatMarkdown({ text, streaming = false }: ChatMarkdownProps) {
  const blocks = streaming ? markdownBlocks(text) : [text]
  const last = blocks.length - 1
  return (
    <div className="markdown text-body">
      {blocks.map((block, i) => (streaming && i === last ? <MarkdownBlock key={i} text={block} writing /> : <MarkdownBlock key={i} text={block} writing={false} />))}
    </div>
  )
})
