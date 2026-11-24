import { useState } from 'react'
import { useAgentos } from '../AgentosContext'
import { useLayout } from '../LayoutContext'
import { gigabytes } from '../memory'
import { dollars, tokenCount } from '../spend'
import { useArmedConfirm } from '../useArmedConfirm'
import type { Session } from '../types'
import { ActionTag } from './ActionTag'
import { Ago } from './Ago'
import { InlineInput } from './InlineInput'
import { ChatTag } from './ChatTag'
import { CleanupControls } from './CleanupControls'
import { Icon } from './Icon'
import { ModelTag } from './ModelTag'
import { StateBadge } from './StateBadge'
import { TimelineButton } from './TimelineButton'

type ViewportHeaderProps = { session: Session }

export function ViewportHeader({ session }: ViewportHeaderProps) {
  const { renameSession, report, focus, killSession, dismissSession, resumeSession } = useAgentos('renameSession', 'report', 'focus', 'killSession', 'dismissSession', 'resumeSession')
  const { rightOpen, toggleRight, mode } = useLayout()
  const [renaming, setRenaming] = useState(false)
  const kill = useArmedConfirm()

  const finishRename = () => {
    setRenaming(false)
    focus('terminal')
  }

  return (
    <header className="flex flex-none flex-col">
      <div className="panel-head flex items-center gap-3 px-4 py-2.5 short:py-1.5">
        <span className="mono w-4 flex-none text-title font-semibold text-soft">{session.n}</span>
        <div className="min-w-0">
          {renaming ? (
            <InlineInput
              initial={session.title}
              blur="submit"
              className="w-64 max-w-full"
              onSubmit={(title) => {
                report(() => renameSession(session.id, title))
                finishRename()
              }}
              onCancel={finishRename}
            />
          ) : (
            <h2
              className="min-w-0 truncate text-title font-semibold"
              title="Double-click to rename"
              onDoubleClick={() => setRenaming(true)}
            >
              {session.title}
            </h2>
          )}
        </div>
        <span className="mono ml-auto flex-none text-small whitespace-nowrap text-dim"><Ago at={session.lastEventAt} /></span>
        {session.cleanup !== '' && <CleanupControls session={session} />}
        {session.state === 'ended' && session.resumable && (
          <button className="btn" title="Start a session that continues this conversation" onClick={() => report(() => resumeSession(session.id))}>
            Resume
          </button>
        )}
        {session.state === 'ended' && (
          <button className="btn" onClick={() => report(() => dismissSession(session.id))}>
            Dismiss
          </button>
        )}
        {session.state === 'ended' ? null : kill.armed ? (
          <span
            className="contents"
            role="alertdialog"
            aria-label="Kill this session"
            onKeyDown={(e) => {
              if (e.key !== 'Escape') return
              e.stopPropagation()
              kill.disarm()
            }}
          >
            <button className="btn" onClick={kill.disarm}>
              Cancel
            </button>
            <button
              className="btn btn-danger"
              autoFocus
              onClick={() => {
                kill.disarm()
                report(() => killSession(session.id))
              }}
            >
              Kill
            </button>
          </span>
        ) : (
          <button className="btn btn-ghost text-danger" onClick={() => kill.arm()} title="Kill this session">
            Kill
          </button>
        )}
        {mode !== 'narrow' && (
          <button
            className="btn btn-ghost h-8 w-8 flex-none justify-center px-0"
            title={`${rightOpen ? 'Hide' : 'Show'} the right panel`}
            aria-label={`${rightOpen ? 'Hide' : 'Show'} the right panel`}
            aria-pressed={rightOpen}
            onClick={() => report(toggleRight)}
          >
            <Icon name="panel-right" />
          </button>
        )}
      </div>
      <div className="short:hidden relative flex flex-wrap items-center gap-x-3 gap-y-1.5 px-4 py-2.5">
        <StateBadge state={session.state} chat={session.chat} />
        <ActionTag action={session.action} />
        {session.chat && <ChatTag />}
        <ModelTag
          agent={session.agent}
          model={session.model}
          effort={session.effort}
          spend={session.tokens > 0 ? `${tokenCount(session.tokens)} tokens · ~${dollars(session.cost)}` : ''}
        />
        {!session.chat && <TimelineButton session={session} />}
        {session.memory > 0 && (
          <span className="mono text-small whitespace-nowrap text-dim" title="Memory this session's processes use">
            {gigabytes(session.memory)}
          </span>
        )}
      </div>
      {kill.armed && (
        <p className="px-4 pb-2 text-center text-small" style={{ color: 'var(--danger)' }}>
          Kill this session? The agent stops now. Its conversation stays on disk and can be resumed from a terminal with{' '}
          <code className="mono">claude --resume</code>.
        </p>
      )}
    </header>
  )
}
