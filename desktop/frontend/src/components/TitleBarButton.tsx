import { Icon } from './Icon'
import type { IconName } from './Icon'

export const titleBarButton = 'btn btn-ghost h-[32px] w-[32px] flex-none justify-center px-0'

type TitleBarButtonProps = { label: string; icon: IconName; pressed?: boolean; disabled?: boolean; unseen?: number; onClick(): void }

export function TitleBarButton({ label, icon, pressed, disabled, unseen = 0, onClick }: TitleBarButtonProps) {
  const name = unseen > 0 ? `${label}, ${unseen} new` : label
  return (
    <button
      className={`${titleBarButton} relative`}
      style={{ color: pressed ? 'var(--text)' : undefined }}
      title={name}
      aria-label={name}
      aria-pressed={pressed}
      disabled={disabled}
      onClick={onClick}
    >
      <Icon name={icon} />
      {unseen > 0 && <span className="mono absolute top-[3px] right-[2px] text-[10px] leading-none text-accent">{unseen > 9 ? '9+' : unseen}</span>}
    </button>
  )
}
