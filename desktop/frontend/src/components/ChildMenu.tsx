import { useAgentos, useLatestAgentos } from '../AgentosContext'
import { api } from '../api'
import { confirmDelete } from '../confirm'
import type { Session } from '../types'
import type { Point } from '../useFloatingAt'
import { ContextMenu } from './ContextMenu'

type ChildMenuProps = { child: Session; at: Point; onClose(): void }

// ChildMenu is the right-click menu of a delegated child, wherever its row shows.
export function ChildMenu({ child, at, onClose }: ChildMenuProps) {
  const a = useAgentos('select', 'settleThread', 'wakeThread', 'deleteThread', 'report')
  const latest = useLatestAgentos()
  const working = child.state === 'working' || child.state === 'waiting'
  const asleep = child.state === 'settled'
  return (
    <ContextMenu
      label={`Actions for ${child.title}`}
      at={at}
      onClose={onClose}
      items={[
        { label: 'Open', run: () => a.select(child.id) },
        working && { label: 'Stop', run: () => a.report(() => api.chatInterrupt(child.id)) },
        asleep && { label: 'Wake', separated: true, run: () => a.report(() => a.wakeThread(child.id)) },
        !asleep && { label: 'Sleep', danger: true, separated: true, run: () => a.report(() => a.settleThread(child.id)) },
        { label: 'Delete', danger: true, run: () => a.report(() => confirmDelete(child, () => latest().sessions, a.deleteThread)) },
      ]}
    />
  )
}
