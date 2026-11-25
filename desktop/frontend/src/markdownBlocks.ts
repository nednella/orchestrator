// A streaming reply is cut into its top-level blocks so that only the block still being written is parsed again on each
// update.

const FENCE = /^( {0,3})(`{3,}|~{3,})(.*)$/
const LIST_ITEM = /^ {0,3}(?:[-+*]|\d{1,9}[.)])(?:[ \t]|$)/
const MAY_BE_LIST_ITEM = /^(?:[-+*]|\d{1,9}[.)]?)$/
// Link reference definitions, raw HTML and footnotes tie blocks together, so a text with any of them is parsed whole.
const TIES = /^\s*(?:\[[^\]]+\]:|<[A-Za-z!?/])|\[\^[^\]\s]+\]/

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
  const open = FENCE.exec(line)
  if (open && !(open[2][0] === '`' && open[3].includes('`'))) {
    const [, indent, mark] = open
    s.fence = { close: new RegExp(`^\\s{0,${indent.length + 3}}${mark[0]}{${mark.length},}\\s*$`), indent: indent.length, inList: s.list }
    return
  }
  if (LIST_ITEM.test(line)) s.list = true
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
