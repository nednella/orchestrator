export type MarkdownKind = 'marker' | 'heading' | 'strong' | 'em' | 'code' | 'link' | 'quote' | 'block'
export type MarkdownSpan = { text: string; kind?: MarkdownKind }

const FENCE = /^ {0,3}(`{3,}|~{3,})/
const HEADING = /^( {0,3}#{1,6})((?:[ \t].*)?)$/
const QUOTE = /^( {0,3}>+ ?)(.*)$/
const LIST = /^([ \t]*)([-*+]|\d{1,9}[.)])([ \t]+.*|)$/

// One alternative per inline form, tried left to right: `code`, [text](url), a bare URL, **strong**, *em*.
const INLINE = new RegExp(
  [
    '`[^`\\n]+`',
    '\\[([^\\]\\n]+)\\]\\(([^)\\s]+)\\)',
    'https?:\\/\\/[^\\s<>()]*[^\\s<>().,;:!?\'"]',
    '(\\*\\*|(?<!\\w)__)(?=\\S)(.+?)(?<=\\S)\\3(?!\\w)',
    '(\\*|(?<!\\w)_)(?=[^\\s*_])(.+?)(?<=[^\\s*_])\\5(?!\\w)',
  ].join('|'),
  'g',
)

function inline(text: string, plain?: MarkdownKind): MarkdownSpan[] {
  const spans: MarkdownSpan[] = []
  let at = 0
  for (const m of text.matchAll(INLINE)) {
    const start = m.index
    if (start > at) spans.push({ text: text.slice(at, start), kind: plain })
    if (m[0].startsWith('`')) spans.push({ text: m[0], kind: 'code' })
    else if (m[1] !== undefined) spans.push({ text: '[', kind: 'marker' }, { text: m[1], kind: 'link' }, { text: `](${m[2]})`, kind: 'marker' })
    else if (m[3] !== undefined) spans.push({ text: m[3], kind: 'marker' }, { text: m[4], kind: 'strong' }, { text: m[3], kind: 'marker' })
    else if (m[5] !== undefined) spans.push({ text: m[5], kind: 'marker' }, { text: m[6], kind: 'em' }, { text: m[5], kind: 'marker' })
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
    const opens = FENCE.exec(source)
    if (fence) {
      const closes = opens && opens[1][0] === fence[0] && opens[1].length >= fence.length
      spans.push({ text: source, kind: closes ? 'marker' : 'block' })
      if (closes) fence = ''
      return
    }
    if (opens) {
      fence = opens[1]
      spans.push({ text: source, kind: 'marker' })
      return
    }
    spans.push(...line(source))
  })
  return spans.filter((span) => span.text !== '')
}
