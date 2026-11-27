import { useState } from 'react'
import { cardTip, type Appearance, type ThemeCardDef } from '../themes'
import { useDelayedTip } from '../useDelayedTip'
import { FloatingTip } from './FloatingTip'
import { ThemeModeButton } from './ThemeModeButton'

type ThemeCardProps = { card: ThemeCardDef; picked: Appearance[]; onUse(mode?: Appearance): void }

// ThemeCard is one theme of the library: a disc picks it for one appearance, the card for every appearance it has.
export function ThemeCard({ card, picked, onUse }: ThemeCardProps) {
  const { anchor, tipHandlers } = useDelayedTip()
  const [overDisc, setOverDisc] = useState(false)
  return (
    <div className="cursor-pointer overflow-hidden rounded-xl border border-(--border-rule) bg-(--bg-card) transition-colors hover:bg-(--bg-row-hover)" onClick={() => onUse()} {...tipHandlers}>
      <div className="flex min-h-16 items-center justify-center gap-2.5 px-3 pt-3">
        {card.previews.map((preview) => (
          <ThemeModeButton key={preview.mode} label={card.label} preview={preview} picked={picked.includes(preview.mode)} onPick={() => onUse(preview.mode)} onHover={setOverDisc} />
        ))}
      </div>
      <div className="flex items-center gap-2 px-3 pt-2 pb-3">
        <div className="min-w-0 flex-1">
          <button
            type="button"
            aria-label={`Use ${card.label} theme`}
            aria-pressed={picked.length === card.previews.length}
            className="block max-w-full truncate rounded-sm text-left text-body font-medium text-ink focus-visible:ring-2 focus-visible:ring-accent focus-visible:outline-none focus-visible:ring-inset"
            onClick={(e) => {
              e.stopPropagation()
              onUse()
            }}
          >
            {card.label}
          </button>
        </div>
      </div>
      {anchor && !overDisc && <FloatingTip anchor={anchor} text={cardTip(card)} />}
    </div>
  )
}
