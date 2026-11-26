import type { ReactNode } from 'react'

type SettingsSectionProps = { title: string; children: ReactNode }

export function SettingsSection({ title, children }: SettingsSectionProps) {
  return (
    <section className="space-y-2.5">
      <h2 className="flex min-h-7 items-center px-4 text-title font-semibold">{title}</h2>
      <div className="rounded-xl border border-(--border-rule) bg-(--bg-card)">{children}</div>
    </section>
  )
}
