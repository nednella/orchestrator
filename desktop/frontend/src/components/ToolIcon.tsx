import { Bot, Ellipsis, ExternalLink, FileText, ListChecks, Search, Sparkles, SquarePen, SquareTerminal } from 'lucide-react'
import type { ToolIconName } from '../chatTools'

const ICONS = { terminal: SquareTerminal, file: FileText, compose: SquarePen, search: Search, external: ExternalLink, agent: Bot, check: ListChecks, model: Sparkles, more: Ellipsis }

type ToolIconProps = { name: ToolIconName; size: number }

export function ToolIcon({ name, size }: ToolIconProps) {
  const Icon = ICONS[name]
  return <Icon size={size} aria-hidden="true" />
}
