import { X } from 'lucide-react'

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
            className="block overflow-hidden rounded-md border border-(--border-rule)"
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
              <X size={12} aria-hidden="true" />
            </button>
          )}
        </span>
      ))}
    </div>
  )
}
