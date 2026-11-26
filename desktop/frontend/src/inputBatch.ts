// makeInputBatch joins the input of one animation frame into one send: a wheel turn alone makes a mouse report per
// line, and each send is a call to the Go side.
export function makeInputBatch(send: (data: string) => void, schedule: (run: () => void) => void): (data: string) => void {
  let queued = ''
  return (data) => {
    if (!data) return
    if (!queued) {
      schedule(() => {
        const batch = queued
        queued = ''
        send(batch)
      })
    }
    queued += data
  }
}
