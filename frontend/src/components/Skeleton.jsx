import React from 'react'

export function MetricSkeleton() {
  return (
    <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
      {[...Array(4)].map((_, i) => (
        <div key={i} className="hero-card p-5 space-y-3 animate-pulse">
          <div className="flex items-center justify-between">
            <div className="h-3.5 w-20 rounded bg-zinc-200/80 dark:bg-white/10" />
            <div className="w-9 h-9 rounded-xl bg-zinc-200/80 dark:bg-white/10" />
          </div>
          <div className="h-8 w-16 rounded bg-zinc-200/80 dark:bg-white/10" />
          <div className="pt-2 border-t border-zinc-100 dark:border-white/5">
            <div className="h-3 w-32 rounded bg-zinc-100 dark:bg-white/5" />
          </div>
        </div>
      ))}
    </div>
  )
}

export function CardGridSkeleton({ count = 6 }) {
  return (
    <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
      {[...Array(count)].map((_, i) => (
        <div key={i} className="hero-card p-5 space-y-4 animate-pulse">
          <div className="flex items-start justify-between gap-3">
            <div className="space-y-2 flex-1">
              <div className="flex items-center gap-2">
                <div className="h-4 w-16 rounded-full bg-zinc-200/80 dark:bg-white/10" />
                <div className="h-4 w-28 rounded bg-zinc-200/80 dark:bg-white/10" />
              </div>
              <div className="h-3 w-40 rounded bg-zinc-100 dark:bg-white/5" />
            </div>
            <div className="h-7 w-20 rounded-lg bg-zinc-100 dark:bg-white/5" />
          </div>
          <div className="pt-3 border-t border-zinc-100 dark:border-white/5 flex justify-between">
            <div className="h-3 w-20 rounded bg-zinc-100 dark:bg-white/5" />
            <div className="h-3 w-16 rounded bg-zinc-100 dark:bg-white/5" />
          </div>
        </div>
      ))}
    </div>
  )
}

export function RowListSkeleton({ count = 3 }) {
  return (
    <div className="space-y-3 p-5">
      {[...Array(count)].map((_, i) => (
        <div
          key={i}
          className="p-4 rounded-xl border border-zinc-100 dark:border-white/5 flex flex-col sm:flex-row sm:items-center justify-between gap-4 animate-pulse"
        >
          <div className="space-y-2.5 flex-1">
            <div className="flex items-center gap-2.5">
              <div className="h-5 w-36 rounded bg-zinc-200/80 dark:bg-white/10" />
              <div className="h-5 w-16 rounded-full bg-zinc-100 dark:bg-white/5" />
            </div>
            <div className="h-3.5 w-52 rounded bg-zinc-100 dark:bg-white/5" />
          </div>
          <div className="flex items-center gap-2">
            <div className="h-8 w-28 rounded-xl bg-zinc-200/80 dark:bg-white/10" />
            <div className="h-8 w-24 rounded-xl bg-zinc-100 dark:bg-white/5" />
          </div>
        </div>
      ))}
    </div>
  )
}
