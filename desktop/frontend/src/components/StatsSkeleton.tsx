import { Skeleton } from './Skeleton'

const CARDS = ['7rem', '12rem', '9rem']

export function StatsSkeleton() {
  return (
    <div role="status" aria-label="Loading" className="mt-4 flex flex-col gap-5">
      {CARDS.map((height) => (
        <Skeleton key={height} shape="card" width="100%" height={height} />
      ))}
    </div>
  )
}
