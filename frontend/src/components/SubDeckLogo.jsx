import React from 'react'

export default function SubDeckMark({ className = 'w-9 h-9' }) {
  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      viewBox="0 0 48 48"
      fill="none"
      className={className}
      aria-hidden="true"
    >
      <defs>
        <linearGradient id="sd-mark-bg" x1="6" y1="4" x2="42" y2="44" gradientUnits="userSpaceOnUse">
          <stop offset="0%" stopColor="#2563EB" />
          <stop offset="100%" stopColor="#1D4ED8" />
        </linearGradient>
      </defs>
      <rect x="2" y="2" width="44" height="44" rx="12" fill="url(#sd-mark-bg)" />
      <rect x="2.5" y="2.5" width="43" height="43" rx="11.5" stroke="#FFFFFF" strokeOpacity="0.18" />
      <rect x="11" y="11.5" width="26" height="6.5" rx="2.2" fill="#FFFFFF" />
      <circle cx="33.5" cy="14.75" r="1.6" fill="#10B981" />
      <rect x="11" y="20.75" width="19.5" height="6.5" rx="2.2" fill="#FFFFFF" fillOpacity="0.82" />
      <circle cx="34.25" cy="24" r="2.75" fill="#93C5FD" />
      <rect x="11" y="30" width="26" height="6.5" rx="2.2" fill="#FFFFFF" fillOpacity="0.52" />
    </svg>
  )
}
