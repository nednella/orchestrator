import { useMemo } from 'react'
import type { KeyboardEvent } from 'react'
import { useAgentos } from '../AgentosContext'
import { RAIL_REM } from '../LayoutContext'
import { waitingByProject } from '../threads'
import type { Project } from '../types'
import { ProjectTile } from './ProjectTile'
import { RailButton } from './RailButton'

function tileLabels(projects: Project[]): string[] {
  const names = projects.map((p) => p.name.toUpperCase())
  const taken = new Set<string>()
  return names.map((name, i) => {
    const words = name.split(/[^A-Z0-9]+/).filter(Boolean)
    const unique = names.filter((o) => o[0] === name[0]).length === 1
    const candidates = [
      ...(unique ? [name[0]] : []),
      name.slice(0, 2),
      words.length > 1 ? words[0][0] + words[1][0] : '',
      name[0] + name.slice(-1),
      ...Array.from({ length: 10 }, (_, n) => `${name[0]}${n}`),
    ]
    const label = candidates.find((c) => c && !taken.has(c) && !names.some((o, j) => j !== i && o === c)) ?? name[0]
    taken.add(label)
    return label
  })
}

export function ProjectRail() {
  const { projects, sessions, threadTree } = useAgentos('projects', 'sessions', 'threadTree')
  const waiting = useMemo(() => waitingByProject(sessions, threadTree), [sessions, threadTree])
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
    <nav className="flex flex-none flex-col items-center gap-1 border-r border-line py-2" style={{ width: `${RAIL_REM}rem` }} aria-label="Projects" onKeyDown={onKeyDown}>
      {projects.map((p, i) => (
        <ProjectTile key={p.key} project={p} label={labels[i]} waiting={waiting.get(p.key) ?? 0} />
      ))}
      <RailButton id="add-project" label="Add project…" icon="plus" />
      <span className="flex-1" />
      <RailButton id="settings" label="Settings" icon="settings" />
    </nav>
  )
}
