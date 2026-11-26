import { Eye, Lock, ShieldOff } from 'lucide-react'
import type { Access } from '../types'

const ICONS = { read: Eye, supervised: Lock, full: ShieldOff }

type AccessIconProps = { access: Access; size?: number }

export function AccessIcon({ access, size = 14 }: AccessIconProps) {
  const Shape = ICONS[access]
  return <Shape size={size} className="flex-none" aria-hidden="true" />
}
