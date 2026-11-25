import { useState } from 'react'
import { useAgentos } from './AgentosContext'

// useAnswering sends an answer to an approval and holds busy until the host takes or refuses it, so a second click
// cannot send another; a failure shows as a toast.
export function useAnswering(): { busy: boolean; send(answer: () => Promise<void>): void } {
  const { report } = useAgentos('report')
  const [busy, setBusy] = useState(false)
  return {
    busy,
    send(answer) {
      setBusy(true)
      report(() => Promise.resolve().then(answer).finally(() => setBusy(false)))
    },
  }
}
