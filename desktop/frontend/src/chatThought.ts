// thoughtLine is the line a thinking block shows: its leading bold heading, else its first sentence, without markdown.
// While the block streams, the line grows until that sentence ends.
export function thoughtLine(text: string): string {
  const trimmed = text.trim()
  const heading = /^\*\*(.+?)\*\*/s.exec(trimmed)
  if (heading) return plain(heading[1])
  const end = /[.?!](?=\s)|。/.exec(trimmed)
  return plain(end ? trimmed.slice(0, end.index + 1) : trimmed)
}

function plain(markdown: string): string {
  return markdown
    .replace(/!?\[([^\]]*)\]\([^)]*\)/g, '$1')
    .replace(/^#+\s+/, '')
    .replace(/\*+|`+|~~/g, '')
    .replace(/\s+/g, ' ')
    .trim()
}
