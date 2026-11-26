type SkeletonProps = {
  shape: 'block' | 'card' | 'pill'
  width?: string
  height?: string
}

export function Skeleton({ shape, width, height }: SkeletonProps) {
  return <span aria-hidden="true" className={`skeleton skeleton-${shape}`} style={{ width, height }} />
}
