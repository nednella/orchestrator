import { useAgentos } from '../AgentosContext'
import { useLayout } from '../LayoutContext'
import type { Session } from '../types'
import type { Point } from '../useFloatingAt'
import { ContextMenu } from './ContextMenu'

// The row keeps what the menu starts in it: the rename field, the timeline and the confirms.
type ThreadMenuProps = {
  session: Session
  selected: boolean
  at: Point
  onClose(): void
  onRename(): void
  onTimeline(): void
  onCleanup(): void
  onKill(): void
}

const CLEANUP_LABEL: Record<Session['cleanup'], string> = { '': '', pending: '', ask: 'Clean up', blocked: 'Clean up anyway' }

export function ThreadMenu({ session, selected, at, onClose, onRename, onTimeline, onCleanup, onKill }: ThreadMenuProps) {
  const a = useAgentos('select', 'resumeSession', 'dismissSession', 'openPR', 'pushToast', 'report')
  const { mode, mobilePanel, rightOpen, showPanel, toggleRight } = useLayout()
  const ended = session.state === 'ended'
  const panelShown = selected && (mode === 'narrow' ? mobilePanel === 'panel' : rightOpen)

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
        !session.chat && { label: 'Show timeline', run: onTimeline },
        session.pr !== null && { label: `Open PR #${session.pr.number}`, run: () => a.report(() => a.openPR(session)) },
        session.branch !== '' && { label: 'Copy branch', run: () => a.report(copyBranch) },
        CLEANUP_LABEL[session.cleanup] !== '' && { label: CLEANUP_LABEL[session.cleanup], run: onCleanup },
        { label: panelShown ? 'Hide right panel' : 'Show right panel', run: toggleOwnPanel },
        ended && session.resumable && { label: 'Resume', separated: true, run: () => a.report(() => a.resumeSession(session.id)) },
        ended && { label: 'Dismiss', separated: !session.resumable, run: () => a.report(() => a.dismissSession(session.id)) },
        !ended && { label: 'Kill thread', danger: true, separated: true, run: onKill },
      ]}
    />
  )
}
