// keepFocusIn wraps Tab and Shift+Tab around the dialog's controls, so the focus never leaves it.
export function keepFocusIn(dialog: HTMLElement | null, e: KeyboardEvent) {
  const controls = [...(dialog?.querySelectorAll<HTMLElement>('button:not(:disabled), input:not(:disabled)') ?? [])]
  if (controls.length === 0) return
  const at = controls.indexOf(document.activeElement as HTMLElement)
  const next = e.shiftKey ? (at <= 0 ? controls.length - 1 : at - 1) : (at === -1 || at === controls.length - 1 ? 0 : at + 1)
  e.preventDefault()
  controls[next].focus()
}
