type RefProps = { children: string }

export function Ref({ children }: RefProps) {
  return (
    <span className="mono inline-flex h-[1.125rem] max-w-[9rem] items-center truncate rounded-sm bg-hover px-1.5 text-soft">{children}</span>
  )
}
