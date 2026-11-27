import { Eye, Lock, ShieldOff } from 'lucide-react'
import type { Access } from '../types'

const ICONS = { read: Eye, supervised: Lock, full: ShieldOff }

type AccessIconProps = { access: Access }

export function AccessIcon({ access }: AccessIconProps) {
  const Shape = ICONS[access]
  return <Shape size={14} className="flex-none" aria-hidden="true" />
}
