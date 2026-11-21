import React from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import { AlertTriangle, X } from 'lucide-react'
import { useEscapeKey } from '../hooks/useEscapeKey'
import { useI18n } from '../i18n'

export default function ConfirmDialog({
  open,
  title,
  description,
  confirmText,
  cancelText,
  variant = 'danger',
  loading = false,
  onConfirm,
  onCancel
}) {
  const { t } = useI18n()
  useEscapeKey(open && !loading, onCancel)

  return (
    <AnimatePresence>
      {open && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
          <motion.div
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            onClick={loading ? undefined : onCancel}
            className="fixed inset-0 bg-black/55 backdrop-blur-sm"
          />
          <motion.div
            initial={{ opacity: 0, scale: 0.96, y: 8 }}
            animate={{ opacity: 1, scale: 1, y: 0 }}
            exit={{ opacity: 0, scale: 0.96, y: 8 }}
            transition={{ duration: 0.16, ease: [0.16, 1, 0.3, 1] }}
            className="relative z-10 w-full max-w-sm bg-white dark:bg-[#11141b] rounded-2xl border border-zinc-200/80 dark:border-white/10 shadow-2xl p-5 sm:p-6 space-y-4"
          >
            <div className="flex items-start justify-between gap-3">
              <div className="flex items-start gap-3">
                <div
                  className={`w-9 h-9 rounded-xl flex items-center justify-center shrink-0 ${
                    variant === 'danger'
                      ? 'bg-rose-500/10 text-rose-600 dark:text-rose-400'
                      : 'bg-amber-500/10 text-amber-600 dark:text-amber-400'
                  }`}
                >
                  <AlertTriangle className="w-4.5 h-4.5" />
                </div>
                <div className="space-y-1">
                  <h3 className="text-sm font-bold text-zinc-900 dark:text-white">{title}</h3>
                  <p className="text-xs text-zinc-500 dark:text-zinc-400 leading-relaxed">{description}</p>
                </div>
              </div>
              <button
                type="button"
                disabled={loading}
                onClick={onCancel}
                className="p-1 rounded-lg text-zinc-400 hover:text-zinc-700 dark:hover:text-white transition-colors"
              >
                <X className="w-4 h-4" />
              </button>
            </div>

            <div className="flex items-center justify-end gap-2 pt-2">
              <button
                type="button"
                disabled={loading}
                onClick={onCancel}
                className="hero-btn-bordered px-4 py-2 text-xs font-medium"
              >
                {cancelText || t('common.cancel')}
              </button>
              <button
                type="button"
                disabled={loading}
                onClick={onConfirm}
                className={`${
                  variant === 'danger' ? 'hero-btn-danger' : 'hero-btn-primary'
                } px-4 py-2 text-xs font-semibold disabled:opacity-50`}
              >
                {loading ? t('common.processing') : (confirmText || t('common.confirm'))}
              </button>
            </div>
          </motion.div>
        </div>
      )}
    </AnimatePresence>
  )
}
