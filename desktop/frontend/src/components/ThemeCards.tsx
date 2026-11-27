import { useOrchestrator } from '../OrchestratorContext'
import { pickedModes, THEME_CARDS, themeHalves, withCard } from '../themes'
import { ThemeCard } from './ThemeCard'

export function ThemeCards() {
  const { settings, setThemes, report } = useOrchestrator('settings', 'setThemes', 'report')
  const halves = themeHalves(settings)
  return (
    <div className="grid w-full grid-cols-[repeat(auto-fit,minmax(min(100%,16rem),1fr))] gap-2">
      {THEME_CARDS.map((card) => (
        <ThemeCard
          key={card.id}
          card={card}
          picked={pickedModes(card, halves)}
          onUse={(mode) => {
            const { dark, light } = withCard(card, halves, mode)
            report(() => setThemes(dark, light))
          }}
        />
      ))}
    </div>
  )
}
