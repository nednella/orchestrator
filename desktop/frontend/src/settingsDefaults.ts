import type { Access, Place } from './types'

// What a setting is until it is set: a row shows its reset button while the value differs from this.
export const SETTING_DEFAULTS: { projectsDir: string; keepAwake: boolean; threadPlace: Place; threadAccess: Access; browserEnabled: boolean } = {
  projectsDir: '',
  keepAwake: true,
  threadPlace: 'worktree',
  threadAccess: 'supervised',
  browserEnabled: true,
}
