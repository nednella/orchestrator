import { Skeleton } from './Skeleton'

const WIDTHS = ['40%', '60%', '33%']

export function SkeletonRows() {
  return (
    <div role="status" aria-label="Loading" className="flex flex-col gap-2 px-3 py-2">
      {WIDTHS.map((width) => (
        <div key={width} className="flex items-center gap-2">
          <Skeleton shape="block" width="12px" height="12px" />
          <Skeleton shape="pill" width={width} height="10px" />
        </div>
      ))}
    </div>
  )
}
