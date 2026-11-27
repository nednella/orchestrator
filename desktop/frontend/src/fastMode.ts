import type { Command } from './chatTimeline'
import type { ChatEvent } from './types'

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

// What Claude's reasons for running without fast mode say to the owner; a reason with no line here is shown as it is.
const FAST_OFF_REASONS: Record<string, string> = {
  extra_usage_disabled: 'Extra usage is off for this account',
  free: 'Fast mode needs a paid plan',
  preference: 'Your organization turned fast mode off',
  model_not_allowed: "The model is not in your organization's allowed models",
  network_error: 'Claude could not check fast mode: the network failed',
  pending: 'Claude is still checking fast mode',
  unknown: 'Claude says fast mode is not available',
  // Claude gives this one until fast mode is asked for, so it says nothing of a failure.
  sdk_opt_in_required: '',
}

// fastModeNote is why Claude runs the thread at normal speed though fast mode is on, from the init of its latest run:
// '' when fast mode is off or Claude names no reason.
export function fastModeNote(events: ChatEvent[], fast: boolean): string {
  if (!fast) return ''
  const init = events.findLast((e) => e.kind === 'init')
  const reason = init?.text ?? ''
  if (reason === '') return ''
  const line = FAST_OFF_REASONS[reason] ?? reason.replaceAll('_', ' ')
  return line === '' ? '' : `${line}, so Claude runs at normal speed.`
}
