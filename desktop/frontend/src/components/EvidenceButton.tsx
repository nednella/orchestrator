import { useAgentos } from '../AgentosContext'
import { useLayout } from '../LayoutContext'
import type { Session } from '../types'
import { Icon } from './Icon'

type EvidenceButtonProps = { session: Session }

export function EvidenceButton({ session }: EvidenceButtonProps) {
  const { select } = useAgentos('select')
  const { showView } = useLayout()
  return (
    <button
      className="ml-auto inline-flex h-6 flex-none items-center gap-1 rounded-sm border border-line-strong px-2 text-small text-soft"
      title={`${session.evidence} media items`}
      onClick={() => {
        select(session.id)
        showView(session.id, 'evidence')
      }}
    >
      <Icon name="image" size={12} />
      <span className="mono">{session.evidence}</span>
    </button>
  )
}
