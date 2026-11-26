import type { ReactNode } from 'react'

type SettingsRowProps = { title: string; description: string; control: ReactNode; children?: ReactNode }

// Below 32rem of row width the control stacks under the text; from there the text takes the rest beside it.
export function SettingsRow({ title, description, control, children }: SettingsRowProps) {
  return (
    <div className="@container border-t border-(--border-rule) px-4 py-3 first:border-t-0">
      <div className="grid gap-3 @lg:grid-cols-[minmax(0,1fr)_auto] @lg:items-center @lg:gap-8">
        <div className="min-w-0 space-y-1">
          <h3 className="min-h-5 text-body font-medium">{title}</h3>
          <p className="max-w-2xl text-small leading-normal text-pretty text-dim">{description}</p>
        </div>
        <div className="flex items-center gap-2 @lg:justify-end">{control}</div>
      </div>
      {children && <div className="pt-3">{children}</div>}
    </div>
  )
}
