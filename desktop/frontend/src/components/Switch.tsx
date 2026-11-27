type SwitchProps = { label: string; checked: boolean; disabled?: boolean; onChange(checked: boolean): void }

export function Switch({ label, checked, disabled = false, onChange }: SwitchProps) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={checked}
      aria-label={label}
      disabled={disabled}
      className="group inline-flex h-[18px] w-[30px] flex-none items-center rounded-full p-0.5 transition-colors duration-200 outline-none focus-visible:ring-2 focus-visible:ring-(--accent) focus-visible:ring-offset-1 focus-visible:ring-offset-(--bg-card) disabled:opacity-64"
      style={{ background: checked ? 'var(--accent)' : 'var(--bg-switch-off)' }}
      onClick={() => onChange(!checked)}
    >
      <span
        aria-hidden="true"
        data-checked={checked}
        className="size-3.5 rounded-full shadow-sm transition-[translate,scale] duration-150 group-active:scale-x-110 data-[checked=true]:translate-x-3"
        style={{ background: checked ? 'var(--text-on-accent)' : 'var(--text-soft)' }}
      />
    </button>
  )
}
