type KeycapProps = { children: string }

const MODIFIERS = /^[⌘⌥⇧⌃]/
const PROSE = /^([a-z]+|\/)$/

// Lower-case words and "/" are prose between keys ("W / S or ↑ / ↓"); every other token is a key.
function split(label: string): { text: string; key: boolean }[] {
  return label.split(' ').flatMap((token) => {
    if (PROSE.test(token)) return [{ text: token, key: false }]
    const parts: { text: string; key: boolean }[] = []
    while (MODIFIERS.test(token)) {
      parts.push({ text: token[0], key: true })
      token = token.slice(1)
    }
    if (token) parts.push({ text: token, key: true })
    return parts
  })
}

export function Keycap({ children }: KeycapProps) {
  return (
    <span className="inline-flex items-center gap-1 text-label whitespace-nowrap text-dim">
      {split(children).map((part, i) =>
        part.key ? (
          <kbd key={i} className="inline-flex h-5 min-w-5 items-center justify-center rounded-md bg-(--bg-palette-key) px-1.5 font-sans text-small font-medium text-soft">
            {part.text}
          </kbd>
        ) : (
          <span key={i}>{part.text}</span>
        ),
      )}
    </span>
  )
}
