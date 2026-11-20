import React, { useState, useEffect, useMemo, useCallback } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import {
  Rss,
  FolderDown,
  Plus,
  RefreshCw,
  Eye,
  Pencil,
  Trash2,
  Copy,
  Search,
  X
} from 'lucide-react'
import { api } from '../api'
import { Toast, useToast } from '../components/Toast'
import ConfirmDialog from '../components/ConfirmDialog'
import NodesModal from '../components/NodesModal'
import { CardGridSkeleton } from '../components/Skeleton'
import { formatUserInfo } from '../utils/subUrl'

export default function Subscriptions() {
  const [subscriptions, setSubscriptions] = useState([])
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [refreshingId, setRefreshingId] = useState('')
  const [isRefreshingAll, setIsRefreshingAll] = useState(false)
  const { toast, showToast } = useToast()

  const [currentTab, setCurrentTab] = useState('')
  const [searchQuery, setSearchQuery] = useState('')

  // Add/Edit Modal
  const [showModal, setShowModal] = useState(false)
  const [isEditing, setIsEditing] = useState(false)
  const [editingId, setEditingId] = useState('')
  const [form, setForm] = useState({
    name: '',
    type: 'remote',
    url: '',
    raw_content: '',
    user_agent: 'ClashMeta/v1.18.0 (SubDeck)',
    insecure_skip_verify: false,
    auto_update: true,
    interval_hours: 12
  })

  // Nodes inspection modal
  const [nodesModal, setNodesModal] = useState({ open: false, title: '', nodes: [] })

  // Delete confirm dialog
  const [deleteTarget, setDeleteTarget] = useState(null)
  const [deleting, setDeleting] = useState(false)

  const copyText = async (text, msg) => {
    try {
      await navigator.clipboard.writeText(text)
      showToast(msg || '已复制到剪贴板！')
    } catch {
      showToast('复制失败，请手动长按复制', 'error')
    }
  }

  const maskUrl = (url) => {
    if (!url) return ''
    try {
      const u = new URL(url)
      return `${u.host}${u.pathname.slice(0, 14)}${u.pathname.length > 14 ? '...' : ''}`
    } catch {
      return url.slice(0, 28) + '...'
    }
  }

  const formatTime = (t) => {
    if (!t || t.startsWith('0001')) return '未同步'
    const date = new Date(t)
    const now = new Date()
    const diffSec = Math.floor((now - date) / 1000)
    if (diffSec < 60) return '刚刚'
    if (diffSec < 3600) return `${Math.floor(diffSec / 60)} 分钟前`
    if (diffSec < 86400) return `${Math.floor(diffSec / 3600)} 小时前`
    return date.toLocaleDateString('zh-CN', { month: '2-digit', day: '2-digit' })
  }

  const remoteCount = useMemo(() => subscriptions.filter((s) => s.type === 'remote').length, [subscriptions])
  const localCount = useMemo(() => subscriptions.filter((s) => s.type === 'local').length, [subscriptions])

  const filteredSubscriptions = useMemo(() => {
    return subscriptions.filter((s) => {
      const matchTab = !currentTab || s.type === currentTab
      const q = searchQuery.trim().toLowerCase()
      const matchQuery =
        !q || s.name.toLowerCase().includes(q) || (s.url && s.url.toLowerCase().includes(q))
      return matchTab && matchQuery
    })
  }, [subscriptions, currentTab, searchQuery])

  const localRecognizedCount = useMemo(() => {
    if (!form.raw_content) return 0
    const lines = form.raw_content
      .split('\n')
      .map((l) => l.trim())
      .filter(Boolean)
    let count = 0
    for (const line of lines) {
      if (/^(vmess|vless|ss|trojan|anytls|hysteria2|hy2|tuic|wireguard|wg|snell):\/\//i.test(line)) {
        count++
      }
    }
    return count > 0 ? count : lines.length > 0 ? '已输入 (Base64/多行)' : 0
  }, [form.raw_content])

  const loadData = useCallback(async () => {
    try {
      const res = await api.getSubscriptions()
      setSubscriptions(Array.isArray(res) ? res : (res?.data || []))
    } catch (err) {
      showToast(err.message || '加载订阅列表失败', 'error')
    } finally {
      setLoading(false)
    }
  }, [showToast])

  useEffect(() => {
    loadData()
  }, [loadData])

  const openModal = (item = null) => {
    if (item) {
      setIsEditing(true)
      setEditingId(item.id)
      setForm({
        name: item.name,
        type: item.type,
        url: item.url || '',
        raw_content: item.raw_content || '',
        user_agent: item.user_agent || 'ClashMeta/v1.18.0 (SubDeck)',
        insecure_skip_verify: !!item.insecure_skip_verify,
        auto_update: item.auto_update,
        interval_hours: item.interval_hours || 12
      })
    } else {
      setIsEditing(false)
      setEditingId('')
      setForm({
        name: '',
        type: currentTab === 'local' ? 'local' : 'remote',
        url: '',
        raw_content: '',
        user_agent: 'ClashMeta/v1.18.0 (SubDeck)',
        insecure_skip_verify: false,
        auto_update: true,
        interval_hours: 12
      })
    }
    setShowModal(true)
  }

  const handleSubmit = async (e) => {
    e.preventDefault()
    setSaving(true)
    try {
      if (isEditing) {
        await api.updateSubscription(editingId, form)
        showToast('订阅源更新成功')
      } else {
        await api.createSubscription(form)
        showToast(form.type === 'local' ? '本地订阅创建成功' : '远程订阅添加成功，后台正在抓取节点')
      }
      setShowModal(false)
      await loadData()
    } catch (err) {
      showToast(err.message || '保存失败', 'error')
    } finally {
      setSaving(false)
    }
  }

  const handleRefresh = async (id) => {
    setRefreshingId(id)
    try {
      const res = await api.refreshSubscription(id)
      showToast(`刷新成功，已拉取 ${res.node_count} 个节点`)
      await loadData()
    } catch (err) {
      showToast(err.message || '刷新失败', 'error')
      await loadData()
    } finally {
      setRefreshingId('')
    }
  }

  const refreshAllRemote = async () => {
    const remotes = subscriptions.filter((s) => s.type === 'remote')
    if (remotes.length === 0) return
    setIsRefreshingAll(true)
    let successCount = 0
    for (const r of remotes) {
      try {
        await api.refreshSubscription(r.id)
        successCount++
      } catch (e) {
        console.error(e)
      }
    }
    showToast(`全量拉取完成 (成功 ${successCount}/${remotes.length})`)
    setIsRefreshingAll(false)
    await loadData()
  }

  const openNodes = async (sub) => {
    try {
      const nodes = await api.getSubscriptionNodes(sub.id)
      setNodesModal({ open: true, title: `${sub.name} 解析节点`, nodes: nodes || [] })
    } catch (err) {
      showToast(err.message || '获取节点失败', 'error')
    }
  }

  const confirmDelete = async () => {
    if (!deleteTarget) return
    setDeleting(true)
    try {
      await api.deleteSubscription(deleteTarget.id)
      showToast('订阅源已删除')
      setDeleteTarget(null)
      await loadData()
    } catch (err) {
      showToast(err.message || '删除失败', 'error')
    } finally {
      setDeleting(false)
    }
  }

  return (
    <div className="space-y-6 max-w-7xl mx-auto">
      <Toast toast={toast} />

      {/* Header */}
      <div className="hero-card flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4 p-5">
        <div className="flex items-center gap-3">
          <h1 className="text-lg sm:text-xl font-bold tracking-tight text-zinc-900 dark:text-white">
            订阅源管理
          </h1>
          <span className="hero-chip-default text-xs font-medium tabular-nums">
            共 <span className="font-mono font-bold text-zinc-700 dark:text-zinc-200 mx-0.5">{subscriptions.length}</span> 个源
          </span>
        </div>
        <div className="flex items-center gap-2.5 shrink-0">
          {remoteCount > 0 && (
            <button
              onClick={refreshAllRemote}
              disabled={isRefreshingAll}
              className="hero-btn-bordered px-3.5 py-2 text-xs font-semibold gap-1.5 disabled:opacity-50"
            >
              <RefreshCw
                className={`w-3.5 h-3.5 text-[#2563EB] dark:text-[#60A5FA] ${isRefreshingAll ? 'animate-spin' : ''}`}
              />
              <span>全量拉取更新</span>
            </button>
          )}
          <button
            onClick={() => openModal()}
            className="hero-btn-primary px-4 py-2 text-xs font-semibold gap-1.5"
          >
            <Plus className="w-4 h-4" />
            <span>添加订阅源</span>
          </button>
        </div>
      </div>

      {/* Filter & Search Toolbar */}
      <div className="hero-card flex flex-col sm:flex-row sm:items-center justify-between gap-3 p-2">
        <div className="flex p-1 bg-zinc-100 dark:bg-white/[0.04] rounded-xl gap-1">
          {[
            { key: '', label: '全部订阅', count: subscriptions.length },
            { key: 'remote', label: '远程订阅', count: remoteCount },
            { key: 'local', label: '本地订阅', count: localCount }
          ].map((tab) => (
            <button
              key={tab.key}
              onClick={() => setCurrentTab(tab.key)}
              className={`flex items-center gap-1.5 px-3.5 py-1.5 rounded-lg text-xs font-semibold transition-all ${
                currentTab === tab.key
                  ? 'bg-white dark:bg-[#1E2330] text-zinc-900 dark:text-white shadow-xs'
                  : 'text-zinc-500 hover:text-zinc-800 dark:hover:text-zinc-200'
              }`}
            >
              <span>{tab.label}</span>
              <span
                className={`px-1.5 py-0.2 rounded-full text-[10px] font-mono tabular-nums ${
                  currentTab === tab.key
                    ? 'bg-[#2563EB]/12 text-[#2563EB] dark:text-[#60A5FA]'
                    : 'bg-zinc-200/70 dark:bg-white/10 text-zinc-500'
                }`}
              >
                {tab.count}
              </span>
            </button>
          ))}
        </div>

        <div className="relative w-full sm:w-72">
          <Search className="w-4 h-4 text-zinc-400 absolute left-3 top-1/2 -translate-y-1/2" />
          <input
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            type="text"
            placeholder="搜索订阅名称或网址..."
            className="hero-input pl-9 pr-8 py-2 text-xs text-zinc-800 dark:text-zinc-200"
          />
          {searchQuery && (
            <button
              onClick={() => setSearchQuery('')}
              className="absolute right-2.5 top-1/2 -translate-y-1/2 text-zinc-400 hover:text-zinc-600 dark:hover:text-zinc-200"
            >
              <X className="w-3.5 h-3.5" />
            </button>
          )}
        </div>
      </div>

      {/* Loading / Empty / Grid */}
      {loading ? (
        <CardGridSkeleton count={6} />
      ) : filteredSubscriptions.length === 0 ? (
        <div className="p-16 hero-card text-center space-y-3">
          <div className="w-12 h-12 rounded-2xl bg-zinc-100 dark:bg-white/5 flex items-center justify-center text-zinc-400 mx-auto">
            <Rss className="w-6 h-6" />
          </div>
          <div>
            <h3 className="text-sm font-bold text-zinc-900 dark:text-white">暂无匹配的订阅源</h3>
            <p className="text-xs text-zinc-400 max-w-sm mx-auto mt-1 leading-relaxed">
              可随时点击下方按钮，添加机场远程订阅链接，或批量导入多行自建节点。
            </p>
          </div>
          <button
            onClick={() => openModal()}
            className="hero-btn-primary px-4 py-2 text-xs font-semibold"
          >
            立即添加
          </button>
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {filteredSubscriptions.map((sub) => {
            const info = formatUserInfo(sub.user_info)
            return (
              <motion.div
                key={sub.id}
                layout
                initial={{ opacity: 0, y: 8 }}
                animate={{ opacity: 1, y: 0 }}
                className="hero-card hero-card-interactive p-5 flex flex-col justify-between space-y-4"
              >
                <div className="space-y-3">
                  <div className="flex items-start justify-between gap-3">
                    <div className="space-y-1.5 min-w-0">
                      <div className="flex items-center gap-2">
                        <span
                          className={`text-[10px] font-bold px-2 py-0.5 rounded-md border uppercase shrink-0 ${
                            sub.type === 'local'
                              ? 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border-emerald-500/20'
                              : 'bg-[#2563EB]/10 text-[#2563EB] dark:text-[#60A5FA] border-[#2563EB]/20'
                          }`}
                        >
                          {sub.type === 'local' ? '本地' : '远程'}
                        </span>
                        <h3 className="font-bold text-sm text-zinc-900 dark:text-white truncate" title={sub.name}>
                          {sub.name}
                        </h3>
                      </div>

                      <div className="text-[11px] text-zinc-400 font-mono truncate flex items-center gap-1.5">
                        <span className="truncate">
                          {sub.type === 'remote' ? maskUrl(sub.url) : '自建节点批量导入'}
                        </span>
                        {sub.type === 'remote' && (
                          <button
                            onClick={() => copyText(sub.url, '原始订阅链接已复制')}
                            title="复制原始链接"
                            className="hover:text-zinc-700 dark:hover:text-zinc-200 transition-colors shrink-0"
                          >
                            <Copy className="w-3 h-3" />
                          </button>
                        )}
                      </div>
                    </div>

                    <div className="flex items-center gap-1 shrink-0">
                      {sub.type === 'remote' && (
                        <button
                          onClick={() => handleRefresh(sub.id)}
                          disabled={refreshingId === sub.id}
                          title="拉取最新节点"
                          className="hero-btn-default p-1.5 rounded-lg text-zinc-400 hover:text-[#2563EB] dark:hover:text-[#60A5FA] disabled:opacity-50"
                        >
                          <RefreshCw className={`w-3.5 h-3.5 ${refreshingId === sub.id ? 'animate-spin' : ''}`} />
                        </button>
                      )}
                      <button
                        onClick={() => openNodes(sub)}
                        title="查看已解析节点"
                        className="hero-btn-default p-1.5 rounded-lg text-zinc-400 hover:text-zinc-900 dark:hover:text-white"
                      >
                        <Eye className="w-3.5 h-3.5" />
                      </button>
                      <button
                        onClick={() => openModal(sub)}
                        title="编辑订阅配置"
                        className="hero-btn-default p-1.5 rounded-lg text-zinc-400 hover:text-zinc-900 dark:hover:text-white"
                      >
                        <Pencil className="w-3.5 h-3.5" />
                      </button>
                      <button
                        onClick={() => setDeleteTarget(sub)}
                        title="删除该订阅"
                        className="hero-btn-default p-1.5 rounded-lg text-zinc-400 hover:text-rose-600 hover:bg-rose-500/10"
                      >
                        <Trash2 className="w-3.5 h-3.5" />
                      </button>
                    </div>
                  </div>

                  {info && (
                    <div className="px-2.5 py-1.5 rounded-lg bg-zinc-50 dark:bg-white/[0.03] border border-zinc-200/60 dark:border-white/[0.06] flex items-center justify-between text-[11px] text-zinc-500 dark:text-zinc-400 font-mono tabular-nums">
                      <span>{info.trafficText || '流量未知'}</span>
                      {info.expireText && <span>到期 {info.expireText}</span>}
                    </div>
                  )}

                  {sub.status === 'error' && sub.error_msg && (
                    <div className="p-2.5 rounded-xl bg-rose-500/10 border border-rose-500/20 text-[11px] text-rose-600 dark:text-rose-400 break-all leading-relaxed">
                      拉取异常: {sub.error_msg}
                    </div>
                  )}
                </div>

                <div className="pt-2.5 border-t border-zinc-100 dark:border-white/[0.06] flex items-center justify-between text-xs tabular-nums">
                  <div className="flex items-center gap-1.5">
                    <span className="text-zinc-400">有效节点</span>
                    <span className="font-bold font-mono text-zinc-900 dark:text-white">{sub.node_count || 0}</span>
                  </div>
                  <div className="flex items-center gap-1.5 font-mono text-zinc-400 text-[11px]">
                    <span
                      className={`w-1.5 h-1.5 rounded-full ${
                        sub.status === 'error' ? 'bg-rose-500' : 'bg-emerald-500'
                      }`}
                    />
                    <span>{formatTime(sub.last_updated)}</span>
                  </div>
                </div>
              </motion.div>
            )
          })}
        </div>
      )}

      {/* Add/Edit Modal */}
      <AnimatePresence>
        {showModal && (
          <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
            <motion.div
              initial={{ opacity: 0 }}
              animate={{ opacity: 1 }}
              exit={{ opacity: 0 }}
              className="fixed inset-0 bg-black/55 backdrop-blur-sm"
              onClick={() => setShowModal(false)}
            />
            <motion.div
              initial={{ opacity: 0, scale: 0.96, y: 8 }}
              animate={{ opacity: 1, scale: 1, y: 0 }}
              exit={{ opacity: 0, scale: 0.96, y: 8 }}
              transition={{ duration: 0.16, ease: [0.16, 1, 0.3, 1] }}
              className="relative w-full max-w-lg bg-white dark:bg-[#11141B] rounded-2xl shadow-2xl border border-zinc-200/80 dark:border-white/10 overflow-hidden z-10"
            >
              <div className="px-6 py-4 border-b border-zinc-100 dark:border-white/[0.06] flex items-center justify-between">
                <h2 className="text-base font-bold text-zinc-900 dark:text-white">
                  {isEditing ? '编辑订阅源' : '添加订阅源'}
                </h2>
                <button
                  onClick={() => setShowModal(false)}
                  className="p-1 rounded-lg text-zinc-400 hover:text-zinc-600 dark:hover:text-white"
                >
                  <X className="w-4 h-4" />
                </button>
              </div>

              <form onSubmit={handleSubmit} className="p-6 space-y-4">
                {!isEditing && (
                  <div className="space-y-1.5">
                    <label className="block text-[11px] font-semibold text-zinc-500 uppercase tracking-wider">
                      订阅类型
                    </label>
                    <div className="grid grid-cols-2 gap-1.5 p-1 bg-zinc-100 dark:bg-white/[0.04] rounded-xl">
                      <button
                        type="button"
                        onClick={() => setForm((f) => ({ ...f, type: 'remote' }))}
                        className={`py-2 text-xs font-bold rounded-lg transition-all flex items-center justify-center gap-2 ${
                          form.type === 'remote'
                            ? 'bg-white dark:bg-[#1E2330] text-[#2563EB] dark:text-[#60A5FA] shadow-xs'
                            : 'text-zinc-500'
                        }`}
                      >
                        <Rss className="w-3.5 h-3.5" />
                        <span>远程订阅 (URL)</span>
                      </button>
                      <button
                        type="button"
                        onClick={() => setForm((f) => ({ ...f, type: 'local' }))}
                        className={`py-2 text-xs font-bold rounded-lg transition-all flex items-center justify-center gap-2 ${
                          form.type === 'local'
                            ? 'bg-white dark:bg-[#1E2330] text-emerald-600 dark:text-emerald-400 shadow-xs'
                            : 'text-zinc-500'
                        }`}
                      >
                        <FolderDown className="w-3.5 h-3.5" />
                        <span>本地订阅 (批量导入)</span>
                      </button>
                    </div>
                  </div>
                )}

                <div>
                  <label className="block text-[11px] font-semibold text-zinc-500 uppercase tracking-wider mb-1.5">
                    订阅名称
                  </label>
                  <input
                    value={form.name}
                    onChange={(e) => setForm((f) => ({ ...f, name: e.target.value }))}
                    type="text"
                    required
                    placeholder={form.type === 'local' ? '例如：自建专线节点' : '例如：机场A-主线'}
                    className="hero-input px-3.5 py-2.5 text-xs text-zinc-900 dark:text-white"
                  />
                </div>

                {form.type === 'remote' ? (
                  <>
                    <div>
                      <label className="block text-[11px] font-semibold text-zinc-500 uppercase tracking-wider mb-1.5">
                        远程订阅链接 (URL)
                      </label>
                      <input
                        value={form.url}
                        onChange={(e) => setForm((f) => ({ ...f, url: e.target.value }))}
                        type="url"
                        required
                        placeholder="https://example.com/api/v1/client/subscribe?token=..."
                        className="hero-input px-3.5 py-2.5 text-xs font-mono text-zinc-900 dark:text-white"
                      />
                    </div>

                    <div>
                      <label className="block text-[11px] font-semibold text-zinc-500 uppercase tracking-wider mb-1.5">
                        自定义 User-Agent
                      </label>
                      <input
                        value={form.user_agent}
                        onChange={(e) => setForm((f) => ({ ...f, user_agent: e.target.value }))}
                        type="text"
                        placeholder="ClashMeta/v1.18.0 (SubDeck)"
                        className="hero-input px-3.5 py-2.5 text-xs font-mono text-zinc-900 dark:text-white"
                      />
                    </div>

                    <div className="grid grid-cols-2 gap-4 pt-1">
                      <div>
                        <label className="block text-[11px] font-semibold text-zinc-500 uppercase tracking-wider mb-1.5">
                          更新周期 (小时)
                        </label>
                        <input
                          value={form.interval_hours}
                          onChange={(e) => setForm((f) => ({ ...f, interval_hours: parseInt(e.target.value) || 12 }))}
                          type="number"
                          min="1"
                          max="720"
                          className="hero-input px-3.5 py-2 text-xs font-mono text-zinc-900 dark:text-white"
                        />
                      </div>
                      <div className="flex items-center pt-5">
                        <label className="flex items-center gap-2.5 cursor-pointer">
                          <input
                            type="checkbox"
                            checked={form.auto_update}
                            onChange={(e) => setForm((f) => ({ ...f, auto_update: e.target.checked }))}
                            className="rounded text-[#2563EB] focus:ring-[#2563EB] w-4 h-4"
                          />
                          <span className="text-xs text-zinc-700 dark:text-zinc-300 font-medium">后台自动定时更新</span>
                        </label>
                      </div>
                    </div>

                    <div className="pt-1">
                      <label className="flex items-center gap-2.5 cursor-pointer">
                        <input
                          type="checkbox"
                          checked={form.insecure_skip_verify}
                          onChange={(e) => setForm((f) => ({ ...f, insecure_skip_verify: e.target.checked }))}
                          className="rounded text-[#2563EB] focus:ring-[#2563EB] w-4 h-4"
                        />
                        <span className="text-xs text-zinc-600 dark:text-zinc-400">
                          跳过 TLS 证书校验（仅自签证书机场需要）
                        </span>
                      </label>
                    </div>
                  </>
                ) : (
                  <div>
                    <div className="flex items-center justify-between mb-1.5">
                      <label className="block text-[11px] font-semibold text-zinc-500 uppercase tracking-wider">
                        批量粘贴节点链接（一行一条）
                      </label>
                      <div className="flex items-center gap-2">
                        <button
                          type="button"
                          onClick={() =>
                            setForm((f) => ({
                              ...f,
                              raw_content:
                                'hysteria2://passwd@hk-01.example.com:443?sni=hk.example.com#🇭🇰 香港01-专线\nvmess://eyJ2IjoiMiIsInBzIjoi8J+Hr/Cfh7Ug5pel5pysMDEtQkdQIiwiYWRkIjoianAtMDEuZXhhbXBsZS5jb20iLCJwb3J0IjoiNDQzIiwiaWQiOiI5OTk5OTk5OS05OTk5LTk5OTktOTk5OS05OTk5OTk5OTk5OTkiLCJhaWQiOiIwIiwic2N5IjoiYXV0byIsIm5ldCI6IndzIiwidHlwZSI6Im5vbmUiLCJob3N0IjoiIiwicGF0aCI6Ii8iLCJ0bHMiOiJ0bHMiLCJzbmkiOiIifQ==\nss://YWVzLTI1Ni1nY206cGFzc3dvcmRAMTI3LjAuMC4xOjEwODAy#🇸🇬 新加坡01-Direct'
                            }))
                          }
                          className="text-[11px] text-[#2563EB] dark:text-[#60A5FA] font-semibold hover:underline"
                        >
                          填入演示样例
                        </button>
                        <span className="text-zinc-300 dark:text-zinc-600">|</span>
                        <span className="hero-chip-success text-[10px] font-mono font-medium tabular-nums">
                          已识别 {localRecognizedCount} 个节点
                        </span>
                      </div>
                    </div>
                    <textarea
                      value={form.raw_content}
                      onChange={(e) => setForm((f) => ({ ...f, raw_content: e.target.value }))}
                      rows={7}
                      required
                      placeholder={`支持粘贴 vmess://, vless://, ss://, trojan://, anytls://, hysteria2://, tuic://, wireguard://, snell:// 格式...\n也可以直接粘贴包含多个节点的 Base64 订阅字符串`}
                      className="hero-input p-3 text-xs font-mono text-zinc-900 dark:text-white"
                    />
                  </div>
                )}

                <div className="pt-3 flex justify-end gap-2.5 border-t border-zinc-100 dark:border-white/[0.06]">
                  <button
                    type="button"
                    onClick={() => setShowModal(false)}
                    className="hero-btn-bordered px-4 py-2 text-xs font-medium"
                  >
                    取消
                  </button>
                  <button
                    type="submit"
                    disabled={saving}
                    className="hero-btn-primary px-5 py-2 text-xs font-semibold disabled:opacity-50"
                  >
                    {saving ? '保存中...' : '确定保存'}
                  </button>
                </div>
              </form>
            </motion.div>
          </div>
        )}
      </AnimatePresence>

      <NodesModal
        open={nodesModal.open}
        title={nodesModal.title}
        nodes={nodesModal.nodes}
        onClose={() => setNodesModal({ open: false, title: '', nodes: [] })}
        onCopy={copyText}
      />

      <ConfirmDialog
        open={!!deleteTarget}
        title="确认删除订阅源？"
        description={deleteTarget ? `即将删除订阅源 "${deleteTarget.name}"，该操作不可撤销。` : ''}
        confirmText="确认删除"
        loading={deleting}
        onConfirm={confirmDelete}
        onCancel={() => setDeleteTarget(null)}
      />
    </div>
  )
}
