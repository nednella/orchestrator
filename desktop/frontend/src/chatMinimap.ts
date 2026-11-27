import { firstLine } from './chatTimeline.ts'
import type { TimelineItem } from './chatTimeline'

// A strip is one mark on the chat's minimap: a message of the owner's, or an error. index is its item's place in the
// timeline; text is the message's or the error's, and reply, for a message, the last reply before the next message.
export type MinimapStrip = { key: string; index: number; kind: 'prompt' | 'error'; text: string; reply: string }

// A row is where an item sits in the chat's scrolled content.
export type MinimapRow = { start: number; end: number }

const parsedLength = 1500

// plainText is markdown as the words a reader sees: markup and link targets dropped, an image's alt text kept, and
// line breaks and runs of spaces made one space.
export function plainText(markdown: string): string {
  return markdown
    .slice(0, parsedLength)
    .replace(/^ {0,3}(```|~~~).*$/gm, '')
    .replace(/!\[([^\]]*)\]\([^)]*\)/g, '$1')
    .replace(/\[([^\]]*)\]\([^)]*\)/g, '$1')
    .replace(/<(https?:[^>\s]+)>/g, '$1')
    .replace(/<\/?[a-zA-Z][^>]*>/g, '')
    .replace(/^ {0,3}(#{1,6}\s+|>\s?|[-*+]\s+(\[[ xX]\]\s+)?|\d+[.)]\s+)/gm, '')
    .replace(/^ {0,3}([-*_]\s*){3,}$/gm, '')
    .replace(/(\*\*|__|~~|\*|`)/g, '')
    .replace(/\s+/g, ' ')
    .trim()
}

// minimapStrips gives a strip for each message of the owner's and each error, in the chat's order.
export function minimapStrips(items: TimelineItem[]): MinimapStrip[] {
  const strips: MinimapStrip[] = []
  let prompt: MinimapStrip | null = null
  items.forEach((item, index) => {
    if (item.kind === 'user') {
      prompt = { key: item.key, index, kind: 'prompt', text: item.text, reply: '' }
      strips.push(prompt)
    } else if (item.kind === 'reply' && prompt) {
      prompt.reply = item.text
    } else if (item.kind === 'notice' && item.error) {
      strips.push({ key: item.key, index, kind: 'error', text: item.text, reply: '' })
    }
  })
  return strips
}

// preview is what a strip's preview card shows: a message as plain text over its reply, or an error's first line.
export function preview(strip: MinimapStrip): { title: string; detail: string } {
  if (strip.kind === 'error') return { title: firstLine(strip.text), detail: '' }
  return { title: plainText(strip.text) || 'User message', detail: plainText(strip.reply) }
}

const stripGap = 8
const railMin = 64
// Room above and below the rail for the preview card of the first and last strip.
const railMargin = 236

// railHeight is how tall the column of strips is in a pane of that height: 8 px between strips until that would not
// fit, then squeezed evenly.
export function railHeight(count: number, pane: number): number {
  return Math.min((count - 1) * stripGap, Math.max(railMin, pane - railMargin))
}

// stripTop is where the centre of strip i sits on a rail of that height: by its place in the order, not by where its
// item is in the chat.
export function stripTop(i: number, count: number, height: number): number {
  return count < 2 ? 0 : (i / (count - 1)) * height
}

// activeStrip is the strip nearest the point y px down the rail.
export function activeStrip(y: number, height: number, count: number): number {
  const at = height > 0 ? Math.min(Math.max(y / height, 0), 1) : 0
  return Math.round(at * (count - 1))
}

const scales = [1, 0.667, 0.417]
const restScale = 0.333

// stripScale is how long a strip is drawn, as a share of its full length, for its distance from the active strip; null
// when no strip is active.
export function stripScale(distance: number | null): number {
  return distance === null ? restScale : (scales[distance] ?? restScale)
}

// hitStrip is the part of the gutter beside the messages that takes the pointer: its left edge from the pane's and its
// width. gutter is the space between the pane's left edge and the messages'.
export function hitStrip(gutter: number): { left: number; width: number } {
  if (gutter >= 46) return { left: 32, width: Math.min(40, gutter - 32) }
  return { left: 0, width: Math.max(0, Math.min(8, gutter)) }
}

// A gutter this wide always shows the minimap; a narrower one shows it only under the pointer.
export const persistentGutter = (gutter: number) => gutter >= 48

// A strip's span runs from its item to the item before the next strip, so that a long reply keeps its message's strip
// in view.
function span(strips: MinimapStrip[], rows: MinimapRow[], i: number): MinimapRow | null {
  const first = rows[strips[i].index]
  const last = rows[(strips[i + 1]?.index ?? rows.length) - 1]
  return first && last ? { start: first.start, end: last.end } : null
}

const overlaps = (row: MinimapRow, top: number, bottom: number) => row.start < bottom && row.end > top

// inView says, for each strip, whether any row of its span is inside the view from top to bottom.
export function inView(strips: MinimapStrip[], rows: MinimapRow[], top: number, bottom: number): boolean[] {
  return strips.map((_, i) => {
    const s = span(strips, rows, i)
    return s !== null && overlaps(s, top, bottom)
  })
}

// A jump leaves its item this far below the top of the view; the row above it does not count as in view.
export const jumpOffset = 24
const jumpSlack = 25

// currentStrip is the strip the previous and next buttons count from: the first whose own row is in view, else the one
// whose span is being read; -1 above the first.
export function currentStrip(strips: MinimapStrip[], rows: MinimapRow[], top: number, bottom: number): number {
  const shown = strips.findIndex((strip) => {
    const row = rows[strip.index]
    return row !== undefined && overlaps(row, top + jumpSlack, bottom)
  })
  if (shown >= 0) return shown
  return strips.findLastIndex((strip) => (rows[strip.index]?.start ?? Infinity) <= top)
}

const lineHeight = 16

// wheelPixels is a wheel delta in px: a line counts 16 px and a page the view's height.
export function wheelPixels(delta: number, mode: number, view: number): number {
  if (mode === 1) return delta * lineHeight
  if (mode === 2) return delta * view
  return delta
}

// isNotched says whether a wheel event came from a wheel that turns in steps, which glides, rather than a trackpad,
// which scrolls by its delta at once. A notch reports 120 in the legacy delta.
export function isNotched(mode: number, legacyDelta: number): boolean {
  return mode !== 0 || (legacyDelta !== 0 && legacyDelta % 120 === 0)
}

export const glideMs = 150

export const easeOutCubic = (t: number) => 1 - (1 - t) ** 3
