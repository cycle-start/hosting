/**
 * Skeleton loading placeholders that match page layouts to prevent layout jumps.
 */

/** Animated placeholder block. */
export function Skeleton({ className = "" }: { className?: string }) {
  return <div className={`animate-pulse rounded bg-gray-200 ${className}`} />;
}

/** Matches the h1 + PageIntro header used by every list page. */
function PageHeaderSkeleton() {
  return (
    <>
      <Skeleton className="mb-2 h-8 w-48" />
      <Skeleton className="mb-6 h-4 w-80" />
    </>
  );
}

/** Grid of placeholder cards matching the list page card layout. */
function CardGridSkeleton({ count = 3 }: { count?: number }) {
  return (
    <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
      {Array.from({ length: count }, (_, i) => (
        <div
          key={i}
          className="rounded-xl bg-white p-6 shadow-sm ring-1 ring-gray-200"
        >
          <div className="mb-3 flex items-start justify-between">
            <Skeleton className="h-5 w-32" />
            <Skeleton className="h-5 w-16 rounded-full" />
          </div>
          <div className="space-y-2">
            <Skeleton className="h-4 w-full" />
            <Skeleton className="h-4 w-2/3" />
          </div>
        </div>
      ))}
    </div>
  );
}

/** Full-page skeleton for list pages (dashboard, databases, webroots, etc.). */
export function PageSkeleton({ cards = 3 }: { cards?: number }) {
  return (
    <div className="p-8">
      <PageHeaderSkeleton />
      <CardGridSkeleton count={cards} />
    </div>
  );
}

/** Full-page skeleton for detail pages (breadcrumbs + title + tab bar). */
export function DetailPageSkeleton({ tabs = 3 }: { tabs?: number }) {
  return (
    <div className="p-8">
      {/* Breadcrumbs */}
      <div className="mb-4 flex items-center gap-2">
        <Skeleton className="h-4 w-20" />
        <Skeleton className="h-4 w-4" />
        <Skeleton className="h-4 w-32" />
      </div>

      {/* Title + status badge */}
      <div className="mb-6 flex items-start justify-between">
        <Skeleton className="h-8 w-56" />
        <Skeleton className="h-5 w-16 rounded-full" />
      </div>

      {/* Tab bar */}
      <div className="mb-6 flex gap-1 rounded-lg bg-gray-100 p-1">
        {Array.from({ length: tabs }, (_, i) => (
          <Skeleton key={i} className="h-9 w-24 rounded-md" />
        ))}
      </div>

      {/* Tab content placeholder */}
      <TabSkeleton />
    </div>
  );
}

/** Skeleton for tab content areas (section header + table rows). */
export function TabSkeleton({ rows = 3 }: { rows?: number }) {
  return (
    <div>
      <div className="mb-4 flex items-center justify-between">
        <Skeleton className="h-6 w-32" />
        <Skeleton className="h-8 w-24 rounded-lg" />
      </div>
      <div className="overflow-hidden rounded-xl bg-white shadow-sm ring-1 ring-gray-200">
        {/* Table header */}
        <div className="flex gap-4 bg-gray-50 px-4 py-3">
          <Skeleton className="h-3 w-24" />
          <Skeleton className="h-3 w-32" />
          <Skeleton className="h-3 w-16" />
        </div>
        {/* Table rows */}
        {Array.from({ length: rows }, (_, i) => (
          <div key={i} className="flex items-center gap-4 border-t border-gray-200 px-4 py-3">
            <Skeleton className="h-4 w-28" />
            <Skeleton className="h-4 w-40" />
            <Skeleton className="h-4 w-14 rounded-full" />
          </div>
        ))}
      </div>
    </div>
  );
}
