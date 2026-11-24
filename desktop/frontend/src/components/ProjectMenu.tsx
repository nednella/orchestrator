import { useAgentos } from '../AgentosContext'
import { api } from '../api'
import type { Project } from '../types'
import type { Point } from '../useFloatingAt'
import { ContextMenu } from './ContextMenu'

type ProjectMenuProps = { project: Project; at: Point; onClose(): void }

export function ProjectMenu({ project, at, onClose }: ProjectMenuProps) {
  const { switchProject, setUpProject, report } = useAgentos('switchProject', 'setUpProject', 'report')

  const setUp = async () => {
    await switchProject(project.name)
    await setUpProject()
  }

  return (
    <ContextMenu
      label={`Actions for ${project.name}`}
      at={at}
      onClose={onClose}
      items={[
        project.repo !== '' && { label: 'Open on GitHub', run: () => report(() => api.openURL(`https://github.com/${project.repo}`)) },
        { label: 'Set up project', run: () => report(setUp) },
      ]}
    />
  )
}
