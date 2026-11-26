import { sessionActions } from '../actions/sessions'
import { useOrchestrator } from '../OrchestratorContext'
import { cleanupForced, cleanupNowShown, cleanupRequest } from '../cleanupConfirm'
import { confirmed } from '../confirm'
import { useLayout } from '../LayoutContext'
import type { Session } from '../types'
import type { Point } from '../useFloatingAt'
import { ContextMenu } from './ContextMenu'

// The row keeps the rename field the menu starts in it, and its delete, which its own Delete button asks for too.
type ThreadMenuProps = {
  session: Session
  selected: boolean
  at: Point
  onClose(): void
  onRename(): void
  onDelete(): void
}

const CLEANUP_LABEL: Record<Session['cleanup'], string> = { '': '', pending: '', ask: 'Clean up', blocked: 'Clean up anyway' }

// A sleep stops the subagents with Claude.
const subagentsWorking = (n: number) => (n === 0 ? undefined : `${n} ${n === 1 ? 'subagent is' : 'subagents are'} still working`)

export function ThreadMenu({ session, selected, at, onClose, onRename, onDelete }: ThreadMenuProps) {
  const a = useOrchestrator()
  const layout = useLayout()
  const { rightOpen, setRightOpen, toggleRight } = layout
  const openInEditor = sessionActions({ a, layout, current: session }).find((x) => x.id === 'open-in-editor')
  const settled = session.state === 'settled'
  const panelShown = selected && rightOpen

  const toggleOwnPanel = () => {
    if (panelShown) return a.report(toggleRight)
    if (!selected) a.select(session.id)
    setRightOpen(true)
  }

  const cleanUp = () => confirmed(cleanupRequest(session), () => a.cleanupSession(session.id, cleanupForced(session.cleanup)))

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
        cleanupNowShown(session) && { label: 'Clean up now', run: () => a.report(() => a.cleanupSession(session.id, false)) },
        CLEANUP_LABEL[session.cleanup] !== '' && { label: CLEANUP_LABEL[session.cleanup], run: () => a.report(cleanUp) },
        { label: panelShown ? 'Hide right panel' : 'Show right panel', run: toggleOwnPanel },
        openInEditor !== undefined && { label: openInEditor.label, run: () => a.report(() => openInEditor.run([])) },
        settled && { label: 'Wake', separated: true, run: () => a.report(() => a.wakeThread(session.id)) },
        !settled && { label: 'Sleep', danger: true, separated: true, detail: subagentsWorking(session.subagents), run: () => a.report(() => a.settleThread(session.id)) },
        { label: 'Delete', danger: true, run: onDelete },
      ]}
    />
  )
}
