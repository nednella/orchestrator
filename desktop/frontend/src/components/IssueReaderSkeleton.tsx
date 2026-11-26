import { Skeleton } from './Skeleton'

const LINES = ['100%', '92%', '97%', '58%']

export function IssueReaderSkeleton() {
  return (
    <div role="status" aria-label="Loading" className="flex flex-col gap-3">
      <Skeleton shape="pill" width="45%" height="14px" />
      <div className="flex flex-col gap-2">
        {LINES.map((width) => (
          <Skeleton key={width} shape="pill" width={width} height="10px" />
        ))}
      </div>
    </div>
  )
}
