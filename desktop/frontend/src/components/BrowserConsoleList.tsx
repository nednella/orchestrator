import { CircleAlert } from 'lucide-react'

type BrowserConsoleListProps = { lines: string[] }

export function BrowserConsoleList({ lines }: BrowserConsoleListProps) {
  return (
    <section>
      <h4 className="pb-2 text-small font-medium text-soft">Console</h4>
      {lines.length === 0 ? (
        <p className="text-small text-dim">No console errors</p>
      ) : (
        <ul className="flex flex-col gap-1">
          {lines.map((line, i) => (
            <li key={i} className="flex items-baseline gap-2" title={line}>
              <span className="flex-none self-center text-danger">
                <CircleAlert size={12} aria-hidden="true" />
              </span>
              <span className="mono min-w-0 truncate text-small">{line}</span>
            </li>
          ))}
        </ul>
      )}
    </section>
  )
}
