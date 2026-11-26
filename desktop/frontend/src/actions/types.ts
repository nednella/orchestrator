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
  label?: string
}

export type Action = {
  id: string
  label: string
  shortcut?: Shortcut
  run(): void | Promise<void>
}

export type ActionContext = { a: Orchestrator; layout: Layout; current: Session | undefined }
