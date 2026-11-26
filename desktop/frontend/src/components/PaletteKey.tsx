import type { ReactNode } from 'react'

type PaletteKeyProps = { children: ReactNode }

export function PaletteKey({ children }: PaletteKeyProps) {
  return <kbd className="inline-flex h-5 min-w-5 items-center justify-center gap-0.5 rounded-sm bg-(--bg-palette-key) px-1 font-sans text-small font-medium text-ink">{children}</kbd>
}
