import type { ReactNode } from 'react'

// tip is the short name the tooltip shows when the label is longer.
type DigestIconButtonProps = { label: string; tip?: string; onClick(): void; children: ReactNode }

export function DigestIconButton({ label, tip = label, onClick, children }: DigestIconButtonProps) {
  return (
    <button
      className="group/tip relative inline-flex h-6 w-6 flex-none items-center justify-center rounded-lg text-soft transition-[background-color,color,transform] duration-[120ms] hover:bg-[var(--bg-ghost-hover)] hover:text-ink active:scale-[0.97] [@media(pointer:coarse)]:h-11 [@media(pointer:coarse)]:w-11"
      aria-label={label}
      onClick={onClick}
    >
      {children}
      <span
        className="pointer-events-none absolute bottom-[calc(100%+0.25rem)] left-1/2 -translate-x-1/2 translate-y-0.5 rounded-md bg-[var(--bg-glass)] px-2 py-1 text-small font-normal whitespace-nowrap text-ink opacity-0 shadow-[var(--shadow-tip)] backdrop-blur-[16px] backdrop-saturate-[1.08] transition-[opacity,transform] duration-[120ms] group-hover/tip:translate-y-0 group-hover/tip:opacity-100 group-focus-visible/tip:translate-y-0 group-focus-visible/tip:opacity-100"
        style={{ zIndex: 'var(--z-popup)' }}
        aria-hidden="true"
      >
        {tip}
      </span>
    </button>
  )
}
