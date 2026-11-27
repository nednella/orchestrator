import type { FilePlace } from '../filesView'
import { formatBytes, isMarkdownPath, withoutFrontMatter } from '../fileTree'
import type { FileContent } from '../types'
import { ChatMarkdown } from './ChatMarkdown'
import { FileCodeView } from './FileCodeView'
import { FileImageView } from './FileImageView'
import { Notice } from './Notice'

type FileBodyProps = {
  linksFrom: { key: string; path: string }
  content: FileContent
  rendered: boolean
  wrap: boolean
  place: FilePlace | null
}

// FileBody is the open file as its kind shows it: an image, a note that it is binary, rendered markdown, or its code.
export function FileBody({ linksFrom, content, rendered, wrap, place }: FileBodyProps) {
  const { path } = linksFrom
  if (content.kind === 'image') return <FileImageView key={path} path={path} content={content} />
  if (content.kind === 'binary') return <Notice title="The file is binary and cannot be displayed as text." hint={formatBytes(content.size)} centered />
  if (content.text === '') return <Notice title="This file is empty." hint="" centered />
  if (rendered && isMarkdownPath(path) && !content.truncated) {
    return (
      <div className="min-h-0 flex-1 overflow-auto">
        <div className="mx-auto max-w-4xl px-6 py-5">
          <ChatMarkdown text={withoutFrontMatter(content.text)} file={linksFrom} page />
        </div>
      </div>
    )
  }
  return (
    <>
      {content.truncated && (
        <p className="flex-none border-b border-(--border-warning) bg-(--bg-warning) px-3 py-1.5 text-label text-(--text-warning)">
          Preview limited to the first 1 MB of a {content.size.toLocaleString()} byte file.
        </p>
      )}
      <FileCodeView key={path} path={path} text={content.text} wrap={wrap} place={place} />
    </>
  )
}
