import { useEffect, useRef } from 'react'

type InlineInputProps = {
  initial?: string
  placeholder?: string
  blur: 'submit' | 'cancel'
  className?: string
  action?: string
  // accent false gives the action a plain button, for an action that is not the one to take by default.
  accent?: boolean
  onSubmit(value: string): void
  onCancel(): void
}

export function InlineInput({ initial = '', placeholder, blur, className = '', action, accent = true, onSubmit, onCancel }: InlineInputProps) {
  const input = useRef<HTMLInputElement>(null)
  const done = useRef(false)

  useEffect(() => {
    input.current?.focus()
    input.current?.select()
  }, [])

  const finish = (submit: boolean) => {
    if (done.current) return
    done.current = true
    if (submit) onSubmit(input.current?.value ?? '')
    else onCancel()
  }

  const field = (
    <input
      ref={input}
      defaultValue={initial}
      placeholder={placeholder}
      spellCheck={false}
      className={action ? 'h-8 flex-1 px-1 text-body' : `field ${className}`}
      onKeyDown={(e) => {
        if (e.key === 'Enter') finish(true)
        if (e.key === 'Escape') finish(false)
      }}
      onBlur={() => finish(blur === 'submit')}
    />
  )
  if (!action) return field

  return (
    <div className={`composer flex-row items-center gap-1.5 pr-1 pl-2.5 ${className}`}>
      {field}
      <button className={`btn h-6 flex-none px-2 text-label ${accent ? 'btn-accent' : ''}`} onMouseDown={(e) => e.preventDefault()} onClick={() => finish(true)}>
        {action}
      </button>
    </div>
  )
}
