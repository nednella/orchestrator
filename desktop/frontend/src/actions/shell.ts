import { closeTerminal, termsOf } from '../projectTerminalOps'
import { activeShell } from '../terminalGroups'
import { showTerminal, splitTerminal } from '../terminalSurface'
import type { Action, ActionContext, Shortcut } from './types'

export const NEW_TERMINAL_KEY: Shortcut = { key: 't' }
export const SPLIT_TERMINAL_KEY: Shortcut = { key: 'd' }
// The keys the terminal takes while it has the focus, in the place of the app's own on the same keys.
export const NEW_TERMINAL_IN_TERMINAL_KEY: Shortcut = { key: 'n' }
export const SPLIT_STACKED_KEY: Shortcut = { key: 'd', shift: true }
export const CLOSE_TERMINAL_KEY: Shortcut = { key: 'w' }

export const shellActions = ({ a, layout }: ActionContext): Action[] => [
  {
    id: 'open-terminal',
    label: 'Open terminal',
    run: () => showTerminal(a, layout, false),
  },
  {
    id: 'toggle-terminal',
    label: 'Toggle terminal',
    shortcut: { key: 'j' },
    run() {
      // The panel closes only when the Terminal surface is what it shows.
      if (layout.rightOpen && layout.panel.active === 'terminal') return layout.togglePanel('right')
      return showTerminal(a, layout, false)
    },
  },
  {
    id: 'new-terminal',
    label: 'New terminal',
    shortcut: NEW_TERMINAL_KEY,
    terminalShortcut: NEW_TERMINAL_IN_TERMINAL_KEY,
    run: () => showTerminal(a, layout, true),
  },
  {
    id: 'split-terminal',
    label: 'Split terminal side by side',
    shortcut: SPLIT_TERMINAL_KEY,
    run: () => splitTerminal(a, layout, 'row'),
  },
  {
    id: 'split-terminal-stacked',
    label: 'Split terminal stacked',
    terminalShortcut: SPLIT_STACKED_KEY,
    run: () => splitTerminal(a, layout, 'column'),
  },
  {
    id: 'close-terminal',
    label: 'Close terminal',
    terminalShortcut: CLOSE_TERMINAL_KEY,
    async run() {
      if (!a.project) throw 'The terminal is unavailable'
      const active = activeShell(termsOf(a.project.key, a.shells))
      if (!active) throw 'No terminal is open'
      await closeTerminal(a.shells, active)
    },
  },
]
