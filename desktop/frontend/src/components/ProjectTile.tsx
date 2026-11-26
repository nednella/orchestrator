import { useState } from 'react'
import { useOrchestrator } from '../OrchestratorContext'
import type { Project } from '../types'
import type { Point } from '../useFloatingAt'
import { ProjectMenu } from './ProjectMenu'

type ProjectTileProps = { project: Project; label: string; waiting: number }

export function ProjectTile({ project, label, waiting }: ProjectTileProps) {
  const { project: current, switchProject, report } = useOrchestrator('project', 'switchProject', 'report')
  const [menuAt, setMenuAt] = useState<Point | null>(null)
  const selected = project.key === current?.key
  const needs = selected ? 0 : waiting
  return (
    <>
      <button
        className="btn btn-ghost relative h-9 w-9 flex-none justify-center px-0 font-semibold"
        title={needs > 0 ? `${project.name}, ${needs} waiting` : project.name}
        aria-label={needs > 0 ? `${project.name}, ${needs} waiting` : project.name}
        aria-current={selected}
        data-selected={selected}
        data-rail-item
        onClick={() => report(() => switchProject(project.name, true))}
        onContextMenu={(e) => {
          e.preventDefault()
          setMenuAt({ x: e.clientX, y: e.clientY })
        }}
        onKeyDown={(e) => {
          if (e.key !== 'ContextMenu' && !(e.shiftKey && e.key === 'F10')) return
          e.preventDefault()
          const box = e.currentTarget.getBoundingClientRect()
          setMenuAt({ x: box.left + 12, y: box.bottom })
        }}
      >
        {label}
        {needs > 0 && <span className="dot absolute top-1 right-1" data-state="waiting" />}
      </button>
      {menuAt && <ProjectMenu project={project} at={menuAt} onClose={() => setMenuAt(null)} />}
    </>
  )
}
