import { ACCESS_HINT, ACCESS_LABEL } from '../access'
import type { Access } from '../types'

type AccessTagProps = { access: Access }

export function AccessTag({ access }: AccessTagProps) {
  return (
    <span className="inline-flex h-6 flex-none items-center rounded-sm border border-line-strong px-2 text-small text-soft" title={ACCESS_HINT[access]}>
      {ACCESS_LABEL[access]}
    </span>
  )
}
