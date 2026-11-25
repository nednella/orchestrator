type ChatShimmerProps = { text: string }

// ChatShimmer is text with a bright band that sweeps over it, for what Claude is doing now. A copy of the text in the
// full text colour sits over it and shows only through the band, so the text itself never moves.
export function ChatShimmer({ text }: ChatShimmerProps) {
  return (
    <span className="chat-shimmer">
      <span className="line-clamp-4">{text}</span>
      <span className="chat-shimmer-band" aria-hidden>
        <span className="chat-shimmer-copy line-clamp-4">{text}</span>
      </span>
    </span>
  )
}
