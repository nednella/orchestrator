import type { IconName } from './components/Icon'

export type ToolSummary = { icon: IconName; title: string; detail: string }
export type DiffLine = { sign: '+' | '−'; text: string }

const resultLimit = 20
const detailLimit = 120

type Input = Record<string, unknown>

const asInput = (input: unknown): Input => (input && typeof input === 'object' ? (input as Input) : {})
const str = (v: unknown): string => (typeof v === 'string' ? v : '')
const lines = (s: string): string[] => (s === '' ? [] : s.split('\n'))
const firstLine = (s: string): string => s.split('\n').find((l) => l.trim() !== '')?.trim() ?? ''

function relative(path: string, folder: string): string {
  const base = folder.replace(/\/+$/, '')
  if (base === '' || !path.startsWith(`${base}/`)) return path
  return path.slice(base.length + 1)
}

function edits(tool: string, input: Input): { old: string; next: string }[] {
  if (tool === 'Write') return [{ old: '', next: str(input.content) }]
  if (tool === 'MultiEdit') return (Array.isArray(input.edits) ? input.edits : []).map((e) => ({ old: str(asInput(e).old_string), next: str(asInput(e).new_string) }))
  return [{ old: str(input.old_string), next: str(input.new_string) }]
}

function clip(s: string, max: number): string {
  return s.length <= max ? s : `${s.slice(0, max - 1)}…`
}

// toolSummary is the one line a tool call shows: its icon, what it is and what it was for.
export function toolSummary(tool: string, input: unknown, folder: string): ToolSummary {
  const i = asInput(input)
  switch (tool) {
    case 'Bash':
      return { icon: 'terminal', title: str(i.description) || 'Run', detail: firstLine(str(i.command)) }
    case 'Read':
      return { icon: 'file', title: 'Read', detail: relative(str(i.file_path), folder) }
    case 'Edit':
    case 'MultiEdit':
    case 'Write': {
      const all = edits(tool, i)
      const added = all.reduce((n, e) => n + lines(e.next).length, 0)
      const removed = all.reduce((n, e) => n + lines(e.old).length, 0)
      return { icon: 'compose', title: tool, detail: `${relative(str(i.file_path), folder)} +${added} −${removed}` }
    }
    case 'Grep':
    case 'Glob':
      return { icon: 'search', title: tool, detail: str(i.pattern) }
    case 'WebFetch':
      return { icon: 'external', title: tool, detail: str(i.url) }
    case 'WebSearch':
      return { icon: 'search', title: tool, detail: str(i.query) }
    case 'Task':
    case 'Agent':
      return { icon: 'agent', title: tool, detail: str(i.description) }
    case 'TodoWrite': {
      const n = Array.isArray(i.todos) ? i.todos.length : 0
      return { icon: 'check', title: 'Todos', detail: `${n} ${n === 1 ? 'task' : 'tasks'}` }
    }
  }
  const mcp = /^mcp__(.+?)__(.+)$/.exec(tool)
  if (mcp) return { icon: 'model', title: `${mcp[1]} · ${mcp[2]}`, detail: '' }
  return { icon: 'more', title: tool, detail: clip(JSON.stringify(input ?? {}), detailLimit) }
}

// toolBody is what an opened call shows of its input: the lines an edit changes, or else the text.
export function toolBody(tool: string, input: unknown): { diff: DiffLine[] } | { text: string } {
  const i = asInput(input)
  if (tool === 'Edit' || tool === 'MultiEdit' || tool === 'Write') {
    return { diff: edits(tool, i).flatMap((e) => [...lines(e.old).map((text) => ({ sign: '−' as const, text })), ...lines(e.next).map((text) => ({ sign: '+' as const, text }))]) }
  }
  if (tool === 'Bash' && typeof i.command === 'string') return { text: i.command }
  return { text: JSON.stringify(input ?? {}, null, 2) }
}

// resultLines clips a result to its first lines, and says how many it has.
export function resultLines(text: string, all: boolean): { shown: string; total: number } {
  const rows = text.split('\n')
  return { shown: all ? text : rows.slice(0, resultLimit).join('\n'), total: rows.length }
}

export function formatWorked(ms: number): string {
  const seconds = Math.max(0, Math.floor(ms / 1000))
  if (seconds < 60) return `${seconds}s`
  const minutes = Math.floor(seconds / 60)
  if (minutes < 60) return seconds % 60 ? `${minutes}m ${seconds % 60}s` : `${minutes}m`
  const hours = Math.floor(minutes / 60)
  return minutes % 60 ? `${hours}h ${minutes % 60}m` : `${hours}h`
}
