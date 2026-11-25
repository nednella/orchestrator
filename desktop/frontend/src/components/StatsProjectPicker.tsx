import { ChevronDown } from 'lucide-react'
import type { Project } from '../types'

type StatsProjectPickerProps = { projects: Project[]; value: string; onChange(key: string): void }

export function StatsProjectPicker({ projects, value, onChange }: StatsProjectPickerProps) {
  return (
    <span className="relative flex items-center text-ink">
      <select
        aria-label="Project"
        className="h-7 cursor-pointer appearance-none rounded-lg bg-transparent pr-6 pl-2 text-small font-medium [field-sizing:content] hover:bg-hover"
        value={value}
        onChange={(e) => onChange(e.target.value)}
      >
        <option value="">All projects</option>
        {projects.map((p) => (
          <option key={p.key} value={p.key}>
            {p.name}
          </option>
        ))}
      </select>
      <ChevronDown size={12} aria-hidden="true" className="pointer-events-none absolute right-2 text-soft" />
    </span>
  )
}
