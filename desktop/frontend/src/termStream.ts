// openTermStream reads the shell's output from GET /term/<id> as it comes, until the shell exits or its terminal
// closes, then calls onEnd. The function it returns stops reading, with no onEnd.
export function openTermStream(id: string, onData: (bytes: Uint8Array) => void, onEnd: () => void): () => void {
  const stop = new AbortController()
  const read = async () => {
    const res = await fetch('/term/' + encodeURIComponent(id), { headers: { 'X-Agentos': '1' }, signal: stop.signal })
    const body = res.ok ? res.body?.getReader() : undefined
    if (!body) return
    for (let chunk = await body.read(); !chunk.done && !stop.signal.aborted; chunk = await body.read()) onData(chunk.value)
  }
  read()
    .catch(() => {})
    .finally(() => {
      if (!stop.signal.aborted) onEnd()
    })
  return () => stop.abort()
}
