// A streaming reply is cut into its top-level blocks so that only the block still being written is parsed again on each
// update, and that block is steadied so half-written markdown does not flip between shapes.

const FENCE = /^( {0,3})(`{3,}|~{3,})([^\n]*)$/
const LIST_ITEM = /^ {0,3}(?:[-+*]|\d{1,9}[.)])(?:\s|$)/
const MAY_BE_LIST_ITEM = /^(?:[-+*]|\d{1,9}[.)]?)\r?$/
// A heading, a block quote, a thematic break or a code fence at the margin ends a list; plain text there continues it.
const ENDS_LIST = /^(?:#{1,6}(?:\s|$)|>|(?:[-*_][ \t]*){3,}\r?$|`{3}|~{3})/
// Link reference definitions (also in a quote or a list item), raw HTML and footnotes tie blocks together, so a text
// with any of them is parsed whole.
const TIES = /^(?:\s|>|[-+*]\s|\d{1,9}[.)]\s)*\[[^\]]+\]:|^\s*<[A-Za-z!?/]|\[\^[^\]\s[]+\]/
const MARKERS_ONLY = /^[\s#>*+\-_=~`|:]*(?:\d{1,9}[.)]?[ \t\r]*)?$/
const FENCE_START = /^ {0,3}(?:`{3,}|~{3,})/
const TABLE_ROW = /^ {0,3}\|/
const DELIMITER_CELL = /^\s*:?-+:?\s*$/
const MARKER_RUN = /`+|\*+/y

type Fence = { close: RegExp; indent: number; inList: boolean }
type Scan = { fence: Fence | null; list: boolean }

const isBlank = (line: string) => line.trim() === ''
const indentOf = (line: string) => /^\s*/.exec(line)![0].length

// leaveFence ends a fence inside a list item at a line that leaves the item.
function leaveFence(s: Scan, line: string) {
  if (s.fence?.inList && !isBlank(line) && indentOf(line) < s.fence.indent) s.fence = null
}

// scan moves s past one line.
function scan(s: Scan, line: string) {
  leaveFence(s, line)
  if (s.fence) {
    if (s.fence.close.test(line)) s.fence = null
    return
  }
  const ends = ENDS_LIST.test(line)
  if (ends) s.list = false
  const open = FENCE.exec(line)
  if (open && !(open[2][0] === '`' && open[3].includes('`'))) {
    const [, indent, mark] = open
    // A closing fence is indented at most 3 spaces past its container's margin: the page's, or for a fence in a list
    // item, about the opening fence's own.
    const margin = s.list ? indent.length : 0
    s.fence = { close: new RegExp(`^ {0,${margin + 3}}${mark[0]}{${mark.length},}\\s*$`), indent: indent.length, inList: s.list }
    return
  }
  if (!ends && LIST_ITEM.test(line)) s.list = true
}

// startsBlock says whether a line after a blank line starts a new top-level block: it starts at the margin, and is not
// (and, still being written, may not become) another item of a list the block holds.
function startsBlock(line: string, list: boolean, writing: boolean) {
  if (/^\s/.test(line)) return false
  if (!list) return true
  return !LIST_ITEM.test(line) && !(writing && MAY_BE_LIST_ITEM.test(line))
}

// markdownBlocks cuts text into top-level blocks at blank lines that close every open block: never inside a fenced
// code block, a list or an indented continuation. The last block is the one still being written.
export function markdownBlocks(text: string): string[] {
  const lines = text.split('\n')
  const blocks: string[] = []
  const s: Scan = { fence: null, list: false }
  let start = 0
  let end = -1 // the block's last line that is not blank
  for (const [i, line] of lines.entries()) {
    leaveFence(s, line)
    if (!s.fence) {
      if (isBlank(line)) continue
      if (TIES.test(line)) return [text]
      if (end >= start && end < i - 1 && startsBlock(line, s.list, i === lines.length - 1)) {
        blocks.push(lines.slice(start, end + 1).join('\n'))
        start = i
        s.list = false
      }
    }
    if (!isBlank(line)) end = i
    scan(s, line)
  }
  blocks.push(lines.slice(start).join('\n'))
  return blocks
}

const cellsOf = (row: string) => row.trim().replace(/^\|/, '').replace(/\|$/, '').split('|')

// withoutUnfinishedTable drops a table's rows until its delimiter row has as many cells as its header, since until then
// they read as a paragraph.
function withoutUnfinishedTable(lines: string[]): string[] {
  let first = lines.length
  while (first > 0 && TABLE_ROW.test(lines[first - 1])) first--
  if (first === lines.length) return lines
  const [header, delimiter] = lines.slice(first)
  const cells = delimiter === undefined ? [] : cellsOf(delimiter)
  if (cells.length === cellsOf(header).length && cells.every((c) => DELIMITER_CELL.test(c))) return lines
  return lines.slice(0, first)
}

// closeInline closes the inline code, bold and italic a line being written has opened, and drops a marker that has no
// text after it yet, so the line keeps the shape it will have.
function closeInline(line: string): string {
  let text = line.replace(/(^|\s)\*+$/, '$1')
  const open: { at: number; n: number }[] = []
  for (let i = 0; i < text.length; ) {
    const c = text[i]
    if (c === '\\') {
      i += 2
      continue
    }
    MARKER_RUN.lastIndex = i
    const run = MARKER_RUN.exec(text)?.[0]
    if (!run) {
      i++
      continue
    }
    if (c === '`') {
      const close = closingRun(text, i + run.length, run.length)
      if (close < 0) {
        text = isBlank(text.slice(i + run.length)) ? text.slice(0, i) : text + run
        break
      }
      i = close + run.length
      continue
    }
    let n = run.length
    if (i > 0 && !/\s/.test(text[i - 1])) {
      while (open.length > 0 && open.at(-1)!.n <= n) n -= open.pop()!.n
    }
    if (n > 0 && n <= 3 && (i === 0 || /[\s([{"']/.test(text[i - 1])) && i + run.length < text.length && !/\s/.test(text[i + run.length])) {
      const at = i + run.length - n
      if (n === 3) open.push({ at, n: 1 }, { at: at + 1, n: 2 })
      else open.push({ at, n })
    }
    i += run.length
  }
  let kept = text.length
  let last = lastContent(text, kept)
  let left = open.length
  while (left > 0 && last < open[left - 1].at + open[left - 1].n) {
    kept = open[left - 1].at
    last = lastContent(text, kept)
    left--
  }
  if (left === 0) return text.slice(0, kept)
  return text.slice(0, last + 1) + open.slice(0, left).reverse().map((o) => '*'.repeat(o.n)).join('')
}

// lastContent is the index of the last character before end that is not whitespace, or -1.
function lastContent(text: string, end: number) {
  let at = end - 1
  while (at >= 0 && /\s/.test(text[at])) at--
  return at
}

function closingRun(text: string, from: number, length: number) {
  const runs = /`+/g
  runs.lastIndex = from
  for (let m = runs.exec(text); m; m = runs.exec(text)) if (m[0].length === length) return m.index
  return -1
}

// steadyBlock is the block still being written as it should show: inside an open code fence only whole lines, and
// elsewhere a last line that is only block markers held back, a table held back until its delimiter row is whole, and
// the last line's inline markers closed.
export function steadyBlock(block: string): string {
  const lines = block.split('\n')
  const writing = lines.pop()!
  const s: Scan = { fence: null, list: false }
  lines.forEach((line) => scan(s, line))
  if (s.fence) return [...lines, ''].join('\n')
  const last = MARKERS_ONLY.test(writing) || FENCE_START.test(writing) ? '' : writing
  const shown = withoutUnfinishedTable(last ? [...lines, last] : lines)
  if (shown.length <= lines.length) return [...shown, ''].join('\n')
  return [...lines, closeInline(last)].join('\n')
}
