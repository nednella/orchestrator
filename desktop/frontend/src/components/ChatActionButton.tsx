import type { ReactNode } from 'react'

// revealOnHover hides a message's actions until the pointer is over the message or the focus is in it; a screen with
// no hover shows them always.
export const revealOnHover = 'opacity-0 transition-opacity duration-200 group-hover/message:opacity-100 group-focus-within/message:opacity-100 [@media(hover:none)]:opacity-100'

// label names a button that shows only an icon. A muted button's icon and text turn from soft to ink on hover; any
// other changes only its background.
type ChatActionButtonProps = { label?: string; muted?: boolean; disabled?: boolean; onClick(): void; children: ReactNode }

export function ChatActionButton({ label, muted = false, disabled = false, onClick, children }: ChatActionButtonProps) {
  return (
    <button
      type="button"
      title={label}
      aria-label={label}
      disabled={disabled}
      onClick={onClick}
      className={`inline-flex h-7 flex-none items-center justify-center gap-1 rounded-lg border border-transparent px-[7px] text-small text-soft transition-[background-color,color,scale] duration-150 ease-in-out enabled:hover:bg-[var(--bg-ghost-hover)] enabled:active:scale-[0.97] disabled:opacity-64 sm:h-6 ${muted ? 'enabled:hover:text-ink' : ''}`}
    >
      {children}
    </button>
  )
}
