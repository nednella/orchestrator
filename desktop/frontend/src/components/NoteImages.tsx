import { Icon } from './Icon'

type NoteImagesProps = {
  urls: string[]
  attachedTo?: 'note' | 'message'
  onOpen?(url: string): void
  onRemove?(url: string): void
}

export function NoteImages({ urls, attachedTo = 'note', onOpen, onRemove }: NoteImagesProps) {
  if (urls.length === 0) return null
  return (
    <div className="flex flex-wrap gap-2">
      {urls.map((url) => (
        <span key={url} className="relative">
          <button
            type="button"
            className="block overflow-hidden rounded-sm border"
            style={{ borderColor: attachedTo === 'note' ? 'var(--border-note)' : 'var(--border-strong)' }}
            title="Open larger"
            onClick={(e) => {
              e.stopPropagation()
              onOpen?.(url)
            }}
          >
            <img src={url} alt={`Attached to the ${attachedTo}`} className="block h-16 w-auto max-w-40 object-cover" />
          </button>
          {onRemove && (
            <button
              type="button"
              className="absolute -top-1.5 -right-1.5 grid h-5 w-5 place-items-center rounded-full border border-line-strong bg-raised"
              aria-label="Remove image"
              onMouseDown={(e) => e.preventDefault()}
              onClick={() => onRemove(url)}
            >
              <Icon name="x" size={10} />
            </button>
          )}
        </span>
      ))}
    </div>
  )
}
