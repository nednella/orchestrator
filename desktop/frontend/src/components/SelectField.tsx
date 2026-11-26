import { ChevronDown } from 'lucide-react'

type SelectOption<T extends string> = { value: T; label: string }

type SelectFieldProps<T extends string> = {
  label: string
  options: SelectOption<T>[]
  value: T
  onChange(value: T): void
}

export function SelectField<T extends string>({ label, options, value, onChange }: SelectFieldProps<T>) {
  return (
    <span className="relative flex min-w-40 items-center">
      <select
        aria-label={label}
        className="h-8 w-full cursor-pointer appearance-none rounded-lg border border-line-strong bg-raised pr-8 pl-3 text-body hover:bg-hover"
        value={value}
        onChange={(e) => onChange(e.target.value as T)}
      >
        {options.map((option) => (
          <option key={option.value} value={option.value}>
            {option.label}
          </option>
        ))}
      </select>
      <ChevronDown size={14} aria-hidden="true" className="pointer-events-none absolute right-3 text-soft" />
    </span>
  )
}
