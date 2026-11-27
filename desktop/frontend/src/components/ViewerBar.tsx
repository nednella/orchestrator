import type { ReactNode } from 'react'

// ViewerBar floats the controls of a full-window image view.
export function ViewerBar({ children }: { children: ReactNode }) {
  return <div className="glass flex flex-wrap items-center justify-center gap-0.5 rounded-2xl border border-(--border-rule) p-1 shadow-(--shadow-pill)">{children}</div>
}
