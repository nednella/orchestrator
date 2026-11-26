import { useOrchestrator } from './OrchestratorContext'
import { termsOf } from './projectTerminalOps'
import { useSavedTerms, useStartedShells } from './projectTerminals'
import type { ProjectTerms } from './terminalGroups'
import type { Shell } from './types'

export type ProjectTerminals = { project: string; thread: string; terms: ProjectTerms; shells: Shell[] }

// useProjectTerminals is the current project's terminals: its shells in the groups the owner made of them. The open
// thread only picks where a new one starts.
export function useProjectTerminals(): ProjectTerminals {
  const { shells, project, selectedId } = useOrchestrator('shells', 'project', 'selectedId')
  const key = project?.key ?? ''
  useStartedShells()
  useSavedTerms(key)
  return { project: key, thread: selectedId ?? '', terms: termsOf(key, shells), shells }
}
