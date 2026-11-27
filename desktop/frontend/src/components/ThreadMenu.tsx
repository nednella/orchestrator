import { threadActions } from '../actions/threads'
import { useOrchestrator } from '../OrchestratorContext'
import { cleanupForced, cleanupNowShown, cleanupRequest } from '../cleanupConfirm'
import { confirmed } from '../confirm'
import { useLayout } from '../LayoutContext'
import type { Thread } from '../types'
import type { Point } from '../useFloatingAt'
import { ContextMenu } from './ContextMenu'

// The row keeps the rename field the menu starts in it, and its delete, which its own Delete button asks for too.
type ThreadMenuProps = {
  thread: Thread
  selected: boolean
  at: Point
  onClose(): void
  onRename(): void
  onDelete(): void
}

const CLEANUP_LABEL: Record<Thread['cleanup'], string> = { '': '', pending: '', ask: 'Clean up', blocked: 'Clean up anyway' }

// A sleep stops the subagents with Claude.
const subagentsWorking = (n: number) => (n === 0 ? undefined : `${n} ${n === 1 ? 'subagent is' : 'subagents are'} still working`)

export function ThreadMenu({ thread, selected, at, onClose, onRename, onDelete }: ThreadMenuProps) {
  const a = useOrchestrator()
  const layout = useLayout()
  const { rightOpen, setRightOpen, toggleRight } = layout
  const openInEditor = threadActions({ a, layout, current: thread }).find((x) => x.id === 'open-in-editor')
  const settled = thread.state === 'settled'
  const panelShown = selected && rightOpen

  const toggleOwnPanel = () => {
    if (panelShown) return a.report(toggleRight)
    if (!selected) a.select(thread.id)
    setRightOpen(true)
  }

  const cleanUp = () => confirmed(cleanupRequest(thread), () => a.cleanupThread(thread.id, cleanupForced(thread.cleanup)))

  const copyBranch = async () => {
    await navigator.clipboard.writeText(thread.branch)
    a.pushToast({ tone: 'info', text: `Copied ${thread.branch}` })
  }

  return (
    <ContextMenu
      label={`Actions for ${thread.title}`}
      at={at}
      onClose={onClose}
      items={[
        { label: 'Rename', run: onRename },
        thread.pr !== null && { label: `Open PR #${thread.pr.number}`, run: () => a.report(() => a.openPR(thread)) },
        thread.branch !== '' && { label: 'Copy branch', run: () => a.report(copyBranch) },
        cleanupNowShown(thread) && { label: 'Clean up now', run: () => a.report(() => a.cleanupThread(thread.id, false)) },
        CLEANUP_LABEL[thread.cleanup] !== '' && { label: CLEANUP_LABEL[thread.cleanup], run: () => a.report(cleanUp) },
        { label: panelShown ? 'Hide right panel' : 'Show right panel', run: toggleOwnPanel },
        openInEditor !== undefined && { label: openInEditor.label, run: () => a.report(() => openInEditor.run()) },
        settled && { label: 'Wake', separated: true, run: () => a.report(() => a.wakeThread(thread.id)) },
        !settled && { label: 'Sleep', danger: true, separated: true, detail: subagentsWorking(thread.subagents), run: () => a.report(() => a.settleThread(thread.id)) },
        { label: 'Delete', danger: true, run: onDelete },
      ]}
    />
  )
}
