import type { ConfirmRequest } from './confirmQueue'
import type { UpdateState } from './types'

// filled says an update is in play: the button has its fill. dot is the ready dot on download; badge is the check
// badge on rotate-cw.
export type UpdateLook = {
  icon: 'refresh-cw' | 'download' | 'rotate-cw'
  spin: boolean
  filled: boolean
  dot: boolean
  badge: boolean
  ring: number | null
  success: boolean
  tooltip: string
}

// restart only relaunches into the update that is in place already; the install binding does it.
export type UpdateAction = 'check' | 'download' | 'install' | 'restart'

export function updateLook(s: UpdateState): UpdateLook {
  const plain = { spin: false, filled: false, dot: false, badge: false, ring: null, success: false }
  const filled = { ...plain, filled: true }
  switch (s.status) {
    case 'checking':
      return { ...plain, icon: 'refresh-cw', spin: true, tooltip: 'Checking for updates…' }
    case 'available':
      return { ...filled, icon: 'download', dot: true, success: true, tooltip: `Update ${s.version} ready to download` }
    case 'downloading':
      if (s.percent < 0) return { ...filled, icon: 'download', tooltip: 'Downloading update…' }
      return { ...filled, icon: 'download', ring: Math.floor(s.percent), tooltip: `Downloading update (${Math.floor(s.percent)}%)` }
    case 'downloaded':
      return { ...filled, icon: 'rotate-cw', badge: true, tooltip: `Update ${s.version} downloaded. Click to restart and install.` }
    case 'installed':
      return { ...filled, icon: 'rotate-cw', badge: true, tooltip: 'Restart to finish the update' }
    case 'failed':
      if (s.failedStep === 'install') return { ...plain, icon: 'rotate-cw', tooltip: `Install failed for ${s.version}. Click to retry.` }
      return { ...plain, icon: 'download', tooltip: `Download failed for ${s.version}. Click to retry.` }
    default:
      return { ...plain, icon: 'refresh-cw', tooltip: 'Check for updates' }
  }
}

// updateAction is what a click does; null while a check or a download runs.
export function updateAction(s: UpdateState): UpdateAction | null {
  switch (s.status) {
    case 'checking':
    case 'downloading':
      return null
    case 'available':
      return 'download'
    case 'downloaded':
      return 'install'
    case 'installed':
      return 'restart'
    case 'failed':
      return s.failedStep === 'install' ? 'install' : 'download'
    default:
      return 'check'
  }
}

export function installConfirm(version: string): Omit<ConfirmRequest, 'id'> {
  return { title: `Install update ${version} and restart the app?`, body: ['Running threads keep going: their hosts outlive the app.'], confirm: 'Restart', tone: 'primary' }
}
