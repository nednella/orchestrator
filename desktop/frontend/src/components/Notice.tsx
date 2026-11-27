import type { ReactNode } from 'react'

type NoticeProps = { title: string; hint: ReactNode; centered?: boolean }

export function Notice({ title, hint, centered = false }: NoticeProps) {
  return (
    <div className={`mx-auto flex w-full max-w-sm flex-col gap-1 px-4 py-8 text-center text-balance ${centered ? 'flex-1 justify-center' : ''}`}>
      <p className="text-body font-medium">{title}</p>
      {hint !== '' && <p className="text-small text-soft">{hint}</p>}
    </div>
  )
}
