import { shellsOf } from '../shells'
import { drawerOpen, setDrawerOpen } from '../terminalDrawer'
import { openTerminal } from '../useThreadTerminals'
import type { Action, ActionContext, Shortcut } from './types'

export const NEW_TERMINAL_KEY: Shortcut = { key: 't' }
export const SPLIT_TERMINAL_KEY: Shortcut = { key: 'd' }

export const shellActions = ({ a, layout }: ActionContext): Action[] => {
  const thread = a.selectedId ?? ''
  const open = async (beside: boolean) => {
    if (!a.project) throw 'The terminal is unavailable'
    a.focus('shell')
    await openTerminal(thread, shellsOf(a.shells, thread), beside)
  }
  return [
    {
      id: 'toggle-shell',
      label: 'Toggle terminal',
      group: 'Navigate',
      shortcut: { key: 'j' },
      run() {
        if (!a.project) throw 'The terminal is unavailable'
        // The expanded right panel hides the chat column and the drawer in it; the focus gives the column back.
        if (drawerOpen(thread) && !layout.panelExpanded) {
          if (document.activeElement?.closest('[data-panel="shell"]')) a.focus('thread')
          return setDrawerOpen(thread, false)
        }
        setDrawerOpen(thread, true)
        a.focus('shell')
      },
    },
    {
      id: 'new-terminal',
      label: 'New terminal',
      group: 'Navigate',
      shortcut: NEW_TERMINAL_KEY,
      run: () => open(false),
    },
    {
      id: 'split-terminal',
      label: 'Split terminal side by side',
      group: 'Navigate',
      shortcut: SPLIT_TERMINAL_KEY,
      run: () => open(true),
    },
  ]
}
