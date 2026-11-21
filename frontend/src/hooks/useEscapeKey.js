import { useEffect } from 'react'

export function useEscapeKey(enabled, onEscape) {
  useEffect(() => {
    if (!enabled) return
    const handleKeyDown = (e) => {
      if (e.key === 'Escape') {
        e.stopPropagation()
        onEscape?.()
      }
    }
    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [enabled, onEscape])
}
