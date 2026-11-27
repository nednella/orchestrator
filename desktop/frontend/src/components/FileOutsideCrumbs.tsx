import { Lock } from 'lucide-react'
import { baseName } from '../fileTree'

type FileOutsideCrumbsProps = { path: string }

// FileOutsideCrumbs is the way to a file outside the thread's folder: its name, with a lock, since it is read only.
export function FileOutsideCrumbs({ path }: FileOutsideCrumbsProps) {
  return (
    <div className="flex min-w-0 flex-1 items-center gap-1.5 overflow-hidden px-1.5 text-small" title={path}>
      <Lock size={12} className="flex-none text-soft" aria-hidden="true" />
      <span className="min-w-0 truncate font-medium text-ink">{baseName(path)}</span>
    </div>
  )
}
