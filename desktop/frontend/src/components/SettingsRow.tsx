import { useRef } from 'react'
import type { ReactNode } from 'react'
import { ResetButton } from './ResetButton'

// onReset is given only while the value differs from its default; the button beside the title puts the default back.
type SettingsRowProps = { title: string; description: string; control: ReactNode; onReset?(): void; children?: ReactNode }

// Below 32rem of row width the control stacks under the text; from there the text takes the rest beside it.
export function SettingsRow({ title, description, control, onReset, children }: SettingsRowProps) {
  const row = useRef<HTMLDivElement>(null)
  return (
    <div ref={row} tabIndex={-1} className="@container border-t border-(--border-rule) px-4 py-3 outline-none first:border-t-0">
      <div className="grid gap-3 @lg:grid-cols-[minmax(0,1fr)_auto] @lg:items-center @lg:gap-8">
        <div className="min-w-0 space-y-1">
          <div className="flex min-h-5 items-center gap-1.5">
            <h3 className="text-body font-medium">{title}</h3>
            <span className="inline-flex size-5">
              {onReset && (
                <ResetButton
                  label={`Reset ${title.toLowerCase()} to default`}
                  onReset={() => {
                    row.current?.focus()
                    onReset()
                  }}
                />
              )}
            </span>
          </div>
          <p className="max-w-2xl text-small leading-normal text-pretty text-dim">{description}</p>
        </div>
        <div className="flex items-center gap-2 @lg:justify-end">{control}</div>
      </div>
      {children && <div className="pt-3">{children}</div>}
    </div>
  )
}
