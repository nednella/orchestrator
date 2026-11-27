// A preview is the markdown Claude gives an option of AskUserQuestion to show it by. It holds, in any order, images
// `![alt](path)`, and text that is an ASCII mockup or code, or markdown.
export type PreviewPart = { kind: 'image'; alt: string; src: string } | { kind: 'code'; text: string } | { kind: 'markdown'; text: string }

const IMAGE = /!\[([^\]\n]*)\]\(\s*([^)\s]+)(?:\s+"[^"\n]*")?\s*\)/g
const MARKDOWN = /^```|^#{1,6}\s|\*\*[^*\n]+\*\*|`[^`\n]+`|\[[^\]\n]+\]\([^)\n]+\)/m

const wideLine = 48

// textPart is a stretch of text between images: markdown if it uses markdown, else a code block that keeps
// its spaces, as an ASCII mockup needs.
const textPart = (text: string): PreviewPart[] => {
  const trimmed = text.replace(/^\s*\n/, '').trimEnd()
  if (trimmed.trim() === '') return []
  return [MARKDOWN.test(trimmed) ? { kind: 'markdown', text: trimmed } : { kind: 'code', text: trimmed }]
}

function withImages(text: string): PreviewPart[] {
  const parts: PreviewPart[] = []
  let at = 0
  for (const m of text.matchAll(IMAGE)) {
    parts.push(...textPart(text.slice(at, m.index)))
    parts.push({ kind: 'image', alt: m[1], src: m[2] })
    at = m.index + m[0].length
  }
  return [...parts, ...textPart(text.slice(at))]
}

// previewParts splits a preview into what shows it, in order.
export const previewParts = withImages

// isWidePreview reports whether the preview needs a whole row: an image, or a line of text that a narrow
// tile would cut.
export const isWidePreview = (parts: PreviewPart[]): boolean =>
  parts.some((p) => p.kind === 'image' || ((p.kind === 'code' || p.kind === 'markdown') && p.text.split('\n').some((line) => line.length > wideLine)))

// imagePath is the file a preview's image names, or null for an address of the web, a data URL or anything else that
// is not a path: the app reads only files of the thread.
export function imagePath(src: string): string | null {
  let path = src
  if (/^file:\/\//i.test(path)) path = path.slice('file://'.length)
  else if (/^[a-z][a-z0-9+.-]*:/i.test(path)) return null
  try {
    path = decodeURI(path)
  } catch {
    return null
  }
  return path === '' ? null : path
}
