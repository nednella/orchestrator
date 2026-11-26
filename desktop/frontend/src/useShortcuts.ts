import { useEffect, useRef } from 'react'
import { useLatestOrchestrator } from './OrchestratorContext'
import { actionsFor, matchShortcut } from './actions'
import { useLayout } from './LayoutContext'

export function useShortcuts(enabled: boolean) {
  const latestOrchestrator = useLatestOrchestrator()
  const layout = useLayout()
  const latest = useRef({ layout, enabled })
  latest.current = { layout, enabled }

  useEffect(() => {
    const onKeyDown = (e: KeyboardEvent) => {
      if (!latest.current.enabled) return
      const orchestrator = latestOrchestrator()
      const action = actionsFor(orchestrator, latest.current.layout).find((x) => x.shortcut && matchShortcut(e, x.shortcut))
      if (action) {
        e.preventDefault()
        orchestrator.report(() => action.run())
      }
    }
    const onEscape = (e: KeyboardEvent) => {
      if (e.key !== 'Escape' || !latest.current.enabled) return
      const orchestrator = latestOrchestrator()
      const lay = latest.current.layout
      if (orchestrator.overlay) {
        orchestrator.setOverlay(null)
        return
      }
      if (lay.overlay) {
        // An Esc held down since it closed something else does not close the panel under it too.
        if (!e.repeat) lay.closeOverlay('escape')
        return
      }
      if (lay.statsOpen || lay.digestOpen) {
        lay.closeCentre()
        orchestrator.focus('thread')
        return
      }
      const el = document.activeElement
      if (el instanceof HTMLElement && el.matches('input, textarea') && !el.closest('.xterm')) {
        el.blur()
        orchestrator.focus(lay.panelExpanded && el.closest('[data-panel="right"]') ? 'right' : 'thread')
      }
    }
    window.addEventListener('keydown', onKeyDown, true)
    window.addEventListener('keydown', onEscape)
    return () => {
      window.removeEventListener('keydown', onKeyDown, true)
      window.removeEventListener('keydown', onEscape)
    }
  }, [latestOrchestrator])
}
