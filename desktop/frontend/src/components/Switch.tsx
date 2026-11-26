type SwitchProps = { label: string; checked: boolean; disabled?: boolean; onChange(checked: boolean): void }

export function Switch({ label, checked, disabled = false, onChange }: SwitchProps) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={checked}
      aria-label={label}
      disabled={disabled}
      className="inline-flex h-[18px] w-[30px] flex-none items-center rounded-full p-0.5 transition-colors duration-150"
      style={{ background: checked ? 'var(--accent)' : 'var(--bg-switch-off)' }}
      onClick={() => onChange(!checked)}
    >
      <span
        aria-hidden="true"
        className="size-3.5 rounded-full shadow-sm transition-transform duration-150"
        style={{ background: checked ? 'var(--text-on-accent)' : 'var(--text-soft)', transform: checked ? 'translateX(12px)' : 'none' }}
      />
    </button>
  )
}
