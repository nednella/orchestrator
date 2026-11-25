import type { ReactNode } from 'react'

const SIZE = { sm: 'h-7 w-7', md: 'h-8 w-8', lg: 'mb-2 h-10 w-10' }

type DigestTileProps = { size: keyof typeof SIZE; children: ReactNode }

export function DigestTile({ size, children }: DigestTileProps) {
  return <span className={`grid flex-none place-items-center rounded-lg bg-[var(--bg-tile)] text-soft shadow-[inset_0_0_0_1px_var(--border)] ${SIZE[size]}`}>{children}</span>
}
