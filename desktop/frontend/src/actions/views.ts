import { toggleContextCard } from '../contextCardOpen'
import { toggleUsage } from '../usageOpen'
import type { Action, ActionContext } from './types'

export const viewActions = ({ a, layout, current }: ActionContext): Action[] => [
  {
    id: 'zoom-in',
    label: 'Larger text',
    shortcut: { key: '=', aliases: ['+'], anyShift: true, label: '+' },
    run: () => layout.zoom(1),
  },
  {
    id: 'zoom-out',
    label: 'Smaller text',
    shortcut: { key: '-', aliases: ['_'], anyShift: true },
    run: () => layout.zoom(-1),
  },
  {
    id: 'zoom-reset',
    label: 'Reset text size',
    shortcut: { key: '0' },
    run: () => layout.zoom(0),
  },
  {
    id: 'settings',
    label: 'Settings',
    shortcut: { key: ',' },
    run: () => a.setOverlay('settings'),
  },
  {
    id: 'usage',
    label: 'Usage',
    shortcut: { key: 'u' },
    run: toggleUsage,
  },
  {
    id: 'toggle-context-card',
    label: 'Toggle thread details',
    shortcut: { key: 'b', shift: true },
    run() {
      if (!current) throw 'Select a thread first'
      toggleContextCard()
    },
  },
  {
    id: 'view-previous',
    label: 'Previous right panel tab',
    shortcut: { key: '[', code: 'BracketLeft', shift: true, label: '[' },
    run() {
      if (layout.panel.tabs.length === 0) throw 'No view is open in the right panel'
      layout.stepPanelTab(-1)
    },
  },
  {
    id: 'view-next',
    label: 'Next right panel tab',
    shortcut: { key: ']', code: 'BracketRight', shift: true, label: ']' },
    run() {
      if (layout.panel.tabs.length === 0) throw 'No view is open in the right panel'
      layout.stepPanelTab(1)
    },
  },
  {
    id: 'digest',
    label: 'News',
    run: () => layout.showPanelView('news'),
  },
  {
    id: 'shortcuts',
    label: 'Keyboard shortcuts',
    shortcut: { key: '/' },
    run: () => a.setOverlay('shortcuts'),
  },
]
