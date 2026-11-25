import { useState } from 'react'
import { useAgentos } from './AgentosContext'

// useAnswering sends an answer to an approval. busy holds from the send on, so a second click cannot send another; it
// ends only when the answer fails, shown as a toast, since a taken answer replaces what asked for it.
export function useAnswering(): { busy: boolean; send(answer: () => Promise<void>): void } {
  const { report } = useAgentos('report')
  const [busy, setBusy] = useState(false)
  return {
    busy,
    send(answer) {
      setBusy(true)
      report(() =>
        Promise.resolve()
          .then(answer)
          .catch((err) => {
            setBusy(false)
            throw err
          }),
      )
    },
  }
}
