import type { Overlay } from './OrchestratorContext'
import type { Release } from './types'

// overlayAtStart is what the window shows once the start's release notes arrive: the notes, unless the owner has
// already opened something else, which they do not take over. The app sends notes only for a version not shown before.
export const overlayAtStart = (current: Overlay, notes: Release[]): Overlay => current ?? (notes.length > 0 ? 'changelog' : null)
