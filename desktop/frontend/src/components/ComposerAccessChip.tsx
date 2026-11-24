import type { ComposerKind } from '../AgentosContext'
import { ComposerMenuChip } from './ComposerMenuChip'
import { Icon } from './Icon'
import { SegmentedControl } from './SegmentedControl'

const ACCESS: Record<ComposerKind, { label: string; hint: string }> = {
  chat: { label: 'Read', hint: 'A chat: it reads the project and answers, and changes no files.' },
  session: { label: 'Write', hint: 'A task: it works on its own branch and can change files.' },
}

type ComposerAccessChipProps = { kind: ComposerKind; onKind(kind: ComposerKind): void }

export function ComposerAccessChip({ kind, onKind }: ComposerAccessChipProps) {
  return (
    <ComposerMenuChip
      label={`Access: ${ACCESS[kind].label}`}
      menu={() => (
        <div className="flex w-64 flex-col gap-1.5 p-1">
          <SegmentedControl
            label="Access"
            variant="track"
            options={[
              { value: 'chat', label: ACCESS.chat.label },
              { value: 'session', label: ACCESS.session.label },
            ]}
            value={kind}
            onChange={onKind}
          />
          <p className="px-1 text-label text-soft">{ACCESS[kind].hint}</p>
        </div>
      )}
    >
      <Icon name={kind === 'chat' ? 'eye' : 'git-branch'} size={14} />
      {ACCESS[kind].label}
    </ComposerMenuChip>
  )
}
