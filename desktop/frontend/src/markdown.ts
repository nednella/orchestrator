export type MarkdownKind = 'marker' | 'heading' | 'strong' | 'em' | 'code' | 'link' | 'quote' | 'block'
export type MarkdownSpan = { text: string; kind?: MarkdownKind }

const FENCE = /^ {0,3}(`{3,}|~{3,})/
const FENCE_CLOSE = /^ {0,3}(`{3,}|~{3,})[ \t]*$/
const HEADING = /^( {0,3}#{1,6})((?:[ \t].*)?)$/
const QUOTE = /^( {0,3}>+ ?)(.*)$/
const LIST = /^([ \t]*)([-*+]|\d{1,9}[.)])([ \t]+.*|)$/

// Runs on every keystroke: no part may scan past the next delimiter of its own kind, or a long line with
// openers and no closers takes quadratic time.
const MAX_INLINE = 2000

// One alternative per inline form, tried left to right: `code`, [text](url), a bare URL, **strong**, __strong__,
// *em*, _em_.
const INLINE = new RegExp(
  [
    '`[^`\\n]+`',
    '\\[([^\\[\\]\\n]+)\\]\\(([^()\\[\\]\\s]+)\\)',
    'https?:\\/\\/[^\\s<>()]*[^\\s<>().,;:!?\'"]',
    '\\*\\*([^*\\s](?:[^*\\n]*[^*\\s])?)\\*\\*',
    '(?<!\\w)__([^_\\s](?:[^_\\n]*[^_\\s])?)__(?!\\w)',
    '\\*([^*\\s](?:[^*\\n]*[^*\\s])?)\\*',
    '(?<!\\w)_([^_\\s](?:[^_\\n]*[^_\\s])?)_(?!\\w)',
  ].join('|'),
  'g',
)

const delimited = (text: string, width: number, kind: MarkdownKind): MarkdownSpan[] => [
  { text: text.slice(0, width), kind: 'marker' },
  { text: text.slice(width, -width), kind },
  { text: text.slice(-width), kind: 'marker' },
]

function inline(text: string, plain?: MarkdownKind): MarkdownSpan[] {
  if (text.length > MAX_INLINE) return [{ text, kind: plain }]
  const spans: MarkdownSpan[] = []
  let at = 0
  for (const m of text.matchAll(INLINE)) {
    const start = m.index
    if (start > at) spans.push({ text: text.slice(at, start), kind: plain })
    if (m[0].startsWith('`')) spans.push({ text: m[0], kind: 'code' })
    else if (m[1] !== undefined) spans.push({ text: '[', kind: 'marker' }, { text: m[1], kind: 'link' }, { text: `](${m[2]})`, kind: 'marker' })
    else if (m[3] !== undefined || m[4] !== undefined) spans.push(...delimited(m[0], 2, 'strong'))
    else if (m[5] !== undefined || m[6] !== undefined) spans.push(...delimited(m[0], 1, 'em'))
    else spans.push({ text: m[0], kind: 'link' })
    at = start + m[0].length
  }
  if (at < text.length) spans.push({ text: text.slice(at), kind: plain })
  return spans
}

function line(text: string): MarkdownSpan[] {
  const heading = HEADING.exec(text)
  if (heading) return [{ text: heading[1], kind: 'marker' }, { text: heading[2], kind: 'heading' }]
  const quote = QUOTE.exec(text)
  if (quote) return [{ text: quote[1], kind: 'marker' }, ...inline(quote[2], 'quote')]
  const list = LIST.exec(text)
  if (list) return [{ text: list[1] }, { text: list[2], kind: 'marker' }, ...inline(list[3])]
  return inline(text)
}

// Splits markdown into spans whose texts join back to the input, so a mirror of it lines up with the source.
export function markdownSpans(text: string): MarkdownSpan[] {
  const spans: MarkdownSpan[] = []
  let fence = ''
  text.split('\n').forEach((source, i) => {
    if (i > 0) spans.push({ text: '\n' })
    if (fence) {
      const close = FENCE_CLOSE.exec(source)
      const closes = close !== null && close[1][0] === fence[0] && close[1].length >= fence.length
      spans.push({ text: source, kind: closes ? 'marker' : 'block' })
      if (closes) fence = ''
      return
    }
    const opens = FENCE.exec(source)
    if (opens) {
      fence = opens[1]
      spans.push({ text: source, kind: 'marker' })
      return
    }
    spans.push(...line(source))
  })
  return spans.filter((span) => span.text !== '')
}
