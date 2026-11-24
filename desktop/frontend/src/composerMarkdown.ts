import { MarkdownManager } from '@tiptap/markdown'
import { StarterKit } from '@tiptap/starter-kit'
import { Mark, getSchema } from '@tiptap/react'
import type { JSONContent } from '@tiptap/react'

// Text that goes to the agent exactly as it is, such as pasted text. It looks like any other text, and text typed
// right after it is not part of it.
const Literal = Mark.create({
  name: 'literal',
  inclusive: false,
  parseHTML: () => [{ tag: 'span[data-literal]' }],
  renderHTML: () => ['span', { 'data-literal': '' }, 0],
})

export const composerSchema = [
  StarterKit.configure({
    underline: false,
    gapcursor: false,
    hardBreak: { keepMarks: false },
    link: { openOnClick: false, markdownLinks: true },
  }),
  Literal,
]

const markdown = new MarkdownManager({ extensions: composerSchema })

const isLiteral = (node?: JSONContent) => Boolean(node?.marks?.some((m) => m.type === 'literal'))

// The library escapes every markdown character in plain text and encodes `<`, `>` and `&`, so `snake_case` would
// reach the agent as `snake\_case`. Text is written as typed instead, escaped only where it could read as markup.
// 'least' escapes what reads as inline markup wherever it stands; 'safe' also a line's block marker and `](`;
// 'all' every markdown character.
type Escaping = 'least' | 'safe' | 'all'
type TextEncoder = { encodeTextForMarkdown(text: string, node: JSONContent, parent?: JSONContent): string }
const encoder = markdown as unknown as TextEncoder
let escaping: Escaping = 'all'
encoder.encodeTextForMarkdown = (text, node, parent) => {
  if (parent?.type === 'codeBlock' || isLiteral(node) || node.marks?.some((m) => m.type === 'code')) return text
  const siblings = parent?.content ?? []
  const at = siblings.indexOf(node)
  const atLineStart = at <= 0 || Boolean(siblings[at - 1].text?.endsWith('\n'))
  return escapeWhereNeeded(text, atLineStart, at === siblings.length - 1, parent?.type === 'paragraph')
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
      if (node.type !== 'text' || isLiteral(node)) return node
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

// Literal text is written as it is, so only typed text is checked: literal text stands in as characters with no
// meaning in markdown, its line breaks kept.
function standIn(node: JSONContent): JSONContent {
  if (node.type === 'text' && isLiteral(node)) return { ...node, text: node.text?.replace(/[^\n]/g, '\uE000') }
  if (!node.content) return node
  const content = node.content.map(standIn)
  return content.every((child, i) => child === node.content?.[i]) ? node : { ...node, content }
}

// Escaping changes which characters are markup, not where lines break, so the check ignores line layout.
const sameText = (a: string, b: string) => a.replace(/\s+/g, ' ') === b.replace(/\s+/g, ' ')

// Writes the text with the fewest escapes that still read back as the same document.
export function toMarkdown(doc: JSONContent): string {
  const typed = standIn(doc)
  const exact = serialize(typed, 'all')
  for (const level of ['least', 'safe'] as const) {
    const text = serialize(typed, level)
    if (sameText(serialize(fromMarkdown(text), 'all'), exact)) return typed === doc ? text : serialize(doc, level)
  }
  return typed === doc ? exact : serialize(doc, 'all')
}

function splitLines(node: JSONContent): JSONContent[] {
  if (node.type === 'text' && node.text?.includes('\n')) {
    const lineBreak = node.marks ? { type: 'hardBreak', marks: node.marks } : { type: 'hardBreak' }
    return node.text.split('\n').flatMap((text, i) => [...(i > 0 ? [lineBreak] : []), ...(text ? [{ ...node, text }] : [])])
  }
  if (!node.content || node.type === 'codeBlock') return [node]
  return [{ ...node, content: node.content.flatMap(splitLines) }]
}

const schema = getSchema(composerSchema)

type Token = { type: string; raw: string; task?: boolean; tokens?: Token[]; items?: Token[] }

// Tables, images and task lists have no node in the composer; they would load empty or half.
const unsupported = (token: Token): boolean =>
  token.type === 'table' || token.type === 'image' || Boolean(token.task) || [...(token.tokens ?? []), ...(token.items ?? [])].some(unsupported)

const literalParagraph = (text: string): JSONContent => (text ? { type: 'paragraph', content: [{ type: 'text', text, marks: [{ type: 'literal' }] }] } : { type: 'paragraph' })
const literalDoc = (text: string) => splitLines({ type: 'doc', content: [literalParagraph(text)] })[0]

// Markdown the composer cannot show loads as its source text, kept literal, so it is sent as it came in.
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
    blocks.push(literalParagraph(token.raw.replace(/^\n+|\n+$/g, '')))
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
  return isValid(doc) ? doc : literalDoc(text)
}

// Pasted text goes in as it is, not read as markdown, and is sent as it is.
export function fromPlainText(text: string): JSONContent {
  return literalDoc(text.replace(/\r\n?/g, '\n'))
}

// A draft is saved as the editor's document, so it reloads as it was; its markdown is what is sent. A draft saved
// before is a markdown string.
export type ComposerDraft = { doc: JSONContent; text: string }
type SavedDraft = { version: 1; doc: JSONContent }

export const draftOf = (doc: JSONContent): ComposerDraft => ({ doc, text: toMarkdown(doc) })
export const EMPTY_DRAFT = draftOf(fromMarkdown(''))
export const saveDraft = (draft: ComposerDraft): SavedDraft => ({ version: 1, doc: draft.doc })

// The text of a saved document, a line per block.
function textOf(node: JSONContent): string {
  if (typeof node.text === 'string') return node.text
  if (node.type === 'hardBreak') return '\n'
  const children = Array.isArray(node.content) ? node.content : []
  const inline = children.some((child) => typeof child.text === 'string' || child.type === 'hardBreak')
  return children.map(textOf).join(inline ? '' : '\n')
}

export function readDraft(saved: unknown): ComposerDraft {
  if (typeof saved === 'string') return draftOf(fromMarkdown(saved))
  const { version, doc } = (saved ?? {}) as Partial<SavedDraft>
  if (typeof doc !== 'object' || doc === null) return EMPTY_DRAFT
  if (version === 1 && isValid(doc)) return draftOf(doc)
  // A document the schema no longer fits, after a rename say, keeps its text as it is.
  const text = textOf(doc).replace(/^\n+|\n+$/g, '')
  return text.trim() ? draftOf(literalDoc(text)) : EMPTY_DRAFT
}
