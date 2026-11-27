import { FileDiff, TriangleAlert, Upload } from 'lucide-react'
import type { LucideIcon } from 'lucide-react'
import type { Thread } from '../types'
import { Chip } from './Chip'

type WorkChipsProps = { thread: Thread }

export function WorkChips({ thread }: WorkChipsProps) {
  const { git, pr } = thread
  if (!git) return null
  const merged = pr?.state === 'merged'
  const tone = merged ? 'bad' : thread.state === 'working' ? 'dim' : 'warn'
  const items: { n: number; icon: LucideIcon; label: string }[] = [
    merged ? { n: git.notInMain, icon: TriangleAlert, label: 'not in main' } : { n: git.unpushed, icon: Upload, label: 'unpushed' },
    { n: git.uncommitted, icon: FileDiff, label: 'uncommitted' },
  ]

  return items
    .filter((item) => item.n > 0)
    .map(({ n, icon, label }) => (
      <Chip key={label} tone={tone} icon={icon} title={`${n} ${label}`}>
        {`${n} ${label}`}
      </Chip>
    ))
}
