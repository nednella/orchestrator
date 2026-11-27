type RefProps = { children: string }

export function Ref({ children }: RefProps) {
  return (
    <span className="mono inline-flex h-5 max-w-[9rem] items-center truncate rounded-md border border-(--border-rule) bg-(--bg-card) px-1.5 text-soft">{children}</span>
  )
}
