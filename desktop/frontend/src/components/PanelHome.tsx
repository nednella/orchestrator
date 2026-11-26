import { useEffect, useRef } from 'react'
import type { KeyboardEvent } from 'react'
import { useLayout } from '../LayoutContext'
import { useOrchestrator } from '../OrchestratorContext'
import { viewForKey } from '../panelTabs'
import type { PanelViewId } from '../panelTabs'
import { PANEL_GROUPS, panelView } from '../panelViews'
import type { PanelRow } from '../panelViews'
import type { Session } from '../types'
import { showTerminal } from '../terminalSurface'
import { isTyping } from '../useListNav'
import { PanelHomeRow } from './PanelHomeRow'

type PanelHomeProps = { session: Session | undefined }

const POPUP = '[role="menu"], [role="dialog"], [role="alertdialog"], [role="listbox"]'

// usePanelLetters opens the view of each bare letter pressed anywhere in the window, but not while the owner types or a
// popup is open, or once something else took the key.
function usePanelLetters(open: (view: PanelViewId) => void) {
  const latest = useRef(open)
  latest.current = open
  useEffect(() => {
    const onKeyDown = (e: globalThis.KeyboardEvent) => {
      if (e.defaultPrevented || e.repeat || e.metaKey || e.ctrlKey || e.altKey || e.shiftKey || isTyping(e.target) || document.querySelector(POPUP)) return
      const view = viewForKey(e.key)
      if (!view) return
      e.preventDefault()
      latest.current(view)
    }
    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [])
}

const FORWARD = ['ArrowDown', 'ArrowRight']
const BACK = ['ArrowUp', 'ArrowLeft']

// PanelHome is the right panel's list of surfaces to open. A view of one thread's things cannot open on Home.
export function PanelHome({ session }: PanelHomeProps) {
  const layout = useLayout()
  const orchestrator = useOrchestrator('project', 'selectedId', 'shells', 'report')
  const reasonShut = (row: PanelRow) => {
    if (row.view === null) return 'Not built yet'
    if (row.view === 'terminal' && !orchestrator.project) return 'Terminals are available from a project.'
    return session === undefined && panelView(row.view)?.thread ? 'Available from a thread.' : null
  }
  // A new terminal for the Terminal row, as it makes its tab with each press.
  const open = (view: PanelViewId) => (view === 'terminal' ? orchestrator.report(() => showTerminal(orchestrator, layout, true)) : layout.showPanelView(view))
  usePanelLetters((id) => {
    const row = PANEL_GROUPS.flatMap((g) => g.rows).find((r) => r.view === id)
    if (row && reasonShut(row) === null) open(id)
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
                <PanelHomeRow key={row.label} row={row} reason={reasonShut(row)} onOpen={() => row.view && open(row.view)} />
              ))}
            </section>
          ))}
        </div>
      </div>
    </div>
  )
}
