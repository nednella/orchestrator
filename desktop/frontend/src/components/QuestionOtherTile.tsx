import { Check } from 'lucide-react'
import { useId } from 'react'
import type { RefObject } from 'react'

type QuestionOtherTileProps = { number: number; multi: boolean; text: string | null; inputRef: RefObject<HTMLInputElement>; onToggle(): void; onText(text: string): void }

// QuestionOtherTile is the last tile of a question: the owner's own answer. It opens an input, text being null while it
// is not picked.
export function QuestionOtherTile({ number, multi, text, inputRef, onToggle, onText }: QuestionOtherTileProps) {
  const label = useId()
  const on = text !== null
  return (
    <div className="q-tile" data-on={on} data-wide="true" data-other="true">
      <button type="button" className="q-tile-hit" role={multi ? 'checkbox' : 'radio'} aria-checked={on} aria-labelledby={label} data-q-tile onClick={onToggle} />
      <div className="q-tile-top" data-inline="true">
        <kbd className="q-key">{number}</kbd>
        <span id={label} className="q-tile-label">
          Other
        </span>
        <span className="q-ind" data-multi={multi}>
          <Check aria-hidden="true" />
        </span>
      </div>
      {on && <input ref={inputRef} className="field q-other-input" aria-label="Your own answer" placeholder="Type your own answer" value={text} onChange={(e) => onText(e.target.value)} />}
    </div>
  )
}
