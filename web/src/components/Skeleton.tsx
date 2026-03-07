interface SkeletonProps {
  className?: string
  count?: number
}

function SkeletonLine({ className = '' }: { className?: string }) {
  return <div className={`animate-pulse bg-gray-200 dark:bg-gray-700 rounded ${className}`} />
}

export default function Skeleton({ className = 'h-4 w-full', count = 1 }: SkeletonProps) {
  if (count === 1) return <SkeletonLine className={className} />
  return (
    <div className="space-y-3">
      {Array.from({ length: count }).map((_, i) => (
        <SkeletonLine key={i} className={className} />
      ))}
    </div>
  )
}

export function CardSkeleton() {
  return (
    <div className="bg-white dark:bg-gray-800 border rounded-xl p-5 animate-pulse">
      <div className="flex items-start justify-between mb-3">
        <div className="w-10 h-10 bg-gray-200 dark:bg-gray-700 rounded-lg" />
        <div className="w-16 h-4 bg-gray-200 dark:bg-gray-700 rounded" />
      </div>
      <div className="h-5 bg-gray-200 dark:bg-gray-700 rounded w-2/3 mb-2" />
      <div className="h-4 bg-gray-200 dark:bg-gray-700 rounded w-full mb-1" />
      <div className="h-4 bg-gray-200 dark:bg-gray-700 rounded w-4/5" />
    </div>
  )
}
