import remarkGfm from 'remark-gfm'
import type { Options } from 'react-markdown'

const WEB_LINK = /^(https?:|mailto:)/i

export const markdownProps: Options = {
  remarkPlugins: [remarkGfm],
  skipHtml: true,
  urlTransform: (url) => (WEB_LINK.test(url) ? url : ''),
  disallowedElements: ['img'],
  unwrapDisallowed: true,
}

export function codeLanguage(className?: string): string {
  return /(?:^|\s)language-(\S+)/.exec(className ?? '')?.[1] ?? ''
}

// textCache keeps values by the text they were made from, the least recently used going first once the texts come to
// more than limit characters in all. A text over a quarter of the limit is never kept.
export function textCache<V>(limit: number) {
  const entries = new Map<string, V>()
  let size = 0
  return {
    get(text: string): V | undefined {
      const value = entries.get(text)
      if (value === undefined) return undefined
      entries.delete(text)
      entries.set(text, value)
      return value
    },
    set(text: string, value: V) {
      if (text.length > limit / 4 || entries.has(text)) return
      entries.set(text, value)
      size += text.length
      for (const old of entries.keys()) {
        if (size <= limit) break
        entries.delete(old)
        size -= old.length
      }
    },
  }
}

// lastLine is the number of the last line of a block's text, the line a block being written is still writing.
export const lastLine = (text: string) => text.trimEnd().split('\n').length

// settled says a link or code span that ends at line end is whole: the block it is in is not being written, writing 0,
// or it is above the line being written, which closes an open code span and so makes a path of every prefix.
export const settled = (writing: number, end: number | undefined) => writing === 0 || (end !== undefined && end < writing)
