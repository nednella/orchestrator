import type { ReactNode } from 'react'

// A plain section has no card and no visible title: its children lay themselves out.
type SettingsSectionProps = { title: string; plain?: boolean; children: ReactNode }

export function SettingsSection({ title, plain = false, children }: SettingsSectionProps) {
  if (plain) {
    return (
      <section>
        <h2 className="sr-only">{title}</h2>
        {children}
      </section>
    )
  }
  return (
    <section className="space-y-2.5">
      <h2 className="flex min-h-7 items-center px-4 text-title font-semibold">{title}</h2>
      <div className="rounded-xl border border-(--border-rule) bg-(--bg-card)">{children}</div>
    </section>
  )
}
