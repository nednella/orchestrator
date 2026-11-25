import { useAgentos } from '../AgentosContext'
import { passingReason } from '../cleanupConfirm'
import { useLayout } from '../LayoutContext'
import type { Session } from '../types'
import type { Point } from '../useFloatingAt'
import { ContextMenu } from './ContextMenu'

// The row keeps what the menu starts in it: the rename field and the clean-up confirm.
type ThreadMenuProps = {
  session: Session
  selected: boolean
  at: Point
  onClose(): void
  onRename(): void
  onCleanup(): void
}

const CLEANUP_LABEL: Record<Session['cleanup'], string> = { '': '', pending: '', ask: 'Clean up', blocked: 'Clean up anyway' }

export function ThreadMenu({ session, selected, at, onClose, onRename, onCleanup }: ThreadMenuProps) {
  const a = useAgentos('select', 'settleThread', 'wakeThread', 'archiveThread', 'openPR', 'cleanupSession', 'pushToast', 'report')
  const { rightOpen, showPanel, toggleRight } = useLayout()
  const settled = session.state === 'settled'
  const panelShown = selected && rightOpen

  const toggleOwnPanel = () => {
    if (panelShown) return a.report(toggleRight)
    if (!selected) a.select(session.id)
    showPanel('details')
  }

  const copyBranch = async () => {
    await navigator.clipboard.writeText(session.branch)
    a.pushToast({ tone: 'info', text: `Copied ${session.branch}` })
  }

  return (
    <ContextMenu
      label={`Actions for ${session.title}`}
      at={at}
      onClose={onClose}
      items={[
        { label: 'Rename', run: onRename },
        session.pr !== null && { label: `Open PR #${session.pr.number}`, run: () => a.report(() => a.openPR(session)) },
        session.branch !== '' && { label: 'Copy branch', run: () => a.report(copyBranch) },
        session.cleanup === 'blocked' && passingReason(session.cleanupReason) && { label: 'Clean up', run: () => a.report(() => a.cleanupSession(session.id, false)) },
        CLEANUP_LABEL[session.cleanup] !== '' && { label: CLEANUP_LABEL[session.cleanup], run: onCleanup },
        { label: panelShown ? 'Hide right panel' : 'Show right panel', run: toggleOwnPanel },
        settled && session.hosted && { label: 'Wake', separated: true, run: () => a.report(() => a.wakeThread(session.id)) },
        !settled && { label: 'Settle', danger: true, separated: true, run: () => a.report(() => a.settleThread(session.id)) },
        { label: 'Archive', danger: true, separated: settled && !session.hosted, run: () => a.report(() => a.archiveThread(session.id)) },
      ]}
    />
  )
}
