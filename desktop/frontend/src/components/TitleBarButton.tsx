import type { LucideIcon } from 'lucide-react'

export const titleBarButton = 'btn btn-ghost h-[32px] w-[32px] flex-none justify-center px-0'

type TitleBarButtonProps = { label: string; icon: LucideIcon; pressed?: boolean; onClick(): void }

export function TitleBarButton({ label, icon: Icon, pressed, onClick }: TitleBarButtonProps) {
  return (
    <button
      className={titleBarButton}
      style={{ color: pressed ? 'var(--text)' : undefined }}
      title={label}
      aria-label={label}
      aria-pressed={pressed}
      onClick={onClick}
    >
      <Icon size={14} aria-hidden="true" />
    </button>
  )
}
