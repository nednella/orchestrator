import { createContext, useContext } from 'react'

export type NetworkStatus = {
  online: boolean
  retrying: boolean
  // When the app first saw its calls fail.
  offlineSince: number
  // retry asks again at once, and shows the toast again if the owner closed it.
  retry(): void
}

export const NetworkContext = createContext<NetworkStatus>({ online: true, retrying: false, offlineSince: 0, retry() {} })

export const useNetworkStatus = () => useContext(NetworkContext)
