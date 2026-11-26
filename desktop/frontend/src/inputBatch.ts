// makeInputBatch sends a key at once, then holds what follows until schedule runs and sends it as one: a wheel turn
// alone makes a mouse report per line, and each send is a call to the Go side. It keeps holding while keys keep coming.
export function makeInputBatch(send: (data: string) => void, schedule: (run: () => void) => void): (data: string) => void {
  let holding = false
  let queued = ''
  const release = () => {
    holding = queued !== ''
    if (!holding) return
    send(queued)
    queued = ''
    schedule(release)
  }
  return (data) => {
    if (!data) return
    if (holding) {
      queued += data
      return
    }
    send(data)
    holding = true
    schedule(release)
  }
}
