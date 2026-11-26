// A browser opens at most 6 connections to the page's server and /__events holds one, so there a terminal reads its
// stream only while it is on screen, and the attach repaints it when it shows again; with 4 panes at most, one
// connection stays free for /__call/. The window has no such limit, and a kept terminal goes on reading.
export function keepsStream(browser: boolean, onScreen: boolean): boolean {
  return !browser || onScreen
}

// servedOverHTTP says whether the page is in a browser, from location.protocol: the window serves it as wails:.
export function servedOverHTTP(protocol: string): boolean {
  return protocol === 'http:' || protocol === 'https:'
}

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
