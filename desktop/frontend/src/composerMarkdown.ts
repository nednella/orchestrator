import { MarkdownManager } from '@tiptap/markdown'
import { StarterKit } from '@tiptap/starter-kit'
import { getSchema } from '@tiptap/react'
import type { JSONContent } from '@tiptap/react'

export const composerSchema = StarterKit.configure({
  underline: false,
  gapcursor: false,
  hardBreak: { keepMarks: false },
  link: { openOnClick: false, markdownLinks: true },
})

const markdown = new MarkdownManager({ extensions: [composerSchema] })

// The library escapes every markdown character in plain text and encodes `<`, `>` and `&`, so `snake_case` would
// reach the agent as `snake\_case`. Text is written as typed instead, escaped only where it could read as markup.
// 'least' escapes what reads as inline markup wherever it stands; 'safe' also a line's block marker and `](`;
// 'all' every markdown character.
type Escaping = 'least' | 'safe' | 'all'
type TextEncoder = { encodeTextForMarkdown(text: string, node: JSONContent, parent?: JSONContent): string }
const encoder = markdown as unknown as TextEncoder
let escaping: Escaping = 'all'
encoder.encodeTextForMarkdown = (text, node, parent) => {
  if (parent?.type === 'codeBlock' || node.marks?.some((m) => m.type === 'code')) return text
  const siblings = parent?.content ?? []
  return escapeWhereNeeded(text, siblings[0] === node, siblings.at(-1) === node, parent?.type === 'paragraph')
}

const URL_START = /https?:\/\/[^\s<>]+/y
const ENTITY = /&#?[a-z0-9]+;/iy
const BLOCK_START = /^[ \t]{0,3}(?:>|#{1,6}(?=[ \t]|$)|[-+*](?=[ \t]|$)|\d{1,9}[.)](?=[ \t]|$)|(?:=+|-+|(?:[-*_][ \t]*){3,})[ \t]*$)/
const isSpace = (c: string) => /\s/.test(c)
const isPunctuation = (c: string) => /[!-/:-@[-`{-~]/.test(c)

// Whether a run of * or _ could open or close emphasis, by CommonMark's flanking rules. A side next to another
// node is unknown, so it is assumed to.
function mayDelimit(char: string, before: string | undefined, after: string | undefined) {
  if (before === undefined || after === undefined) return true
  const left = !isSpace(after) && (!isPunctuation(after) || isSpace(before) || isPunctuation(before))
  const right = !isSpace(before) && (!isPunctuation(before) || isSpace(after) || isPunctuation(after))
  if (char === '*') return left || right
  return (left && (!right || isPunctuation(before))) || (right && (!left || isPunctuation(after)))
}

// Where a line of a paragraph would start a heading, quote, list or rule, the offset of the character to escape.
function blockMarkers(text: string, atStart: boolean) {
  const at = new Set<number>()
  let offset = 0
  text.split('\n').forEach((line, i) => {
    const marker = (i > 0 || atStart) && BLOCK_START.exec(line)
    if (marker) at.add(offset + marker[0].search(/[^ \t\d]/))
    offset += line.length + 1
  })
  return at
}

function escapeWhereNeeded(text: string, atStart: boolean, atEnd: boolean, inParagraph: boolean) {
  const charAt = (i: number) => (i < 0 ? (atStart ? '\n' : undefined) : i >= text.length ? (atEnd ? '\n' : undefined) : text[i])
  const markers = escaping !== 'least' && inParagraph ? blockMarkers(text, atStart) : new Set<number>()
  let out = ''
  for (let i = 0; i < text.length; ) {
    const c = text[i]
    URL_START.lastIndex = i
    const url = c === 'h' && (i === 0 || isSpace(text[i - 1]) || text[i - 1] === '(') ? URL_START.exec(text) : null
    if (url) {
      out += url[0]
      i += url[0].length
      continue
    }
    if (markers.has(i) || (escaping === 'all' && '\\`*_[]~'.includes(c))) {
      out += `\\${c}`
      i++
      continue
    }
    if (c === '*' || c === '_') {
      let end = i
      while (text[end] === c) end++
      const run = text.slice(i, end)
      out += mayDelimit(c, charAt(i - 1), charAt(end)) ? run.replace(/./g, '\\$&') : run
      i = end
      continue
    }
    const next = charAt(i + 1)
    ENTITY.lastIndex = i
    if (c === '`') out += '\\`'
    else if (c === '\\') out += next === undefined || next === '\n' || isPunctuation(next) ? '\\\\' : c
    else if (c === '~') out += next === '/' && text.indexOf('~') === text.lastIndexOf('~') ? c : '\\~'
    else if (c === ']' && escaping !== 'least') out += next === undefined || '([:'.includes(next) ? '\\]' : c
    else if (c === '&') out += ENTITY.test(text) ? '\\&' : c
    else out += c
    i++
  }
  return out
}

const sameMarks = (a: JSONContent, b: JSONContent) => JSON.stringify(a.marks ?? []) === JSON.stringify(b.marks ?? [])

// A link whose text is its own address is written as the bare address.
function unlinkBareUrl(node: JSONContent): JSONContent {
  const link = node.marks?.find((m) => m.type === 'link')
  const href = link?.attrs?.href
  if (!link || (href !== node.text && href !== `http://${node.text}` && href !== `mailto:${node.text}`)) return node
  return { ...node, marks: node.marks?.filter((m) => m !== link) }
}

const isEmptyParagraph = (node?: JSONContent) => node?.type === 'paragraph' && !node.content?.length
const isEmptyList = (node: JSONContent): boolean =>
  (node.type === 'bulletList' || node.type === 'orderedList' || node.type === 'listItem') && (node.content ?? []).every((child) => isEmptyParagraph(child) || isEmptyList(child))

// Markdown drops the spaces that start or end a paragraph and those that end a line, so they are not written.
function trimParagraph(content: JSONContent[]) {
  return content
    .map((node, i) => {
      if (node.type !== 'text') return node
      let text = node.text?.replace(/[ \t]+\n/g, '\n') ?? ''
      if (i === 0) text = text.replace(/^[ \t]+/, '')
      if (i === content.length - 1) text = text.replace(/[ \t]+$/, '')
      return { ...node, text }
    })
    .filter((node) => node.type !== 'text' || node.text)
}

// Hard breaks become newlines in the text, so a new line reaches the agent as one newline, as typed. Blank
// paragraphs in a row become one and blank list items go, since the library writes them as `&nbsp;`.
function prepare(node: JSONContent): JSONContent {
  if (!node.content) return node
  const content: JSONContent[] = []
  for (const child of node.content) {
    if (isEmptyList(child)) continue
    const next = child.type === 'hardBreak' ? { ...child, type: 'text', text: '\n' } : unlinkBareUrl(prepare(child))
    const last = content.at(-1)
    if (next.type === 'text' && last?.type === 'text' && sameMarks(last, next)) content[content.length - 1] = { ...last, text: `${last.text}${next.text}` }
    else if (!(isEmptyParagraph(next) && isEmptyParagraph(last))) content.push(next)
  }
  return { ...node, content: node.type === 'paragraph' ? trimParagraph(content) : content }
}

function serialize(doc: JSONContent, level: Escaping) {
  escaping = level
  try {
    return markdown.serialize(prepare(doc)).replace(/\n+$/, '')
  } finally {
    escaping = 'all'
  }
}

// Writes the text with the fewest escapes that still read back as the same document.
export function toMarkdown(doc: JSONContent): string {
  const exact = serialize(doc, 'all')
  for (const level of ['least', 'safe'] as const) {
    const text = serialize(doc, level)
    if (serialize(fromMarkdown(text), 'all') === exact) return text
  }
  return exact
}

function splitLines(node: JSONContent): JSONContent[] {
  if (node.type === 'text' && node.text?.includes('\n')) {
    const lineBreak = node.marks ? { type: 'hardBreak', marks: node.marks } : { type: 'hardBreak' }
    return node.text.split('\n').flatMap((text, i) => [...(i > 0 ? [lineBreak] : []), ...(text ? [{ ...node, text }] : [])])
  }
  if (!node.content || node.type === 'codeBlock') return [node]
  return [{ ...node, content: node.content.flatMap(splitLines) }]
}

const schema = getSchema([composerSchema])

type Token = { type: string; raw: string; task?: boolean; tokens?: Token[]; items?: Token[] }

// Tables, images and task lists have no node in the composer; they would load empty or half.
const unsupported = (token: Token): boolean =>
  token.type === 'table' || token.type === 'image' || Boolean(token.task) || [...(token.tokens ?? []), ...(token.items ?? [])].some(unsupported)

const literal = (text: string): JSONContent => (text ? { type: 'paragraph', content: [{ type: 'text', text }] } : { type: 'paragraph' })
const paragraphs = (text: string) => text.replace(/^\n+|\n+$/g, '').split(/\n(?:[ \t]*\n)+/).map(literal)

// Markdown the composer cannot show loads as its source text, so no draft loses text.
function parseBlocks(text: string): JSONContent[] {
  const blocks: JSONContent[] = []
  let run = ''
  const flush = () => {
    const source = run.replace(/^\n+|\n+$/g, '')
    if (source) blocks.push(...(markdown.parse(source).content ?? []))
    run = ''
  }
  for (const token of markdown.instance.lexer(text) as Token[]) {
    if (!unsupported(token)) {
      run += token.raw
      continue
    }
    flush()
    blocks.push(...paragraphs(token.raw))
  }
  flush()
  return blocks
}

function isValid(doc: JSONContent) {
  try {
    schema.nodeFromJSON(doc).check()
    return true
  } catch {
    return false
  }
}

export function fromMarkdown(text: string): JSONContent {
  const content = parseBlocks(text)
  if (content.length === 0) return { type: 'doc', content: [{ type: 'paragraph' }] }
  const doc = splitLines({ type: 'doc', content })[0]
  return isValid(doc) ? doc : splitLines({ type: 'doc', content: paragraphs(text) })[0]
}

// Pasted text goes in as it is, not read as markdown: each line stays a line and a blank line starts a paragraph.
export function fromPlainText(text: string): JSONContent {
  return splitLines({ type: 'doc', content: paragraphs(text.replace(/\r\n?/g, '\n')) })[0]
}
