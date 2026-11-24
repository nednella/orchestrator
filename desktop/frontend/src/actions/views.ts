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
    uiCommand: 'stats',
    run: layout.toggleStats,
  },
  {
    id: 'show-terminal',
    label: 'Show terminal',
    group: 'Sessions',
    uiCommand: 'term',
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
    uiCommand: 'browser',
    async run(args) {
      if (!current) throw 'No session selected'
      layout.closeCentre()
      layout.showView(current.id, 'browser')
      if (args.length === 0) return
      await a.openBrowser(current.id, args.join(' '))
      return `Opened ${args.join(' ')}`
    },
  },
  {
    id: 'show-evidence',
    label: 'Show evidence',
    group: 'Sessions',
    uiCommand: 'evidence',
    run() {
      if (!current) throw 'No session selected'
      layout.closeCentre()
      layout.showView(current.id, 'evidence')
    },
  },
  {
    id: 'view-previous',
    label: 'Previous right panel tab (PR, evidence, browser)',
    group: 'Navigate',
    shortcut: { key: '[', code: 'BracketLeft', shift: true, label: '[' },
    palette: false,
    run() {
      layout.setRightOpen(true)
      a.cycleSessionView(-1)
    },
  },
  {
    id: 'view-next',
    label: 'Next right panel tab (PR, evidence, browser)',
    group: 'Navigate',
    shortcut: { key: ']', code: 'BracketRight', shift: true, label: ']' },
    palette: false,
    run() {
      layout.setRightOpen(true)
      a.cycleSessionView(1)
    },
  },
  {
    id: 'digest',
    label: 'News: weekly digest',
    group: 'App',
    uiCommand: 'digest',
    run: layout.toggleDigest,
  },
  {
    id: 'news',
    label: 'News',
    group: 'App',
    shortcut: { key: 'n', code: 'KeyN', alt: true },
    uiCommand: 'news',
    run: layout.toggleNews,
  },
  {
    id: 'patch-notes',
    label: 'Patch notes',
    group: 'App',
    run: () => a.setOverlay('changelog'),
  },
  {
    id: 'shortcuts',
    label: 'Keyboard shortcuts',
    group: 'App',
    shortcut: { key: '/' },
    run: () => a.setOverlay('shortcuts'),
  },
]
