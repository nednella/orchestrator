import type { KeyboardEvent } from 'react'
import { useAgentos } from '../AgentosContext'
import { waitingByProject } from '../threads'
import type { Project } from '../types'
import { ProjectTile } from './ProjectTile'
import { RailButton } from './RailButton'

function tileLabels(projects: Project[]): string[] {
  return projects.map((p) => {
    const name = p.name.toUpperCase()
    const others = projects.filter((o) => o !== p).map((o) => o.name.toUpperCase())
    for (let n = 1; n < name.length; n++) {
      if (!others.some((o) => o.startsWith(name.slice(0, n)))) return name.slice(0, n)
    }
    return name
  })
}

export function ProjectRail() {
  const { projects, sessions } = useAgentos('projects', 'sessions')
  const waiting = waitingByProject(sessions)
  const labels = tileLabels(projects)

  const onKeyDown = (e: KeyboardEvent<HTMLElement>) => {
    const step = e.key === 'ArrowDown' ? 1 : e.key === 'ArrowUp' ? -1 : 0
    if (step === 0) return
    const items = [...e.currentTarget.querySelectorAll<HTMLElement>('[data-rail-item]')]
    const at = items.indexOf(document.activeElement as HTMLElement)
    if (at < 0) return
    e.preventDefault()
    e.stopPropagation()
    items[(at + step + items.length) % items.length].focus()
  }

  return (
    <nav className="flex w-9 flex-none flex-col items-center gap-1" aria-label="Projects" onKeyDown={onKeyDown}>
      {projects.map((p, i) => (
        <ProjectTile key={p.key} project={p} label={labels[i]} waiting={waiting.get(p.key) ?? 0} />
      ))}
      <RailButton id="add-project" label="Add project…" icon="plus" />
      <span className="flex-1" />
      <RailButton id="settings" label="Settings" icon="settings" />
    </nav>
  )
}
