import { useRef } from 'react'
import type { CSSProperties } from 'react'
import { useAgentos } from '../AgentosContext'
import { formatShortcut } from '../actions'
import { NEW_TERMINAL_KEY } from '../actions/shell'
import { termSession } from '../terminal/session'
import { useDrawerCap, useDrawerDrag, useDrawerHeight, useDrawerOpen } from '../terminalDrawer'
import { activeShell, MAX_PANES, showsList } from '../terminalGroups'
import { useFocusRequest } from '../useFocusRequest'
import { useKeptTerminals, useThreadTerminals } from '../useThreadTerminals'
import { Keycap } from './Keycap'
import { TerminalButtons } from './TerminalButtons'
import { TerminalList } from './TerminalList'
import { TerminalSplit } from './TerminalSplit'

// TerminalDrawer holds the open thread's terminals, or Home's, under the chat, at the height dragged on its top edge.
export function TerminalDrawer() {
  const { selectedId, project, report } = useAgentos('selectedId', 'project', 'report')
  const thread = selectedId ?? ''
  const open = useDrawerOpen(thread)
  const height = useDrawerHeight(thread)
  const t = useThreadTerminals(thread)
  const drawer = useRef<HTMLDivElement>(null)
  const resize = useDrawerDrag(thread, drawer)
  useKeptTerminals(thread)
  useDrawerCap(drawer, !!project && open)
  useFocusRequest('shell', () => termSession(activeShell(t.terms)).focus(), open && t.terms.groups.length > 0)

  if (!project || !open) return null

  const shown = t.terms.groups.find((g) => g.id === t.terms.active)
  const active = activeShell(t.terms)
  const closeActive = () => {
    if (active) report(() => t.close(active))
  }
  const buttons = (floating: boolean) => (
    <TerminalButtons
      floating={floating}
      full={(shown?.panes.length ?? 0) >= MAX_PANES}
      onSplit={() => report(() => t.open(true))}
      onNew={() => report(() => t.open(false))}
      onClose={closeActive}
    />
  )

  return (
    <div
      ref={drawer}
      data-panel="shell"
      tabIndex={-1}
      className="relative isolate flex flex-none border-t border-(--border-terminal) bg-app"
      style={{ '--terminal-height': `${height}px`, height: 'max(var(--terminal-height), 180px)', maxHeight: 'var(--terminal-max, 75vh)' } as CSSProperties}
    >
      <div
        className="absolute inset-x-0 -top-[3px] z-30 h-[6px] cursor-row-resize touch-none pointer-coarse:-top-2 pointer-coarse:h-4"
        aria-hidden="true"
        {...resize}
      />
      {shown ? (
        <>
          <TerminalSplit group={shown} onActivate={t.select} />
          {showsList(t.terms) ? (
            <TerminalList
              terms={t.terms}
              shells={t.shells}
              buttons={buttons(false)}
              onSelect={(id) => {
                t.select(id)
                termSession(id).focus()
              }}
              onClose={(id) => report(() => t.close(id))}
            />
          ) : (
            buttons(true)
          )}
        </>
      ) : (
        <div className="flex flex-1 flex-col items-center justify-center gap-3 text-small text-dim">
          {thread ? 'No terminal for this thread yet.' : 'No terminal for this project yet.'}
          <button className="btn h-6 gap-2 bg-transparent px-2 text-label" onClick={() => report(() => t.open(false))}>
            New terminal
            <Keycap>{formatShortcut(NEW_TERMINAL_KEY)}</Keycap>
          </button>
        </div>
      )}
    </div>
  )
}
