import type { ReactNode } from 'react'

type SegmentedOption<T extends string> = { value: T; label: string; icon?: ReactNode }

// shows is what each option shows of its icon and label; one that shows only its icon has the label as its tooltip.
type SegmentedControlProps<T extends string> = {
  label: string
  options: SegmentedOption<T>[]
  value: T
  variant?: 'outline' | 'track'
  shows?: 'label' | 'icon' | 'both'
  disabled?: boolean
  onChange(value: T): void
}

export function SegmentedControl<T extends string>({ label, options, value, variant = 'outline', shows = 'both', disabled = false, onChange }: SegmentedControlProps<T>) {
  const content = (option: SegmentedOption<T>) => (
    <>
      {shows !== 'label' && option.icon}
      {(shows !== 'icon' || !option.icon) && option.label}
    </>
  )
  const named = (option: SegmentedOption<T>) => (shows === 'icon' && option.icon ? option.label : undefined)

  if (variant === 'track') {
    return (
      <div role="group" aria-label={label} className="segments-track">
        {options.map((option) => (
          <button
            key={option.value}
            className={`inline-flex items-center justify-center gap-1.5 ${named(option) ? 'px-1.5' : ''}`}
            aria-pressed={value === option.value}
            aria-label={named(option)}
            title={named(option)}
            disabled={disabled}
            onMouseDown={(e) => e.preventDefault()}
            onClick={() => onChange(option.value)}
          >
            {content(option)}
          </button>
        ))}
      </div>
    )
  }
  return (
    <div role="group" aria-label={label} className="flex flex-none overflow-hidden rounded-md border border-line-strong">
      {options.map((option) => (
        <button
          key={option.value}
          aria-pressed={value === option.value}
          aria-label={named(option)}
          title={named(option)}
          disabled={disabled}
          className="inline-flex h-7 items-center justify-center gap-1.5 px-3 text-small font-medium"
          style={{ background: value === option.value ? 'var(--bg-hover)' : 'transparent', color: value === option.value ? 'var(--text)' : 'var(--text-soft)' }}
          onClick={() => onChange(option.value)}
        >
          {content(option)}
        </button>
      ))}
    </div>
  )
}
