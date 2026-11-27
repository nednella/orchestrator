import { Check } from 'lucide-react'
import { useId, useMemo } from 'react'
import type { QuestionOption } from '../chatQuestions'
import { isWidePreview, previewParts } from '../questionPreview'
import { QuestionPreview } from './QuestionPreview'

type QuestionTileProps = { thread: string; number: number; option: QuestionOption; multi: boolean; on: boolean; onPick(): void }

// QuestionTile is an option as a tile: its number key, the mark of its pick, its preview, its label and description.
// The button that picks it lies under all of them, so a click anywhere in the tile picks, but one on a preview's own
// button or link opens that instead.
export function QuestionTile({ thread, number, option, multi, on, onPick }: QuestionTileProps) {
  const label = useId()
  const parts = useMemo(() => previewParts(option.preview), [option.preview])
  return (
    <div className="q-tile" data-on={on} data-wide={isWidePreview(parts)}>
      <button type="button" className="q-tile-hit" role={multi ? 'checkbox' : 'radio'} aria-checked={on} aria-labelledby={label} data-q-tile onClick={onPick} />
      <div className="q-tile-top">
        <kbd className="q-key">{number}</kbd>
        <span className="q-ind" data-multi={multi}>
          <Check aria-hidden="true" />
        </span>
      </div>
      {parts.length > 0 && (
        <div onClick={(e) => !(e.target as HTMLElement).closest('button, a') && onPick()}>
          <QuestionPreview thread={thread} parts={parts} />
        </div>
      )}
      <span className="q-tile-text">
        <span id={label} className="q-tile-label">
          {option.label}
        </span>
        {option.description && <span className="q-tile-desc">{option.description}</span>}
      </span>
    </div>
  )
}
