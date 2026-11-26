import { toggleContextCard } from '../contextCardOpen'
import { toggleUsage } from '../usageOpen'
import type { Action, ActionContext } from './types'

export const viewActions = ({ a, layout, current }: ActionContext): Action[] => [
  {
    id: 'zoom-in',
    label: 'Larger text',
    group: 'App',
    shortcut: { key: '=', aliases: ['+'], anyShift: true, label: '+' },
    run: () => layout.zoom(1),
  },
  {
    id: 'zoom-out',
    label: 'Smaller text',
    group: 'App',
    shortcut: { key: '-', aliases: ['_'], anyShift: true },
    run: () => layout.zoom(-1),
  },
  {
    id: 'zoom-reset',
    label: 'Reset text size',
    group: 'App',
    shortcut: { key: '0' },
    run: () => layout.zoom(0),
  },
  {
    id: 'settings',
    label: 'Settings',
    group: 'App',
    shortcut: { key: ',' },
    run: () => a.setOverlay('settings'),
  },
  {
    id: 'stats',
    label: 'Stats: what interrupts you',
    group: 'App',
    shortcut: { key: 's', code: 'KeyS', alt: true },
    run: layout.toggleStats,
  },
  {
    id: 'usage',
    label: 'Usage',
    group: 'App',
    shortcut: { key: 'u' },
    run: toggleUsage,
  },
  {
    id: 'toggle-context-card',
    label: 'Toggle thread details',
    group: 'Navigate',
    shortcut: { key: 'b', shift: true },
    run() {
      if (!current) throw 'Select a thread first'
      toggleContextCard()
    },
  },
  {
    id: 'show-thread',
    label: 'Show thread',
    group: 'Sessions',
    run() {
      if (!current) throw 'No session selected'
      layout.closeCentre()
      layout.focusPanel('centre')
    },
  },
  {
    id: 'show-browser',
    label: 'Show browser',
    group: 'Sessions',
    run() {
      if (!current) throw 'No session selected'
      layout.closeCentre()
      layout.showPanelView('browser')
    },
  },
  {
    id: 'show-evidence',
    label: 'Show evidence',
    group: 'Sessions',
    run() {
      if (!current) throw 'No session selected'
      layout.closeCentre()
      layout.showPanelView('evidence')
    },
  },
  {
    id: 'view-previous',
    label: 'Previous right panel tab',
    group: 'Navigate',
    shortcut: { key: '[', code: 'BracketLeft', shift: true, label: '[' },
    run() {
      if (layout.panel.tabs.length === 0) throw 'No view is open in the right panel'
      layout.stepPanelTab(-1)
    },
  },
  {
    id: 'view-next',
    label: 'Next right panel tab',
    group: 'Navigate',
    shortcut: { key: ']', code: 'BracketRight', shift: true, label: ']' },
    run() {
      if (layout.panel.tabs.length === 0) throw 'No view is open in the right panel'
      layout.stepPanelTab(1)
    },
  },
  {
    id: 'digest',
    label: 'News',
    group: 'App',
    run: layout.toggleDigest,
  },
  {
    id: 'shortcuts',
    label: 'Keyboard shortcuts',
    group: 'App',
    shortcut: { key: '/' },
    run: () => a.setOverlay('shortcuts'),
  },
]
