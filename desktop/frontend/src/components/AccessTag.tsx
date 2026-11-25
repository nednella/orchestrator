import { ACCESS_LABEL } from '../access'
import type { Access } from '../types'

const HINT: Record<Access, string> = {
  read: 'It reads the project and plans; Claude asks before it changes anything.',
  supervised: 'Your Claude allow rules apply; other tools ask in the chat.',
  full: 'Every tool runs without asking.',
}

type AccessTagProps = { access: Access }

export function AccessTag({ access }: AccessTagProps) {
  return (
    <span className="inline-flex h-6 flex-none items-center rounded-sm border border-line-strong px-2 text-small text-soft" title={HINT[access]}>
      {ACCESS_LABEL[access]}
    </span>
  )
}
