import type { Command } from './chatTimeline'

// The slash command that turns fast mode on or off from a thread's composer, as the slash menu lists it. The app sets
// it and not Claude, which refuses to turn fast mode on for a host like this one.
export function fastCommands(): Command[] {
  return [{ name: 'fast', description: 'Turn fast mode on or off', argumentHint: '[on|off]', builtin: true }]
}

// fastCommand reads a message that sets fast mode, /fast with an optional on or off: whether fast mode is on after it,
// given whether it is now, or null for a message that is no such command. /fast alone changes it.
export function fastCommand(text: string, now: boolean): boolean | null {
  const match = /^\/fast(?:\s+(on|off))?$/i.exec(text.trim())
  if (!match) return null
  return match[1] === undefined ? !now : match[1].toLowerCase() === 'on'
}
