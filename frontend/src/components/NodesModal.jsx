import React, { useState, useMemo, useEffect } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import { Search, Copy, X, Server } from 'lucide-react'
import { getProtocolBadge } from '../utils/subUrl'

export default function NodesModal({
  open,
  title,
  nodes = [],
  loading = false,
  emptyText = '暂无可用节点。',
  onClose,
  onCopy
}) {
  const [query, setQuery] = useState('')
  const [protoFilter, setProtoFilter] = useState('all')

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
      const t = (n.type || '').toLowerCase()
      if (t && !seen.has(t)) {
        seen.add(t)
        list.push(t)
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
            className="relative z-10 w-full max-w-2xl bg-white dark:bg-[#11141b] rounded-2xl shadow-2xl border border-zinc-200/80 dark:border-white/10 overflow-hidden flex flex-col max-h-[85vh]"
          >
            <div className="px-6 py-4 border-b border-zinc-100 dark:border-white/5 flex items-center justify-between">
              <div className="flex items-center gap-2.5 min-w-0">
                <div className="w-7 h-7 rounded-lg bg-[#2563EB]/10 text-[#2563EB] dark:text-[#60A5FA] flex items-center justify-center shrink-0">
                  <Server className="w-3.5 h-3.5" />
                </div>
                <h3 className="text-sm font-bold text-zinc-900 dark:text-white truncate">{title}</h3>
                <span className="hero-chip-primary font-mono text-[11px] tabular-nums shrink-0">
                  {nodes.length} 节点
                </span>
              </div>
              <button
                type="button"
                onClick={onClose}
                className="p-1 rounded-lg text-zinc-400 hover:text-zinc-600 dark:hover:text-white transition-colors"
              >
                <X className="w-4 h-4" />
              </button>
            </div>

            <div className="px-5 py-3 border-b border-zinc-100 dark:border-white/5 bg-zinc-50/60 dark:bg-white/[0.02] space-y-2.5">
              <div className="relative">
                <Search className="w-3.5 h-3.5 text-zinc-400 absolute left-3 top-1/2 -translate-y-1/2" />
                <input
                  value={query}
                  onChange={(e) => setQuery(e.target.value)}
                  type="text"
                  placeholder="按节点名称、协议或服务器地址搜索..."
                  className="hero-input pl-8 pr-8 py-2 text-xs text-zinc-800 dark:text-zinc-200"
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
                    全部 ({nodes.length})
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

            <div className="p-4 overflow-y-auto divide-y divide-zinc-100 dark:divide-white/5 font-mono text-xs flex-1">
              {loading ? (
                <div className="space-y-2.5 py-2">
                  {[...Array(6)].map((_, i) => (
                    <div key={i} className="h-9 rounded-lg bg-zinc-100 dark:bg-white/5 animate-pulse" />
                  ))}
                </div>
              ) : filtered.length === 0 ? (
                <div className="py-12 text-center text-zinc-400 text-xs font-sans">
                  {query ? '未匹配到符合过滤条件的节点。' : emptyText}
                </div>
              ) : (
                filtered.map((node, index) => (
                  <div
                    key={index}
                    className="py-2.5 flex items-center justify-between gap-3 text-xs hover:bg-zinc-50/80 dark:hover:bg-white/[0.03] px-2 rounded-lg transition-colors group"
                  >
                    <div className="flex items-center gap-2.5 min-w-0">
                      <span className="w-7 text-left font-mono text-[10px] text-zinc-400 shrink-0 tabular-nums">
                        #{index + 1}
                      </span>
                      <span
                        className="font-medium text-zinc-800 dark:text-zinc-200 truncate font-sans max-w-xs sm:max-w-md"
                        title={node.name}
                      >
                        {node.name}
                      </span>
                    </div>
                    <div className="flex items-center gap-2 shrink-0">
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
                          onClick={() => onCopy(node.raw_uri, `节点 [${node.name}] 链接已复制`)}
                          title="复制节点分享链接"
                          className="p-1 text-zinc-400 hover:text-[#2563EB] dark:hover:text-[#60A5FA] opacity-60 group-hover:opacity-100 transition-opacity"
                        >
                          <Copy className="w-3.5 h-3.5" />
                        </button>
                      )}
                    </div>
                  </div>
                ))
              )}
            </div>

            <div className="px-6 py-3 border-t border-zinc-100 dark:border-white/5 flex items-center justify-between text-xs text-zinc-400 tabular-nums">
              <span>
                共显示 {filtered.length} / {nodes.length} 个节点
              </span>
              <button
                type="button"
                onClick={onClose}
                className="hero-btn-bordered px-4 py-1.5 text-xs font-medium"
              >
                关闭
              </button>
            </div>
          </motion.div>
        </div>
      )}
    </AnimatePresence>
  )
}
