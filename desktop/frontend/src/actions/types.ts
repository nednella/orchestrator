import type { Orchestrator } from '../OrchestratorContext'
import type { Layout } from '../LayoutContext'
import type { Session } from '../types'

export type Shortcut = {
  key: string
  shift?: boolean
  alt?: boolean
  code?: string
  aliases?: string[]
  anyShift?: boolean
  unlessTyping?: boolean
  label?: string
}
export type ActionGroup = 'Sessions' | 'Navigate' | 'Projects' | 'Notes' | 'Queue' | 'App'

export type Action = {
  id: string
  label: string
  group: ActionGroup
  shortcut?: Shortcut
  run(args: string[]): string | void | Promise<string | void>
}

export type ActionContext = { a: Orchestrator; layout: Layout; current: Session | undefined }
