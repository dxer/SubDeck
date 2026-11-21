import React, { useState, useEffect, useMemo } from 'react'
import { Copy, Search, Link2, RefreshCw, FileCode2, X } from 'lucide-react'
import Drawer from './Drawer'
import { getExportUrl, copyToClipboard } from '../utils/subUrl'
import { useI18n } from '../i18n'

export default function ConfigPreviewDrawer({ open, profile, onClose, onNotify }) {
  const { t } = useI18n()
  const [activeTab, setActiveTab] = useState('clash-full')
  const [rawText, setRawText] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [filterQuery, setFilterQuery] = useState('')

  const formatTabs = useMemo(
    () => [
      { key: 'clash-full', label: t('configPreview.clashFull'), format: 'clash', full: true },
      { key: 'singbox-full', label: t('configPreview.singboxFull'), format: 'singbox', full: true },
      { key: 'clash-provider', label: t('configPreview.clashProvider'), format: 'clash', full: false },
      { key: 'base64', label: t('configPreview.base64'), format: 'base64', full: false },
      { key: 'json', label: t('configPreview.nodesJson'), format: 'json', full: false }
    ],
    [t]
  )

  const currentSpec = useMemo(
    () => formatTabs.find((item) => item.key === activeTab) || formatTabs[0],
    [activeTab, formatTabs]
  )

  const targetUrl = useMemo(() => {
    if (!profile?.token) return ''
    return getExportUrl(profile.token, currentSpec.format, currentSpec.full)
  }, [profile, currentSpec])

  useEffect(() => {
    if (!open || !targetUrl) return
    let cancelled = false
    setLoading(true)
    setError('')
    fetch(targetUrl)
      .then(async (res) => {
        const text = await res.text()
        if (!res.ok) throw new Error(text || `HTTP ${res.status}`)
        if (!cancelled) setRawText(text)
      })
      .catch((err) => {
        if (!cancelled) setError(err.message || 'Fetch failed')
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [open, targetUrl])

  useEffect(() => {
    if (open) {
      setFilterQuery('')
    }
  }, [open, profile?.id])

  const displayedText = useMemo(() => {
    const q = filterQuery.trim().toLowerCase()
    if (!q || !rawText) return rawText
    return rawText
      .split('\n')
      .filter((line) => line.toLowerCase().includes(q))
      .join('\n')
  }, [rawText, filterQuery])

  const stats = useMemo(() => {
    if (!rawText) return { lines: 0, kb: '0.0' }
    const lines = rawText.split('\n').length
    const kb = (new Blob([rawText]).size / 1024).toFixed(1)
    return { lines, kb }
  }, [rawText])

  const handleCopyContent = async () => {
    const ok = await copyToClipboard(rawText)
    onNotify?.(ok ? t('configPreview.contentCopied') : t('common.copyFailed'), ok ? 'success' : 'error')
  }

  const handleCopyUrl = async () => {
    const ok = await copyToClipboard(targetUrl)
    onNotify?.(
      ok ? t('configPreview.urlCopied', { label: currentSpec.label }) : t('common.copyFailed'),
      ok ? 'success' : 'error'
    )
  }

  return (
    <Drawer
      open={open}
      onClose={onClose}
      size="xl"
      title={t('configPreview.title', { name: profile?.name || '' })}
      headerExtra={
        <span className="hero-chip-default font-mono text-[11px] tabular-nums hidden sm:inline-flex">
          {t('configPreview.stats', { lines: stats.lines, kb: stats.kb })}
        </span>
      }
      footer={
        <div className="flex flex-wrap items-center justify-between gap-2">
          <button
            type="button"
            onClick={handleCopyUrl}
            className="hero-btn-bordered px-3 py-2 text-xs font-medium gap-1.5"
          >
            <Link2 className="w-3.5 h-3.5 text-[#2563EB] dark:text-[#60A5FA]" />
            <span>{t('configPreview.copyUrl')}</span>
          </button>
          <div className="flex items-center gap-2 ml-auto">
            <button
              type="button"
              onClick={onClose}
              className="hero-btn-bordered px-3.5 py-2 text-xs font-medium"
            >
              {t('common.close')}
            </button>
            <button
              type="button"
              disabled={!rawText || loading}
              onClick={handleCopyContent}
              className="hero-btn-primary px-3.5 sm:px-4 py-2 text-xs font-semibold gap-1.5 disabled:opacity-50"
            >
              <Copy className="w-3.5 h-3.5" />
              <span>{t('configPreview.copyContent')}</span>
            </button>
          </div>
        </div>
      }
    >
      <div className="p-4 sm:p-5 space-y-3.5 sm:space-y-4">
        {/* Format Switcher + Filter */}
        <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-2.5 sm:gap-3">
          <div className="flex p-1 bg-zinc-100 dark:bg-white/[0.04] rounded-xl gap-1 overflow-x-auto">
            {formatTabs.map((tab) => (
              <button
                key={tab.key}
                type="button"
                onClick={() => setActiveTab(tab.key)}
                className={`px-3 py-1.5 rounded-lg text-xs font-semibold transition-all whitespace-nowrap ${
                  activeTab === tab.key
                    ? 'bg-white dark:bg-[#1E2330] text-zinc-900 dark:text-white shadow-xs'
                    : 'text-zinc-500 hover:text-zinc-900 dark:hover:text-white'
                }`}
              >
                {tab.label}
              </button>
            ))}
          </div>

          <div className="relative w-full sm:w-56">
            <Search className="w-3.5 h-3.5 text-zinc-400 absolute left-3 top-1/2 -translate-y-1/2" />
            <input
              value={filterQuery}
              onChange={(e) => setFilterQuery(e.target.value)}
              type="text"
              placeholder={t('configPreview.filterPh')}
              className="hero-input pl-8 pr-7 py-1.5 text-sm sm:text-xs"
            />
            {filterQuery && (
              <button
                type="button"
                onClick={() => setFilterQuery('')}
                className="absolute right-2 top-1/2 -translate-y-1/2 text-zinc-400 hover:text-zinc-700 dark:hover:text-white"
              >
                <X className="w-3.5 h-3.5" />
              </button>
            )}
          </div>
        </div>

        {/* URL Strip */}
        <div className="px-3 py-2 rounded-lg bg-zinc-50 dark:bg-white/[0.03] border border-zinc-200/70 dark:border-white/[0.07] flex items-center justify-between gap-2 text-[11px] font-mono text-zinc-500 dark:text-zinc-400">
          <span className="truncate">{targetUrl}</span>
          <FileCode2 className="w-3.5 h-3.5 shrink-0 text-zinc-400" />
        </div>

        {/* Code Output */}
        {loading ? (
          <div className="p-8 rounded-xl border border-zinc-200/70 dark:border-white/[0.08] bg-zinc-50 dark:bg-[#0B0E14] flex items-center justify-center gap-2 text-xs text-zinc-400">
            <RefreshCw className="w-4 h-4 animate-spin text-[#2563EB]" />
            <span>{t('common.loading')}</span>
          </div>
        ) : error ? (
          <div className="p-4 rounded-xl bg-rose-500/10 border border-rose-500/20 text-xs text-rose-600 dark:text-rose-400">
            {error}
          </div>
        ) : (
          <pre className="p-3.5 sm:p-4 rounded-xl border border-zinc-200/80 dark:border-white/[0.08] bg-[#F8FAFC] dark:bg-[#0B0E14] text-[11px] sm:text-[11.5px] leading-relaxed font-mono text-zinc-800 dark:text-zinc-200 overflow-x-auto max-h-[58dvh] sm:max-h-[62vh] whitespace-pre">
            {displayedText || t('configPreview.empty')}
          </pre>
        )}
      </div>
    </Drawer>
  )
}
