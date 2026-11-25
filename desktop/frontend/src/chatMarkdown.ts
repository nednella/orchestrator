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
