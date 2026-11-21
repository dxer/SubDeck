import React from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import { X } from 'lucide-react'
import { useEscapeKey } from '../hooks/useEscapeKey'
import { useI18n } from '../i18n'

const SIZE_MAP = {
  md: 'max-w-lg',
  lg: 'max-w-2xl',
  xl: 'max-w-3xl'
}

export default function Drawer({
  open,
  title,
  subtitle,
  size = 'lg',
  headerExtra,
  footer,
  onClose,
  children
}) {
  const { t } = useI18n()
  useEscapeKey(open, onClose)

  return (
    <AnimatePresence>
      {open && (
        <div className="fixed inset-0 z-50 flex justify-end">
          <motion.div
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            transition={{ duration: 0.16 }}
            onClick={onClose}
            className="fixed inset-0 bg-black/45 backdrop-blur-xs"
          />
          <motion.div
            initial={{ x: '100%' }}
            animate={{ x: 0 }}
            exit={{ x: '100%' }}
            transition={{ type: 'spring', damping: 32, stiffness: 360 }}
            className={`relative z-10 w-full ${SIZE_MAP[size] || SIZE_MAP.lg} h-[100dvh] bg-white dark:bg-[#11141B] border-l border-zinc-200/80 dark:border-white/10 shadow-2xl flex flex-col`}
          >
            {/* Header */}
            <div className="px-4 sm:px-6 py-3.5 sm:py-4 border-b border-zinc-100 dark:border-white/[0.06] flex items-center justify-between gap-3 shrink-0">
              <div className="min-w-0">
                <h2 className="text-sm sm:text-base font-bold text-zinc-900 dark:text-white truncate">{title}</h2>
                {subtitle && <p className="text-[11px] text-zinc-400 truncate mt-0.5">{subtitle}</p>}
              </div>
              <div className="flex items-center gap-2 shrink-0">
                {headerExtra}
                <button
                  type="button"
                  onClick={onClose}
                  title={t('common.close')}
                  className="p-1.5 rounded-lg text-zinc-400 hover:text-zinc-700 dark:hover:text-white hover:bg-zinc-100 dark:hover:bg-white/5 transition-colors"
                >
                  <X className="w-4 h-4" />
                </button>
              </div>
            </div>

            {/* Body */}
            <div className="flex-1 overflow-y-auto overscroll-contain">{children}</div>

            {/* Footer */}
            {footer && (
              <div className="px-4 sm:px-6 py-3 pb-[calc(0.75rem+env(safe-area-inset-bottom))] sm:pb-3.5 border-t border-zinc-100 dark:border-white/[0.06] bg-zinc-50/70 dark:bg-white/[0.02] shrink-0">
                {footer}
              </div>
            )}
          </motion.div>
        </div>
      )}
    </AnimatePresence>
  )
}
