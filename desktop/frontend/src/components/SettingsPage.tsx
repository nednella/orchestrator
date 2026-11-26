import type { ReactNode } from 'react'
import { useOrchestrator } from '../OrchestratorContext'

type SettingsPageProps = { perProject?: boolean; children: ReactNode }

// A page of per-project settings says whose they are, and has nothing to change with no project open.
export function SettingsPage({ perProject = false, children }: SettingsPageProps) {
  const { project } = useOrchestrator('project')
  if (perProject && !project) return <p className="px-4 text-body text-dim">Open a project to change its settings.</p>
  return (
    <div className="flex flex-col gap-8">
      {perProject && project && <p className="px-4 text-small text-dim">These settings apply to {project.name}.</p>}
      {children}
    </div>
  )
}
