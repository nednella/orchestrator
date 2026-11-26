import { useEffect, useRef } from 'react'
import type { KeyboardEvent } from 'react'
import { useAgentosSelector } from '../AgentosContext'
import { useLayout } from '../LayoutContext'
import { PANEL_KEYS, viewForKey } from '../panelTabs'
import type { PanelViewId } from '../panelTabs'
import { LISTED_VIEWS, panelView, usePanelMeta } from '../panelViews'
import type { Session } from '../types'
import { isTyping } from '../useListNav'

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

// PanelHome is the right panel's list of views. A view of one thread's things cannot open on Home.
export function PanelHome({ session }: PanelHomeProps) {
  const { showPanelView } = useLayout()
  const title = useAgentosSelector((a) => session?.title ?? a.project?.name ?? '')
  const meta = usePanelMeta(session)
  const opens = (id: PanelViewId) => session !== undefined || panelView(id)?.thread === false
  usePanelLetters((id) => opens(id) && showPanelView(id))

  const onKeyDown = (e: KeyboardEvent<HTMLDivElement>) => {
    if (e.key !== 'ArrowDown' && e.key !== 'ArrowUp') return
    e.preventDefault()
    const rows = [...e.currentTarget.querySelectorAll<HTMLElement>('[data-panel-view]')]
    const at = rows.indexOf(document.activeElement as HTMLElement)
    const step = e.key === 'ArrowDown' ? 1 : -1
    rows[at < 0 ? (step > 0 ? 0 : rows.length - 1) : (at + step + rows.length) % rows.length]?.focus()
  }

  return (
    <div className="absolute inset-0 overflow-y-auto">
      <div className="mx-auto flex w-full max-w-80 flex-col gap-px p-6" onKeyDown={onKeyDown}>
        <h2 className="mb-2 truncate px-2.5 text-body font-medium">{title}</h2>
        {LISTED_VIEWS.map((view) => {
          const shut = !opens(view.id)
          return (
            <button
              key={view.id}
              data-panel-view={view.id}
              aria-disabled={shut}
              aria-keyshortcuts={PANEL_KEYS[view.id]}
              title={shut ? 'Available in a thread' : undefined}
              className="flex h-8 w-full flex-none items-center gap-2.5 rounded-lg px-2.5 text-left text-body hover:bg-(--bg-row-hover) focus-visible:bg-(--bg-row-hover) aria-disabled:cursor-default aria-disabled:opacity-50 aria-disabled:hover:bg-transparent"
              onClick={() => !shut && showPanelView(view.id)}
            >
              <view.icon size={16} className="flex-none text-soft" aria-hidden="true" />
              <span className="min-w-0 truncate">{view.label}</span>
              <span className="ml-auto truncate pl-2 text-small text-soft">{shut ? '' : meta[view.id]}</span>
              <kbd className="keycap flex-none" aria-hidden="true">
                {PANEL_KEYS[view.id]}
              </kbd>
            </button>
          )
        })}
      </div>
    </div>
  )
}
