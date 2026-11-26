import { api } from '../api'
import { useOrchestrator } from '../OrchestratorContext'
import { useLayout } from '../LayoutContext'
import { workLink, workTitle } from '../statsFigures'
import type { WorkRow } from '../types'

type StatsWorkTitleProps = { row: WorkRow; showProject: boolean }

export function StatsWorkTitle({ row, showProject }: StatsWorkTitleProps) {
  const { projects, sessions, select, report } = useOrchestrator('projects', 'sessions', 'select', 'report')
  const { closeCentre } = useLayout()
  const { ref, title } = workTitle(row)
  const repo = projects.find((p) => p.name === row.project)?.repo ?? ''
  const link = workLink(row, repo, sessions.map((s) => s.id))
  const text = (
    <>
      {ref && <span className="text-soft">{ref} </span>}
      {title}
      {showProject && <span className="text-soft"> · {row.project}</span>}
    </>
  )
  if (!link) return text
  const open = () => {
    if ('url' in link) {
      report(() => api.openURL(link.url))
      return
    }
    closeCentre()
    select(link.thread)
  }
  return (
    <button className="block max-w-full truncate text-left underline-offset-3 hover:underline" title={'url' in link ? `Open ${ref} on GitHub` : 'Open the thread'} onClick={open}>
      {text}
    </button>
  )
}
