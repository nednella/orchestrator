import { CircleDashed, CircleQuestionMark, Lock, type LucideIcon } from 'lucide-react'
import type { WaitKind } from './types'

export const KIND_COLOR: Record<WaitKind, string> = {
  permission: 'var(--waiting)',
  question: 'var(--accent)',
  idle: 'var(--idle)',
}

export const KIND_LABEL: Record<WaitKind, string> = {
  permission: 'Permission',
  question: 'Question',
  idle: 'Reply landed',
}

export const KIND_ICON: Record<WaitKind, LucideIcon> = {
  permission: Lock,
  question: CircleQuestionMark,
  idle: CircleDashed,
}
