import type { ConfirmRequest } from './confirmQueue'
import type { UpdateState } from './types'

// dot puts a mark at the icon's corner: the ready dot on download, the check badge on rotate-cw.
export type UpdateLook = {
  icon: 'refresh-cw' | 'download' | 'rotate-cw'
  spin: boolean
  dot: boolean
  ring: number | null
  success: boolean
  tooltip: string
}

export type UpdateAction = 'check' | 'download' | 'install'

const installFailed = (s: UpdateState) => s.status === 'failed' && s.percent >= 100

export function updateLook(s: UpdateState): UpdateLook {
  const plain = { spin: false, dot: false, ring: null, success: false }
  switch (s.status) {
    case 'checking':
      return { ...plain, icon: 'refresh-cw', spin: true, tooltip: 'Checking for updates…' }
    case 'available':
      return { ...plain, icon: 'download', dot: true, success: true, tooltip: `Update ${s.version} ready to download` }
    case 'downloading':
      if (s.percent < 0) return { ...plain, icon: 'download', tooltip: 'Downloading update…' }
      return { ...plain, icon: 'download', ring: Math.floor(s.percent), tooltip: `Downloading update (${Math.floor(s.percent)}%)` }
    case 'downloaded':
      return { ...plain, icon: 'rotate-cw', dot: true, tooltip: `Update ${s.version} downloaded. Click to restart and install.` }
    case 'failed':
      if (installFailed(s)) return { ...plain, icon: 'rotate-cw', tooltip: `Install failed for ${s.version}. Click to retry.` }
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
    case 'failed':
      return installFailed(s) ? 'install' : 'download'
    default:
      return 'check'
  }
}

export function installConfirm(version: string): Omit<ConfirmRequest, 'id'> {
  return { title: `Install update ${version} and restart the app?`, body: ['Running threads keep going: their hosts outlive the app.'], confirm: 'Restart', tone: 'primary' }
}
