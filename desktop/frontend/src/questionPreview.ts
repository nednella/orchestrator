// A preview is the markdown Claude gives an option of AskUserQuestion to show it by. It holds, in any order, ```html
// blocks, images `![alt](path)`, and text that is an ASCII mockup or code, or markdown.
export type PreviewPart = { kind: 'html'; html: string } | { kind: 'image'; alt: string; src: string } | { kind: 'code'; text: string } | { kind: 'markdown'; text: string }

const HTML_BLOCK = /```html[^\S\n]*\n([\s\S]*?)(?:\n[^\S\n]*)?```/gi
const IMAGE = /!\[([^\]\n]*)\]\(\s*([^)\s]+)(?:\s+"[^"\n]*")?\s*\)/g
const MARKDOWN = /^```|^#{1,6}\s|\*\*[^*\n]+\*\*|`[^`\n]+`|\[[^\]\n]+\]\([^)\n]+\)/m

const wideLine = 48

// textPart is a stretch of text between images and pages: markdown if it uses markdown, else a code block that keeps
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
export function previewParts(preview: string): PreviewPart[] {
  const parts: PreviewPart[] = []
  let at = 0
  for (const m of preview.matchAll(HTML_BLOCK)) {
    parts.push(...withImages(preview.slice(at, m.index)))
    if (m[1].trim() !== '') parts.push({ kind: 'html', html: m[1] })
    at = m.index + m[0].length
  }
  return [...parts, ...withImages(preview.slice(at))]
}

// isWidePreview reports whether the preview needs a whole row: a page or an image, or a line of text that a narrow
// tile would cut.
export const isWidePreview = (parts: PreviewPart[]): boolean =>
  parts.some((p) => p.kind === 'html' || p.kind === 'image' || ((p.kind === 'code' || p.kind === 'markdown') && p.text.split('\n').some((line) => line.length > wideLine)))

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

// The page a preview's html block is shown in. Its sandbox holds no scripts, forms, popups or links out, its content
// security policy lets no request out, and the theme's tokens are its only part from the app.
export const previewCSP = "default-src 'none'; style-src 'unsafe-inline'; img-src data:; font-src data:"
export const previewSandbox = ''

// previewTokens are the theme tokens a page may use as var(--name).
export const previewTokens = [
  '--bg-app', '--bg-surface', '--bg-raised', '--bg-hover', '--bg-code', '--bg-segment', '--bg-segment-on',
  '--border', '--border-strong',
  '--text', '--text-soft', '--text-dim',
  '--accent', '--text-on-accent', '--waiting', '--finished', '--idle', '--danger',
  '--font-ui', '--font-mono',
]

// previewScheme is the theme's color-scheme, passed as a token too so that the page's own controls and canvas match.
const SCHEME = 'color-scheme'

// previewDocument is the page of an html block: the policy first, then the token values, then the block as it came.
// values holds a value for each token the theme sets; one that cannot sit in a style rule is left out.
export function previewDocument(html: string, values: Record<string, string>): string {
  const safe = (v: string) => !/[<>{}]/.test(v)
  const tokens = [SCHEME, ...previewTokens]
    .filter((name) => values[name] && safe(values[name]))
    .map((name) => `${name}:${values[name]}`)
    .join(';')
  return `<meta http-equiv="Content-Security-Policy" content="${previewCSP}"><style>:root{${tokens}}</style>${html}`
}

// previewFrame is the attributes of the iframe that shows an html block, and so of the one that shows it full size: an
// empty sandbox, which allows nothing, and the page.
export const previewFrame = (html: string, values: Record<string, string>) => ({ sandbox: previewSandbox, srcDoc: previewDocument(html, values), referrerPolicy: 'no-referrer' as const })

// The full-size frame gets no pointer events, so that a click on a link in the page cannot navigate the frame.
export const previewDialogFrameStyle = { pointerEvents: 'none', background: 'var(--bg-app)' } as const

// previewWidth is the width a page is laid out at, and previewHeight the height of its tile before it opens.
export const previewWidth = 800
export const previewHeight = 480
