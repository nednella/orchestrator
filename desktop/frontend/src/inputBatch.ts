// makeInputBatch sends a key at once, then holds what follows until schedule runs and sends it as one: a wheel turn
// alone makes a mouse report per line, and each send is a call to the Go side. It keeps holding while keys keep coming.
// schedule returns a function that cancels the run; dispose cancels it and drops the held keys, and later writes too.
export function makeInputBatch(
  send: (data: string) => void,
  schedule: (run: () => void) => () => void,
): { write: (data: string) => void; dispose: () => void } {
  let holding = false
  let disposed = false
  let queued = ''
  let cancel = () => {}
  const hold = () => {
    holding = true
    cancel = schedule(release)
  }
  const release = () => {
    holding = false
    if (!queued) return
    send(queued)
    queued = ''
    hold()
  }
  return {
    write(data) {
      if (!data || disposed) return
      if (holding) {
        queued += data
        return
      }
      send(data)
      hold()
    },
    dispose() {
      disposed = true
      cancel()
      queued = ''
    },
  }
}
