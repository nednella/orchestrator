import { useEffect, useRef } from 'react'
import type { KeyboardEvent } from 'react'
import { useLayout } from '../LayoutContext'
import { useOrchestrator } from '../OrchestratorContext'
import { targetForKey } from '../panelTabs'
import type { PanelRowTarget } from '../panelTabs'
import { PANEL_GROUPS, panelView } from '../panelViews'
import type { PanelRow } from '../panelViews'
import { sideChatReason } from '../sideChat'
import { useSideChatReason } from '../sideChatStore'
import type { Thread } from '../types'
import { showTerminal } from '../terminalSurface'
import { isTyping } from '../useListNav'
import { useSideChat } from '../useSideChat'
import { PanelHomeRow } from './PanelHomeRow'

type PanelHomeProps = { thread: Thread | undefined }

const POPUP = '[role="menu"], [role="dialog"], [role="alertdialog"], [role="listbox"]'

// usePanelLetters does what each bare letter pressed anywhere in the window does from the list, but not while the owner
// types or a popup is open, or once something else took the key.
function usePanelLetters(open: (target: PanelRowTarget) => void) {
  const latest = useRef(open)
  latest.current = open
  useEffect(() => {
    const onKeyDown = (e: globalThis.KeyboardEvent) => {
      if (e.defaultPrevented || e.repeat || e.metaKey || e.ctrlKey || e.altKey || e.shiftKey || isTyping(e.target) || document.querySelector(POPUP)) return
      const target = targetForKey(e.key)
      if (!target) return
      e.preventDefault()
      latest.current(target)
    }
    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [])
}

const FORWARD = ['ArrowDown', 'ArrowRight']
const BACK = ['ArrowUp', 'ArrowLeft']

// PanelHome is the right panel's list of surfaces to open. A view of one thread's things cannot open on Home, and a
// side chat starts only from a thread that can fork.
export function PanelHome({ thread }: PanelHomeProps) {
  const layout = useLayout()
  const orchestrator = useOrchestrator('project', 'selectedId', 'shells', 'report', 'digestUnseenCount')
  const sideChat = useSideChat()
  const chatReason = useSideChatReason(thread?.id ?? '')
  const sideReason = sideChatReason(thread) || chatReason
  const reasonShut = (row: PanelRow) => {
    if (row.target === 'sideChat') return sideReason || null
    if (row.target === 'terminal' && !orchestrator.project) return 'Terminals are available from a project.'
    return thread === undefined && panelView(row.target)?.thread ? 'Available from a thread.' : null
  }
  // A new terminal for the Terminal row, as it makes its tab with each press.
  const open = (target: PanelRowTarget) => {
    if (target === 'sideChat') return thread && orchestrator.report(() => sideChat.start(thread.id))
    if (target === 'terminal') return orchestrator.report(() => showTerminal(orchestrator, layout, true))
    layout.showPanelView(target)
  }
  usePanelLetters((target) => {
    const row = PANEL_GROUPS.flatMap((g) => g.rows).find((r) => r.target === target)
    if (row && reasonShut(row) === null) open(target)
  })

  const onKeyDown = (e: KeyboardEvent<HTMLDivElement>) => {
    const step = FORWARD.includes(e.key) ? 1 : BACK.includes(e.key) ? -1 : 0
    if (step === 0) return
    e.preventDefault()
    const rows = [...e.currentTarget.querySelectorAll<HTMLElement>('[data-panel-view]:not([aria-disabled="true"])')]
    const at = rows.indexOf(document.activeElement as HTMLElement)
    rows[at < 0 ? (step > 0 ? 0 : rows.length - 1) : (at + step + rows.length) % rows.length]?.focus()
  }

  return (
    <div className="absolute inset-0 overflow-y-auto">
      <div className="flex min-h-full items-center justify-center px-6 pb-[52px]">
        <div className="w-full max-w-80 py-6" onKeyDown={onKeyDown}>
          <h3 className="mb-3 text-center text-body font-medium">Open a surface</h3>
          {PANEL_GROUPS.map((group) => (
            <section key={group.heading} className="flex flex-col gap-0.5 not-first:mt-1">
              <h4 className="flex min-h-8 items-center px-2.5 text-label font-medium text-soft">{group.heading}</h4>
              {group.rows.map((row) => (
                <PanelHomeRow key={row.label} row={row} reason={reasonShut(row)} unseen={row.target === 'news' ? orchestrator.digestUnseenCount : 0} onOpen={() => open(row.target)} />
              ))}
            </section>
          ))}
        </div>
      </div>
    </div>
  )
}
