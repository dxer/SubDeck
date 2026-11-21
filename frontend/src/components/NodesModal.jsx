import React, { useState, useMemo, useEffect } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import { Search, Copy, X, Server, Activity } from 'lucide-react'
import { getProtocolBadge } from '../utils/subUrl'
import { useEscapeKey } from '../hooks/useEscapeKey'
import { useI18n } from '../i18n'

export default function NodesModal({
  open,
  title,
  nodes = [],
  loading = false,
  probing = false,
  emptyText,
  onClose,
  onCopy,
  onProbe
}) {
  const { t } = useI18n()
  const [query, setQuery] = useState('')
  const [protoFilter, setProtoFilter] = useState('all')
  useEscapeKey(open, onClose)

  useEffect(() => {
    if (open) {
      setQuery('')
      setProtoFilter('all')
    }
  }, [open, title])

  const protocols = useMemo(() => {
    const seen = new Set()
    const list = []
    nodes.forEach((n) => {
      const p = (n.type || '').toLowerCase()
      if (p && !seen.has(p)) {
        seen.add(p)
        list.push(p)
      }
    })
    return list
  }, [nodes])

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase()
    return nodes.filter((n) => {
      const matchProto = protoFilter === 'all' || (n.type && n.type.toLowerCase() === protoFilter)
      const matchQuery =
        !q ||
        (n.name && n.name.toLowerCase().includes(q)) ||
        (n.server && n.server.toLowerCase().includes(q)) ||
        (n.type && n.type.toLowerCase().includes(q))
      return matchProto && matchQuery
    })
  }, [nodes, protoFilter, query])

  return (
    <AnimatePresence>
      {open && (
        <div className="fixed inset-0 z-50 flex items-end sm:items-center justify-center sm:p-4">
          <motion.div
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            onClick={onClose}
            className="fixed inset-0 bg-black/55 backdrop-blur-sm"
          />
          <motion.div
            initial={{ opacity: 0, scale: 0.98, y: 16 }}
            animate={{ opacity: 1, scale: 1, y: 0 }}
            exit={{ opacity: 0, scale: 0.98, y: 16 }}
            transition={{ duration: 0.16, ease: [0.16, 1, 0.3, 1] }}
            className="relative z-10 w-full sm:max-w-2xl bg-white dark:bg-[#11141b] rounded-t-2xl sm:rounded-2xl shadow-2xl border border-zinc-200/80 dark:border-white/10 overflow-hidden flex flex-col max-h-[88dvh] sm:max-h-[85vh]"
          >
            <div className="px-4 sm:px-6 py-3.5 sm:py-4 border-b border-zinc-100 dark:border-white/5 flex items-center justify-between gap-2">
              <div className="flex items-center gap-2 min-w-0">
                <div className="w-7 h-7 rounded-lg bg-[#2563EB]/10 text-[#2563EB] dark:text-[#60A5FA] flex items-center justify-center shrink-0">
                  <Server className="w-3.5 h-3.5" />
                </div>
                <h3 className="text-sm font-bold text-zinc-900 dark:text-white truncate">{title}</h3>
                <span className="hero-chip-primary font-mono text-[11px] tabular-nums shrink-0">
                  {nodes.length}
                </span>
              </div>
              <div className="flex items-center gap-1.5 shrink-0">
                {onProbe && nodes.length > 0 && (
                  <button
                    type="button"
                    disabled={probing}
                    onClick={onProbe}
                    className="hero-btn-bordered px-2.5 py-1.5 text-[11px] font-semibold gap-1.5 disabled:opacity-60"
                  >
                    <Activity className={`w-3.5 h-3.5 text-emerald-500 ${probing ? 'animate-pulse' : ''}`} />
                    <span>{probing ? t('nodesModal.probing') : t('nodesModal.probeBtn')}</span>
                  </button>
                )}
                <button
                  type="button"
                  onClick={onClose}
                  className="p-1.5 rounded-lg text-zinc-400 hover:text-zinc-600 dark:hover:text-white transition-colors"
                >
                  <X className="w-4 h-4" />
                </button>
              </div>
            </div>

            <div className="px-4 sm:px-5 py-3 border-b border-zinc-100 dark:border-white/5 bg-zinc-50/60 dark:bg-white/[0.02] space-y-2.5">
              <div className="relative">
                <Search className="w-3.5 h-3.5 text-zinc-400 absolute left-3 top-1/2 -translate-y-1/2" />
                <input
                  value={query}
                  onChange={(e) => setQuery(e.target.value)}
                  type="text"
                  placeholder={t('nodesModal.searchPh')}
                  className="hero-input pl-8 pr-8 py-2 text-sm sm:text-xs text-zinc-800 dark:text-zinc-200"
                />
                {query && (
                  <button
                    type="button"
                    onClick={() => setQuery('')}
                    className="absolute right-2.5 top-1/2 -translate-y-1/2 text-zinc-400 hover:text-zinc-600 dark:hover:text-zinc-200"
                  >
                    <X className="w-3.5 h-3.5" />
                  </button>
                )}
              </div>

              {protocols.length > 0 && (
                <div className="flex items-center gap-1.5 overflow-x-auto pb-0.5">
                  <button
                    type="button"
                    onClick={() => setProtoFilter('all')}
                    className={`px-2.5 py-1 rounded-lg text-[11px] font-semibold transition-all shrink-0 tabular-nums ${
                      protoFilter === 'all'
                        ? 'bg-[#2563EB] text-white shadow-xs'
                        : 'bg-zinc-100 dark:bg-white/5 text-zinc-500 hover:text-zinc-900 dark:hover:text-white'
                    }`}
                  >
                    {t('nodesModal.allProto', { count: nodes.length })}
                  </button>
                  {protocols.map((proto) => {
                    const count = nodes.filter((n) => (n.type || '').toLowerCase() === proto).length
                    return (
                      <button
                        key={proto}
                        type="button"
                        onClick={() => setProtoFilter(proto)}
                        className={`px-2.5 py-1 rounded-lg text-[11px] font-mono font-bold uppercase transition-all shrink-0 tabular-nums ${
                          protoFilter === proto
                            ? 'bg-[#2563EB] text-white shadow-xs'
                            : 'bg-zinc-100 dark:bg-white/5 text-zinc-500 hover:text-zinc-900 dark:hover:text-white'
                        }`}
                      >
                        {proto} ({count})
                      </button>
                    )
                  })}
                </div>
              )}
            </div>

            <div className="p-3 sm:p-4 overflow-y-auto divide-y divide-zinc-100 dark:divide-white/5 font-mono text-xs flex-1 overscroll-contain">
              {loading ? (
                <div className="space-y-2.5 py-2">
                  {[...Array(6)].map((_, i) => (
                    <div key={i} className="h-9 rounded-lg bg-zinc-100 dark:bg-white/5 animate-pulse" />
                  ))}
                </div>
              ) : filtered.length === 0 ? (
                <div className="py-12 text-center text-zinc-400 text-xs font-sans">
                  {query ? t('nodesModal.emptySearch') : (emptyText || t('nodesModal.emptyDefault'))}
                </div>
              ) : (
                filtered.map((node, index) => (
                  <div
                    key={index}
                    className="py-2.5 flex items-center justify-between gap-2.5 text-xs hover:bg-zinc-50/80 dark:hover:bg-white/[0.03] px-2 rounded-lg transition-colors group"
                  >
                    <div className="flex items-center gap-2 min-w-0">
                      <span className="w-6 sm:w-7 text-left font-mono text-[10px] text-zinc-400 shrink-0 tabular-nums">
                        #{index + 1}
                      </span>
                      <div className="min-w-0">
                        <div
                          className="font-medium text-zinc-800 dark:text-zinc-200 truncate font-sans"
                          title={node.name}
                        >
                          {node.name}
                        </div>
                        <div className="text-[10px] text-zinc-400 truncate sm:hidden tabular-nums">
                          {node.server}:{node.port}
                        </div>
                      </div>
                    </div>
                    <div className="flex items-center gap-1.5 sm:gap-2 shrink-0">
                      {typeof node.latency_ms === 'number' && node.latency_ms !== 0 && (
                        <span
                          className={`px-1.5 py-0.5 rounded text-[10px] font-mono font-semibold tabular-nums border ${
                            node.latency_ms > 0
                              ? node.latency_ms <= 200
                                ? 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border-emerald-500/20'
                                : 'bg-amber-500/10 text-amber-600 dark:text-amber-400 border-amber-500/20'
                              : 'bg-rose-500/10 text-rose-600 dark:text-rose-400 border-rose-500/20'
                          }`}
                        >
                          {node.latency_ms > 0 ? `${node.latency_ms}ms` : t('nodesModal.timeout')}
                        </span>
                      )}
                      <span
                        className={`px-2 py-0.5 rounded-full text-[10px] font-bold uppercase border ${getProtocolBadge(node.type)}`}
                      >
                        {node.type}
                      </span>
                      <span className="text-zinc-400 text-[11px] hidden sm:inline tabular-nums">
                        {node.server}:{node.port}
                      </span>
                      {node.raw_uri && (
                        <button
                          type="button"
                          onClick={() =>
                            onCopy(node.raw_uri, t('nodesModal.nodeUriCopied', { name: node.name }))
                          }
                          title={t('nodesModal.copyNodeUri')}
                          className="p-1.5 text-zinc-400 hover:text-[#2563EB] dark:hover:text-[#60A5FA] sm:opacity-60 sm:group-hover:opacity-100 transition-opacity"
                        >
                          <Copy className="w-3.5 h-3.5" />
                        </button>
                      )}
                    </div>
                  </div>
                ))
              )}
            </div>

            <div className="px-4 sm:px-6 py-3 pb-[calc(0.75rem+env(safe-area-inset-bottom))] sm:pb-3 border-t border-zinc-100 dark:border-white/5 flex items-center justify-between text-xs text-zinc-400 tabular-nums">
              <span>{t('nodesModal.footerCount', { filtered: filtered.length, total: nodes.length })}</span>
              <button
                type="button"
                onClick={onClose}
                className="hero-btn-bordered px-4 py-1.5 text-xs font-medium"
              >
                {t('common.close')}
              </button>
            </div>
          </motion.div>
        </div>
      )}
    </AnimatePresence>
  )
}
