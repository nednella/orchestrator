import { WifiOff } from 'lucide-react'
import { useNetworkStatus } from '../NetworkContext'
import { useDelayedTip } from '../useDelayedTip'
import { FloatingTip } from './FloatingTip'
import { titleBarButton } from './TitleBarButton'

// NetworkIndicator is in the top bar while the app cannot reach GitHub; a click asks again.
export function NetworkIndicator() {
  const { online, retrying, offlineSince, retry } = useNetworkStatus()
  const { anchor, tipHandlers } = useDelayedTip()
  if (online) return null
  const since = new Date(offlineSince).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
  const tip = `Can't reach GitHub since ${since}\nClick to retry`
  return (
    <button className={`${titleBarButton} text-danger`} aria-label="Can't reach GitHub. Retry" onClick={retry} {...tipHandlers}>
      <WifiOff size={14} strokeWidth={2.25} aria-hidden="true" className={retrying ? 'animate-spin' : undefined} />
      {anchor && <FloatingTip anchor={anchor} text={tip} below />}
    </button>
  )
}
