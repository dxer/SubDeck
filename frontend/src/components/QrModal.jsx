import React, { useMemo, useState, useEffect } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import { X, Copy } from 'lucide-react'
import { QRCodeSVG } from 'qrcode.react'
import { getUniversalUrl, getExportUrl } from '../utils/subUrl'

export default function QrModal({ open, token, profileName, onClose, onCopy }) {
  const [qrFormat, setQrFormat] = useState('auto')

  useEffect(() => {
    if (open) setQrFormat('auto')
  }, [open, token])

  const currentQrUrl = useMemo(() => {
    if (!token) return ''
    if (qrFormat === 'auto') return getUniversalUrl(token)
    if (qrFormat === 'clash') return getExportUrl(token, 'clash', true)
    if (qrFormat === 'singbox') return getExportUrl(token, 'singbox', true)
    if (qrFormat === 'base64') return getExportUrl(token, 'base64', false)
    return getUniversalUrl(token)
  }, [token, qrFormat])

  return (
    <AnimatePresence>
      {open && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
          <motion.div
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            onClick={onClose}
            className="fixed inset-0 bg-black/55 backdrop-blur-sm"
          />
          <motion.div
            initial={{ opacity: 0, scale: 0.96, y: 8 }}
            animate={{ opacity: 1, scale: 1, y: 0 }}
            exit={{ opacity: 0, scale: 0.96, y: 8 }}
            transition={{ duration: 0.16, ease: [0.16, 1, 0.3, 1] }}
            className="relative z-10 w-full max-w-xs bg-white dark:bg-[#11141b] rounded-2xl shadow-2xl border border-zinc-200/80 dark:border-white/10 p-6 space-y-4 text-center"
          >
            <div className="flex items-center justify-between">
              <div className="text-left min-w-0">
                <h3 className="text-sm font-bold text-zinc-900 dark:text-white">手机扫码添加订阅</h3>
                <p className="text-[11px] text-zinc-400 truncate max-w-[200px]">{profileName}</p>
              </div>
              <button
                type="button"
                onClick={onClose}
                className="p-1 rounded-lg text-zinc-400 hover:text-zinc-600 dark:hover:text-white transition-colors"
              >
                <X className="w-4 h-4" />
              </button>
            </div>

            <div className="flex p-1 bg-zinc-100 dark:bg-white/5 rounded-xl text-xs">
              {['auto', 'clash', 'singbox', 'base64'].map((fmt) => (
                <button
                  key={fmt}
                  type="button"
                  onClick={() => setQrFormat(fmt)}
                  className={`flex-1 py-1.5 rounded-lg font-semibold transition-all ${
                    qrFormat === fmt
                      ? 'bg-white dark:bg-white/12 text-zinc-900 dark:text-white shadow-xs'
                      : 'text-zinc-400 hover:text-zinc-600 dark:hover:text-zinc-300'
                  }`}
                >
                  {fmt === 'auto' ? '自适应' : fmt.toUpperCase()}
                </button>
              ))}
            </div>

            <div className="p-4 bg-white rounded-2xl inline-block border border-zinc-200/60 shadow-xs mx-auto">
              <QRCodeSVG value={currentQrUrl} size={180} level="M" />
            </div>

            <div className="space-y-2">
              <button
                type="button"
                onClick={() => onCopy(currentQrUrl, '当前格式订阅链接已复制！')}
                className="w-full hero-btn-flat py-2 px-3 text-xs gap-1.5"
              >
                <Copy className="w-3.5 h-3.5" />
                <span>复制当前二维码链接</span>
              </button>
              <p className="text-[11px] text-zinc-400 leading-relaxed">
                Shadowrocket / Clash Verge / Sing-box 相机直接扫码
              </p>
            </div>
          </motion.div>
        </div>
      )}
    </AnimatePresence>
  )
}
