import { FitAddon } from '@xterm/addon-fit'
import { Unicode11Addon } from '@xterm/addon-unicode11'
import { WebglAddon } from '@xterm/addon-webgl'
import { Terminal } from '@xterm/xterm'
import type { ITheme } from '@xterm/xterm'
import { api, errorMessage } from '../api'
import { makeInputBatch } from '../inputBatch'
import { TERMINAL_FONT, terminalThemeFrom } from '../terminalTheme'
import { keepsStream, servedOverHTTP } from '../termStream'

const FONT_PX = 12
const LINE_HEIGHT = 1.35
const FIRST_FIT_MS = 30
const RESIZE_SETTLE_MS = 120
const INPUT_BATCH_MS = 16
const DOUBLE_ESC_MS = 500

const SHIFT_ENTER = '\x1b[13;2u'
const LINE_START = '\x01'
const LINE_END = '\x05'
// ⌘ with these keys stays the terminal's: copy, paste and the system's own. Every other ⌘ key is the app's.
const SYSTEM_KEYS = new Set(['c', 'v', 'x', 'm', 'h', 'w', 'q'])

const DEVICE_REPORT = /^(\x1b\[[?>]?[\d;]*c|\x1bP[^\x1b]*\x1b\\|\x1b\]\d+;[^\x07\x1b]*(\x07|\x1b\\))+$/

const inBrowser = servedOverHTTP(location.protocol)
const reducedMotion = matchMedia('(prefers-reduced-motion: reduce)')

// A remount of one id must not let the old session's close land after the new session's open.
const generations = new Map<string, number>()

// Every live terminal by shell id. A session outlives its pane, so showing it again needs no attach.
export const sessions = new Map<string, TermSession>()

type Look = { theme: ITheme; fontSize: number; blink: boolean; key: string }

function readLook(): Look {
  const root = document.documentElement
  const scale = parseFloat(getComputedStyle(root).getPropertyValue('--ui-scale')) || 1
  const theme = terminalThemeFrom(root)
  const fontSize = Math.round(FONT_PX * scale * 2) / 2
  return { theme, fontSize, blink: !reducedMotion.matches, key: JSON.stringify(theme) }
}

let look: Look | null = null
let lookWatch: MutationObserver | null = null

const relook = () => {
  look = readLook()
  sessions.forEach((s) => s.restyle())
}

// The terminals follow the app's theme and text size live, both set on the root element, and the system's reduced
// motion.
function currentLook(): Look {
  if (!lookWatch) {
    lookWatch = new MutationObserver(relook)
    lookWatch.observe(document.documentElement, { attributes: true, attributeFilter: ['data-theme', 'style'] })
    reducedMotion.addEventListener('change', relook)
  }
  look ??= readLook()
  return look
}

function stopWatchingLook() {
  lookWatch?.disconnect()
  lookWatch = null
  reducedMotion.removeEventListener('change', relook)
  look = null
}

let webglBroken = false

export class TermSession {
  // onLeave runs on a double Esc, which hands the keys back to the chat.
  onLeave = () => {}
  private readonly term: Terminal
  private readonly fitter = new FitAddon()
  private readonly input: ReturnType<typeof makeInputBatch>
  private readonly generation: number
  private host: HTMLElement | null = null
  private observer: ResizeObserver | null = null
  private webgl: WebglAddon | null = null
  private state: 'new' | 'opening' | 'open' | 'gone' = 'new'
  private sent = { cols: 0, rows: 0 }
  private styled = ''
  private firstFit = 0
  private fitFrame = 0
  private settle = 0
  private stopStream: (() => void) | null = null
  private focusOnMount = false
  private lastEsc = 0
  private partialLines = 0

  constructor(readonly id: string) {
    this.generation = (generations.get(id) ?? 0) + 1
    generations.set(id, this.generation)
    const { theme, fontSize, blink, key } = currentLook()
    this.styled = key
    this.term = new Terminal({
      fontFamily: TERMINAL_FONT,
      fontSize,
      lineHeight: LINE_HEIGHT,
      // xterm blinks the cursor only while the terminal has the focus.
      cursorBlink: blink,
      // tmux holds the history. A mount onto a running stream never sees tmux's switch to the alternate screen, so
      // any scrollback here fills with lines tmux pushes off the bottom and xterm offers a slider over them.
      scrollback: 0,
      allowProposedApi: true,
      theme,
    })
    this.term.loadAddon(this.fitter)
    this.term.loadAddon(new Unicode11Addon())
    this.term.unicode.activeVersion = '11'

    // A timer, not an animation frame: WebKit holds frames back while the window is hidden.
    this.input = makeInputBatch(
      (data) => void api.termWrite(id, data),
      (run) => {
        const timer = window.setTimeout(run, INPUT_BATCH_MS)
        return () => window.clearTimeout(timer)
      },
    )
    this.term.attachCustomKeyEventHandler((event) => this.key(event))
    this.term.onData((data) => {
      // xterm answers tmux's "what terminal are you" queries (device attributes, version, colours). tmux has stopped
      // waiting by the time the answer crosses the bridge, so it would land in the shell as typed text.
      if (DEVICE_REPORT.test(data)) return
      this.input.write(data)
    })
    this.term.onBinary(this.input.write)
  }

  mount(el: HTMLElement) {
    this.host = el
    if (this.term.element) el.appendChild(this.term.element)
    else this.term.open(el)
    el.addEventListener('wheel', this.wheel, { capture: true, passive: false })
    this.observer = new ResizeObserver(() => this.requestFit())
    this.observer.observe(el)
    this.firstFit = window.setTimeout(() => {
      this.firstFit = 0
      this.requestFit()
    }, FIRST_FIT_MS)
    this.syncStream()
    // Kept for a tick: a mount that React undoes and makes again at once (StrictMode) takes the focus too.
    if (this.focusOnMount) {
      this.term.focus()
      window.setTimeout(() => (this.focusOnMount = false))
    }
  }

  unmount() {
    const el = this.host
    if (!el) return
    this.host = null
    this.observer?.disconnect()
    this.observer = null
    window.clearTimeout(this.firstFit)
    this.firstFit = 0
    cancelAnimationFrame(this.fitFrame)
    this.fitFrame = 0
    el.removeEventListener('wheel', this.wheel, { capture: true })
    this.dropWebgl()
    this.term.element?.remove()
    this.syncStream()
  }

  // An open still on its way closes when it lands, in open; closing now could reach Go before it.
  dispose() {
    this.unmount()
    const attached = this.state === 'open'
    this.state = 'gone'
    this.stopStream?.()
    this.stopStream = null
    this.input.dispose()
    window.clearTimeout(this.settle)
    if (attached) this.closeLater()
    this.term.dispose()
    sessions.delete(this.id)
    if (sessions.size === 0) stopWatchingLook()
  }

  focus() {
    if (this.host) this.term.focus()
    else this.focusOnMount = true
  }

  restyle() {
    const { theme, fontSize, blink, key } = currentLook()
    if (this.term.options.cursorBlink !== blink) this.term.options.cursorBlink = blink
    if (key !== this.styled) {
      this.styled = key
      this.term.options.theme = theme
    }
    if (this.term.options.fontSize !== fontSize) {
      this.term.options.fontSize = fontSize
      this.requestFit()
    }
  }

  // Deferred a tick: a new session of this id bumps the generation and keeps the attach open.
  private closeLater() {
    window.setTimeout(() => {
      if (generations.get(this.id) !== this.generation) return
      generations.delete(this.id)
      void api.termClose(this.id).catch(() => {})
    })
  }

  // A fit at most once a frame, and none before the first, which waits for the layout to settle.
  private requestFit() {
    if (this.firstFit || this.fitFrame || !this.host) return
    this.fitFrame = requestAnimationFrame(() => {
      this.fitFrame = 0
      this.fit()
    })
  }

  // The grid refits on every size change; the shell hears only when the cell count moves, and only once a drag
  // settles: each resize makes the shell redraw its prompt, and a redraw per step leaves stale prompt fragments.
  private fit() {
    const el = this.host
    if (!el) return
    if (el.clientWidth === 0 || el.clientHeight === 0) return this.dropWebgl()
    this.useWebgl()
    this.fitter.fit()
    const { cols, rows } = this.term
    if (this.state === 'new') return this.open(cols, rows)
    window.clearTimeout(this.settle)
    if (this.state !== 'open' || (cols === this.sent.cols && rows === this.sent.rows)) return
    this.settle = window.setTimeout(() => {
      this.sent = { cols, rows }
      void api.termResize(this.id, cols, rows)
    }, RESIZE_SETTLE_MS)
  }

  private open(cols: number, rows: number) {
    this.state = 'opening'
    this.sent = { cols, rows }
    api.termOpen(this.id, cols, rows).then(
      () => {
        if (this.state === 'gone') return this.closeLater()
        this.state = 'open'
        this.syncStream()
        this.requestFit()
      },
      (err) => {
        if (this.state !== 'gone') this.term.write(`\x1b[2m${errorMessage(err)}\x1b[0m\r\n`)
      },
    )
  }

  private syncStream() {
    const wanted = this.state === 'open' && keepsStream(inBrowser, this.host !== null)
    if (wanted && !this.stopStream) {
      this.stopStream = api.termStream(
        this.id,
        (bytes) => this.term.write(bytes),
        () => {
          this.stopStream = null
          this.term.write('\r\n\x1b[2m[terminal] Process exited\x1b[0m\r\n')
        },
      )
    }
    if (!wanted && this.stopStream) {
      this.stopStream()
      this.stopStream = null
    }
  }

  private useWebgl() {
    if (this.webgl || webglBroken) return
    try {
      const addon = new WebglAddon()
      addon.onContextLoss(() => {
        addon.dispose()
        if (this.webgl === addon) this.webgl = null
      })
      this.term.loadAddon(addon)
      this.webgl = addon
    } catch {
      // the default renderer keeps working without WebGL
      webglBroken = true
    }
  }

  private dropWebgl() {
    this.webgl?.dispose()
    this.webgl = null
  }

  private key(event: KeyboardEvent): boolean {
    if (event.key === 'Escape' && event.type === 'keydown') {
      const now = performance.now()
      if (now - this.lastEsc < DOUBLE_ESC_MS) {
        this.lastEsc = 0
        this.onLeave()
        return false
      }
      this.lastEsc = now
    }
    const key = event.key.toLowerCase()
    if (event.metaKey && key === 'c' && this.term.hasSelection()) {
      void navigator.clipboard.writeText(this.term.getSelection())
      return false
    }
    if (event.metaKey && (event.key === 'ArrowLeft' || event.key === 'ArrowRight')) {
      if (event.type === 'keydown') {
        event.preventDefault()
        this.input.write(event.key === 'ArrowLeft' ? LINE_START : LINE_END)
      }
      return false
    }
    if (event.metaKey && !SYSTEM_KEYS.has(key)) return false
    if (event.key === 'Enter' && event.shiftKey) {
      if (event.type === 'keydown') {
        event.preventDefault()
        this.input.write(SHIFT_ENTER)
      }
      return false
    }
    return true
  }

  // xterm forwards at most one wheel report per DOM event to tmux, and damps small deltas, so a trackpad or mouse
  // scrolls far slower than in a native terminal. Turn the travel into whole lines here; xterm forwards each
  // line-mode event as one report, as Ghostty does per cell.
  private wheel = (event: WheelEvent) => {
    if (event.deltaMode !== WheelEvent.DOM_DELTA_PIXEL) return
    event.preventDefault()
    event.stopPropagation()
    const screen = this.term.element?.querySelector('.xterm-screen')
    if (!screen) return
    this.partialLines += event.deltaY / (screen.clientHeight / this.term.rows)
    const lines = Math.trunc(this.partialLines)
    this.partialLines -= lines
    for (let i = 0; i < Math.abs(lines); i++) {
      screen.dispatchEvent(
        new WheelEvent('wheel', { deltaY: Math.sign(lines), deltaMode: WheelEvent.DOM_DELTA_LINE, clientX: event.clientX, clientY: event.clientY, bubbles: true, cancelable: true }),
      )
    }
  }
}

// termSession is the shell's terminal, made the first time it is asked for.
export function termSession(id: string): TermSession {
  let session = sessions.get(id)
  if (!session) {
    session = new TermSession(id)
    sessions.set(id, session)
  }
  return session
}

// keepSessions disposes every terminal whose shell is not in keep; the shells themselves run on in tmux.
export function keepSessions(keep: Set<string>) {
  for (const [id, session] of sessions) if (!keep.has(id)) session.dispose()
}
