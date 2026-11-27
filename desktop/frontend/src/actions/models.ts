import { CLAUDE_MODELS } from '../model.ts'
import { jumpToModel } from '../modelJump.ts'
import type { Action } from './types'

// One action for each model the menu can list; each does nothing while no model menu is open.
export const modelActions = (): Action[] =>
  Object.keys(CLAUDE_MODELS).map((_, i) => ({
    id: `pick-model-${i + 1}`,
    label: `Pick model ${i + 1} in the open model menu`,
    shortcut: { key: String(i + 1) },
    run: () => jumpToModel(i + 1),
  }))
