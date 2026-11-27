import { Pipette } from 'lucide-react'
import { useEffect, useRef } from 'react'
import type { Appearance } from '../themes'

// custom is the custom accent in use, '' while none is; start is where the color picker opens without one.
type AccentCustomSwatchProps = { mode: Appearance; custom: string; start: string; onPick(accent: string): void }

// AccentCustomSwatch opens the system's color picker through a color input laid over the disc, and shows the custom
// accent while one is in use.
export function AccentCustomSwatch({ mode, custom, start, onPick }: AccentCustomSwatchProps) {
  const input = useRef<HTMLInputElement>(null)
  const picked = custom !== ''

  // The picker sends an input event for each color it passes through and a change for the one it settles on; only
  // that one is saved.
  useEffect(() => {
    const el = input.current
    if (!el) return
    const save = () => onPick(el.value)
    el.addEventListener('change', save)
    return () => el.removeEventListener('change', save)
  }, [onPick])

  useEffect(() => {
    if (input.current) input.current.value = custom || start
  }, [custom, start])

  return (
    <span
      title="Custom"
      className={`relative flex size-8 shrink-0 transform-gpu items-center justify-center rounded-full p-1 transition-transform has-[input:focus-visible]:outline-2 has-[input:focus-visible]:outline-ink/60 has-[input:focus-visible]:outline-solid ${picked ? '' : 'hover:scale-110'}`}
    >
      <span
        aria-hidden="true"
        className={`flex size-full items-center justify-center rounded-full ${picked ? '' : 'bg-(--bg-raised) text-soft'}`}
        style={{ backgroundColor: picked ? custom : undefined, boxShadow: `var(--shadow-preview-${mode})` }}
      >
        {!picked && <Pipette size={12} />}
      </span>
      {picked && <span aria-hidden="true" className="pointer-events-none absolute inset-0 rounded-full ring-2 ring-(--accent) ring-inset" />}
      <input ref={input} type="color" aria-label={`Custom accent for ${mode} mode`} defaultValue={custom || start} className="absolute inset-0 size-full cursor-pointer rounded-full opacity-0" />
    </span>
  )
}
