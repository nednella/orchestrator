import { Children, isValidElement, memo, useContext } from 'react'
import type { ReactElement, ReactNode } from 'react'
import Markdown from 'react-markdown'
import type { Components, UrlTransform } from 'react-markdown'
import { useOrchestrator } from '../OrchestratorContext'
import { api } from '../api'
import { codeLanguage, markdownProps, textCache } from '../chatMarkdown'
import { filesKey, imageTarget, linkTarget, pickFile, planPath } from '../filesView'
import { useLayout } from '../LayoutContext'
import { markdownBlocks, steadyBlock } from '../markdownBlocks'
import { ChatCodeBlock } from './ChatCodeBlock'
import { FileHighlight } from './FileHighlight'
import { FileLinks } from './fileLinks'
import { FileMarkdownImage } from './FileMarkdownImage'

// file is the file the text is, when the Files view shows it: its Files view's key, and its path, which its relative
// links start from. Without one, a link opens in the open thread's Files view, from its folder. A page is set in the
// larger type of a document to read, not of a chat reply.
type ChatMarkdownProps = { text: string; streaming?: boolean; file?: { key: string; path: string }; page?: boolean }

const textOf = (node: ReactNode): string =>
  Children.toArray(node)
    .map((child) => (isValidElement(child) ? textOf((child as ReactElement<{ children?: ReactNode }>).props.children) : String(child)))
    .join('')

const components: Components = {
  a: function MarkdownLink({ href, children }) {
    const { report, selectedId, project } = useOrchestrator('report', 'selectedId', 'project')
    const { showPanelView } = useLayout()
    const from = useContext(FileLinks)
    if (!href) return <>{children}</>
    const file = planPath(href)
    return (
      <a
        href={href}
        onClick={(e) => {
          e.preventDefault()
          if (file === null) {
            report(() => api.openURL(href))
            return
          }
          const path = linkTarget(from?.path ?? '', file)
          const thread = selectedId ?? undefined
          if (e.metaKey && thread) {
            report(() => api.openInEditor(thread, path))
            return
          }
          pickFile(from?.key ?? filesKey(thread, project?.key ?? ''), path)
          showPanelView('files')
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

// A link to a markdown file keeps its address, for the Files view to open; any other address goes as the chat's rules say.
const urlTransform: UrlTransform = (url, key, node) => (planPath(url) !== null ? url : (markdownProps.urlTransform?.(url, key, node) ?? ''))

// A page keeps its code coloured and shows the images its own folder holds, by a path that is not a web address.
const pageComponents: Components = {
  ...components,
  pre: ({ children }) => {
    const code = Children.toArray(children)[0]
    const className = isValidElement(code) ? (code as ReactElement<{ className?: string }>).props.className : undefined
    const language = codeLanguage(className)
    return (
      <div className="chat-code">
        {language && <div className="chat-code-bar">{language}</div>}
        <FileHighlight language={language} code={textOf(children).replace(/\n$/, '')} />
      </div>
    )
  },
  img: ({ src, alt }) => <FileMarkdownImage src={src} alt={alt} />,
}

const pageUrlTransform: UrlTransform = (url, key, node) => (key === 'src' && node.tagName === 'img' && imageTarget('', url) !== null ? url : urlTransform(url, key, node))

const parse = (text: string, page = false) =>
  Markdown(page ? { ...markdownProps, disallowedElements: [], unwrapDisallowed: false, urlTransform: pageUrlTransform, components: pageComponents, children: text } : { ...markdownProps, urlTransform, components, children: text })

// A row the virtualised list mounts again finds its blocks here instead of parsing them again.
const parsed = textCache<ReactElement>(500_000)
const parsedPages = textCache<ReactElement>(500_000)

function parseCached(text: string, page: boolean) {
  const cache = page ? parsedPages : parsed
  const hit = cache.get(text)
  if (hit) return hit
  const tree = parse(text, page)
  cache.set(text, tree)
  return tree
}

type MarkdownBlockProps = { text: string; writing: boolean; page: boolean }

// The block still being written changes on every update, so it is neither cached nor kept.
const MarkdownBlock = memo(function MarkdownBlock({ text, writing, page }: MarkdownBlockProps) {
  return writing ? parse(text, page) : parseCached(text, page)
})

// A reply renders block by block: a streaming update parses only its last block, and the finished reply takes its other
// blocks from the cache, with the same markup a parse of the whole text gives.
export const ChatMarkdown = memo(function ChatMarkdown({ text, streaming = false, file, page = false }: ChatMarkdownProps) {
  const blocks = markdownBlocks(text)
  const last = blocks.length - 1
  return (
    <FileLinks.Provider value={file ?? null}>
      <div className={page ? 'markdown markdown-page' : 'markdown text-body'}>
        {blocks.map((block, i) => (streaming && i === last ? <MarkdownBlock key={i} text={steadyBlock(block)} writing page={page} /> : <MarkdownBlock key={i} text={block} writing={false} page={page} />))}
      </div>
    </FileLinks.Provider>
  )
})
