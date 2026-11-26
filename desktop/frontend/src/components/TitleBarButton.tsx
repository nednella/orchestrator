import { Icon } from './Icon'
import type { IconName } from './Icon'

export const titleBarButton = 'btn btn-ghost h-[32px] w-[32px] flex-none justify-center px-0'

type TitleBarButtonProps = { label: string; icon: IconName; pressed?: boolean; disabled?: boolean; onClick(): void }

export function TitleBarButton({ label, icon, pressed, disabled, onClick }: TitleBarButtonProps) {
  return (
    <button
      className={titleBarButton}
      style={{ color: pressed ? 'var(--text)' : undefined }}
      title={label}
      aria-label={label}
      aria-pressed={pressed}
      disabled={disabled}
      onClick={onClick}
    >
      <Icon name={icon} fixed />
    </button>
  )
}
