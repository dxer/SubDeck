import React, { useState, useEffect, useMemo, useCallback } from 'react'
import { motion } from 'framer-motion'
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
  X,
  LayoutGrid,
  List as ListIcon,
  Upload
} from 'lucide-react'
import { api } from '../api'
import { useI18n } from '../i18n'
import { Toast, useToast } from '../components/Toast'
import ConfirmDialog from '../components/ConfirmDialog'
import NodesModal from '../components/NodesModal'
import Drawer from '../components/Drawer'
import { CardGridSkeleton } from '../components/Skeleton'
import { formatUserInfo, quotaSeverity, copyToClipboard } from '../utils/subUrl'

export default function Subscriptions() {
  const { t, lang } = useI18n()
  const [subscriptions, setSubscriptions] = useState([])
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [refreshingId, setRefreshingId] = useState('')
  const [refreshProgress, setRefreshProgress] = useState(null)
  const { toast, showToast } = useToast()

  const [currentTab, setCurrentTab] = useState('')
  const [searchQuery, setSearchQuery] = useState('')
  const [viewMode, setViewMode] = useState(() => localStorage.getItem('subdeck_sub_view') || 'grid')

  const [showDrawer, setShowDrawer] = useState(false)
  const [isEditing, setIsEditing] = useState(false)
  const [editingId, setEditingId] = useState('')
  const [isDraggingFile, setIsDraggingFile] = useState(false)
  const [form, setForm] = useState({
    name: '',
    type: 'remote',
    urls: '',
    url: '',
    raw_content: '',
    user_agent: 'ClashMeta/v1.18.0 (SubDeck)',
    insecure_skip_verify: false,
    auto_update: true,
    interval_hours: 12,
    fetch_proxy_url: '',
    failure_policy: 'strict',
    fetch_mode: 'merge'
  })

  const [nodesModal, setNodesModal] = useState({ open: false, subId: '', title: '', nodes: [] })
  const [probingModal, setProbingModal] = useState(false)
  const [deleteTarget, setDeleteTarget] = useState(null)
  const [deleting, setDeleting] = useState(false)

  const switchViewMode = (mode) => {
    setViewMode(mode)
    localStorage.setItem('subdeck_sub_view', mode)
  }

  const copyText = async (text, msg) => {
    const ok = await copyToClipboard(text)
    if (ok) {
      showToast(msg || t('common.copied'))
    } else {
      showToast(t('common.copyFailed'), 'error')
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

  const formatTime = (val) => {
    if (!val || val.startsWith('0001')) return t('subscriptions.unsynced')
    const date = new Date(val)
    const diffSec = Math.floor((Date.now() - date.getTime()) / 1000)
    if (diffSec < 60) return t('subscriptions.justNow')
    if (diffSec < 3600) return t('subscriptions.minsAgo', { n: Math.floor(diffSec / 60) })
    if (diffSec < 86400) return t('subscriptions.hoursAgo', { n: Math.floor(diffSec / 3600) })
    return date.toLocaleDateString(lang === 'en' ? 'en-US' : 'zh-CN', { month: '2-digit', day: '2-digit' })
  }

  // Quota health helpers: traffic bar colors and an expiry chip shown only when
  // the plan is close to expiring (<=7d) or already expired.
  const quotaBarColor = (sev) =>
    sev === 'danger' ? 'bg-rose-500' : sev === 'warn' ? 'bg-amber-500' : 'bg-emerald-500'

  const expiryChip = (quota) => {
    if (!quota || quota.expire == null) return null
    const sev = quotaSeverity(quota).expiry
    if (sev === 'ok') return null
    const cls =
      sev === 'warn'
        ? 'bg-amber-500/10 text-amber-600 dark:text-amber-400 border-amber-500/20'
        : 'bg-rose-500/10 text-rose-600 dark:text-rose-400 border-rose-500/20'
    const text = quota.expired
      ? t('subscriptions.quotaExpired')
      : quota.daysLeft === 0
        ? t('subscriptions.quotaToday')
        : t('subscriptions.quotaExpireDays', { days: quota.daysLeft })
    return (
      <span className={`px-1.5 py-0.5 rounded-full border text-[10px] font-mono font-semibold tabular-nums shrink-0 ${cls}`}>
        {text}
      </span>
    )
  }

  const remoteCount = useMemo(() => subscriptions.filter((s) => s.type === 'remote').length, [subscriptions])
  const localCount = useMemo(() => subscriptions.filter((s) => s.type === 'local').length, [subscriptions])

  const filteredSubscriptions = useMemo(() => {
    const q = searchQuery.trim().toLowerCase()
    return subscriptions
      .filter((s) => {
        const matchTab = !currentTab || s.type === currentTab
        const urlsText = Array.isArray(s.urls) && s.urls.length > 0 ? s.urls.join(' ') : (s.url || '')
        const matchQuery = !q || s.name.toLowerCase().includes(q) || urlsText.toLowerCase().includes(q)
        return matchTab && matchQuery
      })
      .sort((a, b) => {
        const aErr = a.status === 'error' ? 0 : 1
        const bErr = b.status === 'error' ? 0 : 1
        return aErr - bErr
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
    return count > 0 ? count : lines.length
  }, [form.raw_content])

  const loadData = useCallback(async () => {
    try {
      const res = await api.getSubscriptions()
      setSubscriptions(Array.isArray(res) ? res : (res?.data || []))
    } catch (err) {
      showToast(err.message || 'Load failed', 'error')
    } finally {
      setLoading(false)
    }
  }, [showToast])

  useEffect(() => {
    loadData()
  }, [loadData])

  const openDrawer = (item = null) => {
    if (item) {
      setIsEditing(true)
      setEditingId(item.id)
      const urls = Array.isArray(item.urls) && item.urls.length > 0 ? item.urls : (item.url ? [item.url] : [])
      setForm({
        name: item.name,
        type: item.type,
        urls: urls.join('\n'),
        url: item.url || '',
        raw_content: item.raw_content || '',
        user_agent: item.user_agent || 'ClashMeta/v1.18.0 (SubDeck)',
        insecure_skip_verify: !!item.insecure_skip_verify,
        auto_update: item.auto_update,
        interval_hours: item.interval_hours || 12,
        fetch_proxy_url: item.fetch_proxy_url || '',
        failure_policy: item.failure_policy || 'strict',
        fetch_mode: item.fetch_mode || 'merge'
      })
    } else {
      setIsEditing(false)
      setEditingId('')
      setForm({
        name: '',
        type: currentTab === 'local' ? 'local' : 'remote',
        urls: '',
        url: '',
        raw_content: '',
        user_agent: 'ClashMeta/v1.18.0 (SubDeck)',
        insecure_skip_verify: false,
        auto_update: true,
        interval_hours: 12,
        fetch_proxy_url: '',
        failure_policy: 'strict',
        fetch_mode: 'merge'
      })
    }
    setIsDraggingFile(false)
    setShowDrawer(true)
  }

  const handleFileDrop = async (e) => {
    e.preventDefault()
    setIsDraggingFile(false)
    const file = e.dataTransfer?.files?.[0]
    if (!file) return
    if (file.type && !file.type.startsWith('text') && !/\.(txt|ya?ml|conf|json)$/i.test(file.name)) {
      showToast(t('subscriptions.nonTextError'), 'error')
      return
    }
    try {
      const text = await file.text()
      setForm((f) => ({
        ...f,
        raw_content: f.raw_content ? `${f.raw_content.trimEnd()}\n${text.trim()}` : text.trim()
      }))
      showToast(t('subscriptions.fileLoaded', { name: file.name }))
    } catch {
      showToast(t('subscriptions.nonTextError'), 'error')
    }
  }

  const handleSubmit = async (e) => {
    e.preventDefault()
    const urlList = (form.type === 'remote' ? form.urls : '')
      .split('\n')
      .map((l) => l.trim())
      .filter(Boolean)
    if (form.type === 'remote' && urlList.length === 0) {
      showToast(t('subscriptions.needUrl'), 'error')
      return
    }
    if (form.type === 'remote' && urlList.length > 10) {
      showToast(t('subscriptions.tooManyUrls'), 'error')
      return
    }
    setSaving(true)
    try {
      const payload = {
        ...form,
        urls: urlList,
        url: urlList[0] || ''
      }
      if (isEditing) {
        await api.updateSubscription(editingId, payload)
        showToast(t('subscriptions.saveUpdated'))
      } else {
        await api.createSubscription(payload)
        showToast(
          form.type === 'local'
            ? t('subscriptions.saveCreatedLocal')
            : t('subscriptions.saveCreatedRemote')
        )
      }
      setShowDrawer(false)
      await loadData()
    } catch (err) {
      showToast(err.message || 'Save failed', 'error')
    } finally {
      setSaving(false)
    }
  }

  const handleRefresh = async (id) => {
    setRefreshingId(id)
    try {
      const res = await api.refreshSubscription(id)
      showToast(t('subscriptions.refreshOk', { count: res.node_count }))
      await loadData()
    } catch (err) {
      showToast(err.message || t('subscriptions.refreshFailed'), 'error')
      await loadData()
    } finally {
      setRefreshingId('')
    }
  }

  const refreshAllRemote = async () => {
    const remotes = subscriptions.filter((s) => s.type === 'remote')
    if (remotes.length === 0) return

    setRefreshProgress({ done: 0, total: remotes.length })
    let successCount = 0
    let doneCount = 0
    let cursor = 0
    const concurrency = Math.min(3, remotes.length)

    const worker = async () => {
      while (cursor < remotes.length) {
        const sub = remotes[cursor++]
        try {
          await api.refreshSubscription(sub.id)
          successCount++
        } catch (e) {
          console.error(e)
        } finally {
          doneCount++
          setRefreshProgress({ done: doneCount, total: remotes.length })
        }
      }
    }

    await Promise.all(Array.from({ length: concurrency }, () => worker()))
    showToast(t('subscriptions.syncDoneToast', { ok: successCount, total: remotes.length }))
    setRefreshProgress(null)
    await loadData()
  }

  const openNodes = async (sub) => {
    try {
      const nodes = await api.getSubscriptionNodes(sub.id)
      setNodesModal({
        open: true,
        subId: sub.id,
        title: t('subscriptions.modalNodesTitle', { name: sub.name }),
        nodes: nodes || []
      })
    } catch (err) {
      showToast(err.message || 'Load nodes failed', 'error')
    }
  }

  const handleProbeNodes = async () => {
    if (!nodesModal.subId) return
    setProbingModal(true)
    try {
      const res = await api.probeSubscription(nodesModal.subId)
      const updatedNodes = Array.isArray(res?.nodes) ? res.nodes : []
      setNodesModal((prev) => ({ ...prev, nodes: updatedNodes }))
      showToast(t('subscriptions.probeOk', { healthy: res?.healthy ?? 0, failed: res?.failed ?? 0 }))
    } catch (err) {
      showToast(err.message || t('subscriptions.probeFailed'), 'error')
    } finally {
      setProbingModal(false)
    }
  }

  const confirmDelete = async () => {
    if (!deleteTarget) return
    setDeleting(true)
    try {
      await api.deleteSubscription(deleteTarget.id)
      showToast(t('subscriptions.deleted'))
      setDeleteTarget(null)
      await loadData()
    } catch (err) {
      showToast(err.message || 'Delete failed', 'error')
    } finally {
      setDeleting(false)
    }
  }

  const subUrlCount = (sub) => (Array.isArray(sub.urls) && sub.urls.length > 0 ? sub.urls.length : (sub.url ? 1 : 0))

  return (
    <div className="space-y-4 sm:space-y-5 max-w-7xl mx-auto">
      <Toast toast={toast} />

      {/* Header */}
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3 pb-1">
        <div className="flex items-center gap-2.5">
          <h1 className="text-xl sm:text-2xl font-bold tracking-tight text-zinc-900 dark:text-white">
            {t('subscriptions.title')}
          </h1>
          <span className="hero-chip-default text-xs font-medium tabular-nums">
            {t('subscriptions.countBadge', { count: subscriptions.length })}
          </span>
        </div>

        <div className="grid grid-cols-2 sm:flex items-center gap-2 shrink-0">
          {remoteCount > 0 && (
            <button
              onClick={refreshAllRemote}
              disabled={!!refreshProgress}
              className="hero-btn-bordered px-3.5 py-2 text-xs font-semibold gap-1.5 disabled:opacity-60 tabular-nums"
            >
              <RefreshCw
                className={`w-3.5 h-3.5 text-[#2563EB] dark:text-[#60A5FA] ${refreshProgress ? 'animate-spin' : ''}`}
              />
              <span>
                {refreshProgress
                  ? t('subscriptions.syncingProgress', { done: refreshProgress.done, total: refreshProgress.total })
                  : t('subscriptions.syncAll')}
              </span>
            </button>
          )}
          <button
            onClick={() => openDrawer()}
            className={`hero-btn-primary px-4 py-2 text-xs font-semibold gap-1.5 ${
              remoteCount === 0 ? 'col-span-2 sm:col-span-1' : ''
            }`}
          >
            <Plus className="w-4 h-4" />
            <span>{t('subscriptions.addSource')}</span>
          </button>
        </div>
      </div>

      {/* Filter, Search & View Mode Toolbar */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-2.5 sm:gap-3">
        <div className="flex p-1 bg-zinc-200/60 dark:bg-white/[0.04] rounded-xl gap-1 w-full sm:w-fit border border-zinc-200/70 dark:border-white/[0.06] overflow-x-auto">
          {[
            { key: '', label: t('subscriptions.tabAll'), count: subscriptions.length },
            { key: 'remote', label: t('subscriptions.tabRemote'), count: remoteCount },
            { key: 'local', label: t('subscriptions.tabLocal'), count: localCount }
          ].map((tab) => (
            <button
              key={tab.key}
              onClick={() => setCurrentTab(tab.key)}
              className={`flex-1 sm:flex-none flex items-center justify-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-semibold transition-all whitespace-nowrap ${
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
                    : 'bg-zinc-200/80 dark:bg-white/10 text-zinc-500'
                }`}
              >
                {tab.count}
              </span>
            </button>
          ))}
        </div>

        <div className="flex items-center gap-2 w-full sm:w-auto">
          <div className="relative flex-1 sm:w-64">
            <Search className="w-3.5 h-3.5 text-zinc-400 absolute left-3 top-1/2 -translate-y-1/2" />
            <input
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              type="text"
              placeholder={t('subscriptions.searchPlaceholder')}
              className="hero-input pl-8 pr-8 py-2 text-sm sm:text-xs text-zinc-800 dark:text-zinc-200"
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

          <div className="flex p-1 bg-zinc-200/60 dark:bg-white/[0.04] rounded-xl border border-zinc-200/70 dark:border-white/[0.06] shrink-0">
            <button
              type="button"
              onClick={() => switchViewMode('grid')}
              className={`p-1.5 rounded-lg transition-all ${
                viewMode === 'grid'
                  ? 'bg-white dark:bg-[#1E2330] text-zinc-900 dark:text-white shadow-xs'
                  : 'text-zinc-400 hover:text-zinc-700 dark:hover:text-zinc-200'
              }`}
            >
              <LayoutGrid className="w-3.5 h-3.5" />
            </button>
            <button
              type="button"
              onClick={() => switchViewMode('table')}
              className={`p-1.5 rounded-lg transition-all ${
                viewMode === 'table'
                  ? 'bg-white dark:bg-[#1E2330] text-zinc-900 dark:text-white shadow-xs'
                  : 'text-zinc-400 hover:text-zinc-700 dark:hover:text-zinc-200'
              }`}
            >
              <ListIcon className="w-3.5 h-3.5" />
            </button>
          </div>
        </div>
      </div>

      {/* Content */}
      {loading ? (
        <CardGridSkeleton count={6} />
      ) : filteredSubscriptions.length === 0 ? (
        <div className="p-12 sm:p-16 hero-card text-center space-y-3">
          <div className="w-11 h-11 rounded-2xl bg-zinc-100 dark:bg-white/5 flex items-center justify-center text-zinc-400 mx-auto">
            <Rss className="w-5 h-5" />
          </div>
          <h3 className="text-sm font-bold text-zinc-900 dark:text-white">
            {searchQuery ? t('subscriptions.emptySearch') : t('subscriptions.emptyTitle')}
          </h3>
          <button onClick={() => openDrawer()} className="hero-btn-primary px-4 py-2 text-xs font-semibold">
            {t('subscriptions.addNow')}
          </button>
        </div>
      ) : viewMode === 'table' ? (
        <div className="hero-card overflow-hidden">
          <div className="overflow-x-auto">
            <table className="w-full text-left border-collapse text-xs">
              <thead>
                <tr className="border-b border-zinc-100 dark:border-white/[0.06] text-[11px] font-semibold text-zinc-400 uppercase tracking-wider bg-zinc-50/70 dark:bg-white/[0.02]">
                  <th className="py-3 px-4">{t('subscriptions.colSource')}</th>
                  <th className="py-3 px-4 hidden md:table-cell">{t('subscriptions.colUrlTraffic')}</th>
                  <th className="py-3 px-4">{t('subscriptions.colNodes')}</th>
                  <th className="py-3 px-4 hidden sm:table-cell">{t('subscriptions.colStatus')}</th>
                  <th className="py-3 px-4 text-right">{t('subscriptions.colActions')}</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-zinc-100 dark:divide-white/[0.06]">
                {filteredSubscriptions.map((sub) => {
                  const info = formatUserInfo(sub.user_info, lang)
                  return (
                    <tr
                      key={sub.id}
                      className="hover:bg-zinc-50/80 dark:hover:bg-white/[0.02] transition-colors"
                    >
                      <td className="py-3.5 px-4">
                        <div className="flex items-center gap-2">
                          <span
                            className={`text-[10px] font-bold px-1.5 py-0.5 rounded border uppercase shrink-0 ${
                              sub.type === 'local'
                                ? 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border-emerald-500/20'
                                : 'bg-[#2563EB]/10 text-[#2563EB] dark:text-[#60A5FA] border-[#2563EB]/20'
                            }`}
                          >
                            {sub.type === 'local' ? t('dashboard.local') : t('dashboard.remote')}
                          </span>
                          <span className="font-semibold text-zinc-900 dark:text-white truncate max-w-[140px] sm:max-w-[200px]">
                            {sub.name}
                          </span>
                        </div>
                        {sub.status === 'error' && sub.error_msg && (
                          <div className="text-[11px] text-rose-500 mt-1 truncate max-w-[180px] sm:max-w-xs" title={sub.error_msg}>
                            {sub.error_msg}
                          </div>
                        )}
                      </td>
                      <td className="py-3.5 px-4 hidden md:table-cell font-mono text-[11px] text-zinc-400">
                        <div className="flex items-center gap-1.5">
                          <span className="truncate max-w-[220px]">
                            {sub.type === 'remote' ? maskUrl(sub.url) : t('subscriptions.localImportDesc')}
                          </span>
                          {sub.type === 'remote' && subUrlCount(sub) > 1 && (
                            <span className="hero-chip-default text-[9px] font-mono shrink-0 tabular-nums">
                              {t('subscriptions.urlCount', { count: subUrlCount(sub) })}
                            </span>
                          )}
                          {sub.type === 'remote' && (
                            <button
                              onClick={() => copyText(sub.url, t('subscriptions.rawCopied'))}
                              className="hover:text-zinc-700 dark:hover:text-white"
                            >
                              <Copy className="w-3 h-3" />
                            </button>
                          )}
                        </div>
                        {info && (
                          <div className="flex items-center gap-1.5 mt-0.5">
                            <span
                              className={`text-[10px] ${
                                quotaSeverity(info.quota).traffic === 'danger'
                                  ? 'text-rose-600 dark:text-rose-400'
                                  : quotaSeverity(info.quota).traffic === 'warn'
                                    ? 'text-amber-600 dark:text-amber-400'
                                    : 'text-zinc-500'
                              }`}
                            >
                              {info.trafficText}
                            </span>
                            {expiryChip(info.quota)}
                          </div>
                        )}
                      </td>
                      <td className="py-3.5 px-4 font-mono font-bold text-zinc-900 dark:text-white tabular-nums">
                        {sub.node_count || 0}
                      </td>
                      <td className="py-3.5 px-4 hidden sm:table-cell font-mono text-[11px] text-zinc-400">
                        <span className="inline-flex items-center gap-1.5">
                          <span
                            className={`w-1.5 h-1.5 rounded-full ${
                              sub.status === 'error' ? 'bg-rose-500' : sub.status === 'partial' ? 'bg-amber-500' : 'bg-emerald-500'
                            }`}
                          />
                          <span>{formatTime(sub.last_updated)}</span>
                        </span>
                      </td>
                      <td className="py-3.5 px-4">
                        <div className="flex items-center justify-end gap-1">
                          {sub.type === 'remote' && (
                            <button
                              onClick={() => handleRefresh(sub.id)}
                              disabled={refreshingId === sub.id}
                              title={t('subscriptions.refreshTip')}
                              className="hero-btn-default p-1.5 rounded-lg text-zinc-400 hover:text-[#2563EB] dark:hover:text-[#60A5FA]"
                            >
                              <RefreshCw className={`w-3.5 h-3.5 ${refreshingId === sub.id ? 'animate-spin' : ''}`} />
                            </button>
                          )}
                          <button
                            onClick={() => openNodes(sub)}
                            title={t('subscriptions.viewNodesTip')}
                            className="hero-btn-default p-1.5 rounded-lg text-zinc-400 hover:text-zinc-900 dark:hover:text-white"
                          >
                            <Eye className="w-3.5 h-3.5" />
                          </button>
                          <button
                            onClick={() => openDrawer(sub)}
                            title={t('subscriptions.editTip')}
                            className="hero-btn-default p-1.5 rounded-lg text-zinc-400 hover:text-zinc-900 dark:hover:text-white"
                          >
                            <Pencil className="w-3.5 h-3.5" />
                          </button>
                          <button
                            onClick={() => setDeleteTarget(sub)}
                            title={t('subscriptions.deleteTip')}
                            className="hero-btn-default p-1.5 rounded-lg text-zinc-400 hover:text-rose-600 hover:bg-rose-500/10"
                          >
                            <Trash2 className="w-3.5 h-3.5" />
                          </button>
                        </div>
                      </td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </div>
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-3.5 sm:gap-4">
          {filteredSubscriptions.map((sub) => {
            const info = formatUserInfo(sub.user_info, lang)
            return (
              <motion.div
                key={sub.id}
                layout
                initial={{ opacity: 0, y: 6 }}
                animate={{ opacity: 1, y: 0 }}
                className="hero-card hero-card-interactive p-4 sm:p-5 flex flex-col justify-between space-y-4"
              >
                <div className="space-y-3">
                  <div className="flex items-start justify-between gap-2.5">
                    <div className="space-y-1.5 min-w-0">
                      <div className="flex items-center gap-2">
                        <span
                          className={`text-[10px] font-bold px-2 py-0.5 rounded-md border uppercase shrink-0 ${
                            sub.type === 'local'
                              ? 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border-emerald-500/20'
                              : 'bg-[#2563EB]/10 text-[#2563EB] dark:text-[#60A5FA] border-[#2563EB]/20'
                          }`}
                        >
                          {sub.type === 'local' ? t('dashboard.local') : t('dashboard.remote')}
                        </span>
                        <h3 className="font-bold text-sm text-zinc-900 dark:text-white truncate" title={sub.name}>
                          {sub.name}
                        </h3>
                      </div>

                      <div className="text-[11px] text-zinc-400 font-mono truncate flex items-center gap-1.5">
                        <span className="truncate">
                          {sub.type === 'remote' ? maskUrl(sub.url) : t('subscriptions.localImportDesc')}
                        </span>
                        {sub.type === 'remote' && subUrlCount(sub) > 1 && (
                          <span className="hero-chip-default text-[9px] font-mono shrink-0 tabular-nums">
                            {t('subscriptions.urlCount', { count: subUrlCount(sub) })}
                          </span>
                        )}
                        {sub.type === 'remote' && (
                          <button
                            onClick={() => copyText(sub.url, t('subscriptions.rawCopied'))}
                            title={t('common.copy')}
                            className="hover:text-zinc-700 dark:hover:text-zinc-200 transition-colors shrink-0"
                          >
                            <Copy className="w-3 h-3" />
                          </button>
                        )}
                      </div>
                    </div>

                    <div className="flex items-center gap-0.5 sm:gap-1 shrink-0">
                      {sub.type === 'remote' && (
                        <button
                          onClick={() => handleRefresh(sub.id)}
                          disabled={refreshingId === sub.id}
                          title={t('subscriptions.refreshTip')}
                          className="hero-btn-default p-2 sm:p-1.5 rounded-lg text-zinc-400 hover:text-[#2563EB] dark:hover:text-[#60A5FA] disabled:opacity-50"
                        >
                          <RefreshCw className={`w-3.5 h-3.5 ${refreshingId === sub.id ? 'animate-spin' : ''}`} />
                        </button>
                      )}
                      <button
                        onClick={() => openNodes(sub)}
                        title={t('subscriptions.viewNodesTip')}
                        className="hero-btn-default p-2 sm:p-1.5 rounded-lg text-zinc-400 hover:text-zinc-900 dark:hover:text-white"
                      >
                        <Eye className="w-3.5 h-3.5" />
                      </button>
                      <button
                        onClick={() => openDrawer(sub)}
                        title={t('subscriptions.editTip')}
                        className="hero-btn-default p-2 sm:p-1.5 rounded-lg text-zinc-400 hover:text-zinc-900 dark:hover:text-white"
                      >
                        <Pencil className="w-3.5 h-3.5" />
                      </button>
                      <button
                        onClick={() => setDeleteTarget(sub)}
                        title={t('subscriptions.deleteTip')}
                        className="hero-btn-default p-2 sm:p-1.5 rounded-lg text-zinc-400 hover:text-rose-600 hover:bg-rose-500/10"
                      >
                        <Trash2 className="w-3.5 h-3.5" />
                      </button>
                    </div>
                  </div>

                  {info && (
                    <div className="px-2.5 py-2 rounded-lg bg-zinc-50 dark:bg-white/[0.03] border border-zinc-200/60 dark:border-white/[0.06] space-y-1.5">
                      {info.quota.total > 0 ? (
                        <div className="space-y-1">
                          <div className="flex items-center justify-between gap-2 text-[11px] font-mono tabular-nums">
                            <span className="truncate text-zinc-500 dark:text-zinc-400">{info.trafficText}</span>
                            <span
                              className={`shrink-0 font-semibold ${
                                quotaSeverity(info.quota).traffic === 'danger'
                                  ? 'text-rose-600 dark:text-rose-400'
                                  : quotaSeverity(info.quota).traffic === 'warn'
                                    ? 'text-amber-600 dark:text-amber-400'
                                    : 'text-zinc-500 dark:text-zinc-400'
                              }`}
                            >
                              {info.quota.usedPercent}%
                            </span>
                          </div>
                          <div className="h-1.5 rounded-full bg-zinc-200 dark:bg-white/10 overflow-hidden">
                            <div
                              className={`h-full rounded-full transition-all ${quotaBarColor(quotaSeverity(info.quota).traffic)}`}
                              style={{ width: `${info.quota.usedPercent}%` }}
                            />
                          </div>
                        </div>
                      ) : (
                        <div className="text-[11px] font-mono text-zinc-400">{t('subscriptions.trafficUnknown')}</div>
                      )}
                      {info.expireText && (
                        <div className="flex items-center justify-end gap-1.5 text-[11px] font-mono tabular-nums">
                          {expiryChip(info.quota) || (
                            <span className="text-zinc-500 dark:text-zinc-400">
                              {t('subscriptions.expireAt', { date: info.expireText })}
                            </span>
                          )}
                        </div>
                      )}
                    </div>
                  )}

                  {sub.status === 'partial' && sub.error_msg && (
                    <div className="p-2.5 rounded-xl bg-amber-500/10 border border-amber-500/20 text-[11px] text-amber-600 dark:text-amber-400 break-all leading-relaxed space-y-1">
                      <div className="font-semibold">{t('subscriptions.partialTitle')}</div>
                      <div>{sub.error_msg}</div>
                    </div>
                  )}
                  {sub.status === 'error' && sub.error_msg && (
                    <div className="p-2.5 rounded-xl bg-rose-500/10 border border-rose-500/20 text-[11px] text-rose-600 dark:text-rose-400 break-all leading-relaxed space-y-1">
                      <div className="font-semibold">
                        {sub.consecutive_failures > 0
                          ? t('subscriptions.fetchFailedCached', { n: sub.consecutive_failures })
                          : t('subscriptions.fetchFailedTitle')}
                      </div>
                      <div>{sub.error_msg}</div>
                    </div>
                  )}
                </div>

                <div className="pt-2.5 border-t border-zinc-100 dark:border-white/[0.06] flex items-center justify-between text-xs tabular-nums">
                  <div className="flex items-center gap-1.5">
                    <span className="text-zinc-400">{t('subscriptions.validNodes')}</span>
                    <span className="font-bold font-mono text-zinc-900 dark:text-white">{sub.node_count || 0}</span>
                  </div>
                  <div className="flex items-center gap-1.5 font-mono text-zinc-400 text-[11px]">
                    <span
                      className={`w-1.5 h-1.5 rounded-full ${
                        sub.status === 'error' ? 'bg-rose-500' : sub.status === 'partial' ? 'bg-amber-500' : 'bg-emerald-500'
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

      {/* Slide-over Drawer */}
      <Drawer
        open={showDrawer}
        onClose={() => setShowDrawer(false)}
        size="md"
        title={isEditing ? t('subscriptions.drawerEditTitle') : t('subscriptions.drawerAddTitle')}
        footer={
          <div className="flex justify-end gap-2.5">
            <button
              type="button"
              onClick={() => setShowDrawer(false)}
              className="hero-btn-bordered px-4 py-2 text-xs font-medium"
            >
              {t('common.cancel')}
            </button>
            <button
              type="submit"
              form="subscription-form"
              disabled={saving}
              className="hero-btn-primary px-5 py-2 text-xs font-semibold disabled:opacity-50"
            >
              {saving ? t('common.saving') : t('common.save')}
            </button>
          </div>
        }
      >
        <form id="subscription-form" onSubmit={handleSubmit} className="p-4 sm:p-6 space-y-5">
          {!isEditing && (
            <div className="space-y-1.5">
              <label className="block text-[11px] font-semibold text-zinc-500 uppercase tracking-wider">
                {t('subscriptions.typeLabel')}
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
                  <span>{t('subscriptions.typeRemote')}</span>
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
                  <span>{t('subscriptions.typeLocal')}</span>
                </button>
              </div>
            </div>
          )}

          <div>
            <label className="block text-[11px] font-semibold text-zinc-500 uppercase tracking-wider mb-1.5">
              {t('subscriptions.nameLabel')}
            </label>
            <input
              value={form.name}
              onChange={(e) => setForm((f) => ({ ...f, name: e.target.value }))}
              type="text"
              required
              placeholder={form.type === 'local' ? t('subscriptions.nameLocalPh') : t('subscriptions.nameRemotePh')}
              className="hero-input px-3.5 py-2.5 text-sm sm:text-xs text-zinc-900 dark:text-white"
            />
          </div>

          {form.type === 'remote' ? (
            <>
              <div>
                <label className="block text-[11px] font-semibold text-zinc-500 uppercase tracking-wider mb-1.5">
                  {t('subscriptions.urlLabel')}
                </label>
                <textarea
                  value={form.urls}
                  onChange={(e) => setForm((f) => ({ ...f, urls: e.target.value }))}
                  rows={form.urls.split('\n').length > 2 ? Math.min(form.urls.split('\n').length + 1, 6) : 3}
                  required
                  placeholder={'https://...\nhttps://... (可选，多链接逐行填写，按顺序合并)'}
                  className="hero-input px-3.5 py-2.5 text-sm sm:text-xs font-mono text-zinc-900 dark:text-white leading-relaxed resize-y"
                />
                <p className="text-[11px] text-zinc-400 mt-1">{t('subscriptions.urlsHint')}</p>
              </div>

              <div>
                <label className="block text-[11px] font-semibold text-zinc-500 uppercase tracking-wider mb-1.5">
                  {t('subscriptions.fetchProxyLabel')}
                </label>
                <input
                  value={form.fetch_proxy_url}
                  onChange={(e) => setForm((f) => ({ ...f, fetch_proxy_url: e.target.value }))}
                  type="text"
                  placeholder={t('subscriptions.fetchProxyPh')}
                  className="hero-input px-3.5 py-2.5 text-sm sm:text-xs font-mono text-zinc-900 dark:text-white"
                />
                <p className="text-[11px] text-zinc-400 mt-1">{t('subscriptions.fetchProxyHint')}</p>
              </div>

              <div>
                <label className="block text-[11px] font-semibold text-zinc-500 uppercase tracking-wider mb-1.5">
                  {t('subscriptions.failurePolicyLabel')}
                </label>
                <div className="grid grid-cols-2 gap-1.5 p-1 bg-zinc-100 dark:bg-white/[0.04] rounded-xl">
                  {[
                    { value: 'strict', label: t('subscriptions.policyStrict'), desc: t('subscriptions.policyStrictDesc') },
                    { value: 'skip', label: t('subscriptions.policySkip'), desc: t('subscriptions.policySkipDesc') }
                  ].map((opt) => (
                    <button
                      key={opt.value}
                      type="button"
                      onClick={() => setForm((f) => ({ ...f, failure_policy: opt.value }))}
                      className={`py-2 px-2 text-xs font-bold rounded-lg transition-all ${
                        (form.failure_policy || 'strict') === opt.value
                          ? 'bg-white dark:bg-[#1E2330] text-[#2563EB] dark:text-[#60A5FA] shadow-xs'
                          : 'text-zinc-500'
                      }`}
                    >
                      {opt.label}
                    </button>
                  ))}
                </div>
                <p className="text-[11px] text-zinc-400 mt-1">
                  {(form.failure_policy || 'strict') === 'strict' ? t('subscriptions.policyStrictDesc') : t('subscriptions.policySkipDesc')}
                </p>
              </div>

              <div>
                <label className="block text-[11px] font-semibold text-zinc-500 uppercase tracking-wider mb-1.5">
                  {t('subscriptions.fetchModeLabel')}
                </label>
                <div className="grid grid-cols-2 gap-1.5 p-1 bg-zinc-100 dark:bg-white/[0.04] rounded-xl">
                  {[
                    { value: 'merge', label: t('subscriptions.modeMerge') },
                    { value: 'failover', label: t('subscriptions.modeFailover') }
                  ].map((opt) => (
                    <button
                      key={opt.value}
                      type="button"
                      onClick={() => setForm((f) => ({ ...f, fetch_mode: opt.value }))}
                      className={`py-2 px-2 text-xs font-bold rounded-lg transition-all ${
                        (form.fetch_mode || 'merge') === opt.value
                          ? 'bg-white dark:bg-[#1E2330] text-[#2563EB] dark:text-[#60A5FA] shadow-xs'
                          : 'text-zinc-500'
                      }`}
                    >
                      {opt.label}
                    </button>
                  ))}
                </div>
                <p className="text-[11px] text-zinc-400 mt-1">
                  {(form.fetch_mode || 'merge') === 'failover'
                    ? t('subscriptions.modeFailoverDesc')
                    : t('subscriptions.modeMergeDesc')}
                </p>
              </div>

              <div>
                <label className="block text-[11px] font-semibold text-zinc-500 uppercase tracking-wider mb-1.5">
                  {t('subscriptions.uaLabel')}
                </label>
                <input
                  value={form.user_agent}
                  onChange={(e) => setForm((f) => ({ ...f, user_agent: e.target.value }))}
                  type="text"
                  placeholder="ClashMeta/v1.18.0 (SubDeck)"
                  className="hero-input px-3.5 py-2.5 text-sm sm:text-xs font-mono text-zinc-900 dark:text-white"
                />
              </div>

              <div className="grid grid-cols-1 sm:grid-cols-2 gap-3.5 sm:gap-4 pt-1">
                <div>
                  <label className="block text-[11px] font-semibold text-zinc-500 uppercase tracking-wider mb-1.5">
                    {t('subscriptions.intervalLabel')}
                  </label>
                  <input
                    value={form.interval_hours}
                    onChange={(e) => setForm((f) => ({ ...f, interval_hours: parseInt(e.target.value) || 12 }))}
                    type="number"
                    min="1"
                    max="720"
                    className="hero-input px-3.5 py-2 text-sm sm:text-xs font-mono text-zinc-900 dark:text-white"
                  />
                </div>
                <div className="flex items-center sm:pt-5">
                  <label className="flex items-center gap-2.5 cursor-pointer">
                    <input
                      type="checkbox"
                      checked={form.auto_update}
                      onChange={(e) => setForm((f) => ({ ...f, auto_update: e.target.checked }))}
                      className="rounded text-[#2563EB] focus:ring-[#2563EB] w-4 h-4"
                    />
                    <span className="text-xs text-zinc-700 dark:text-zinc-300 font-medium">
                      {t('subscriptions.autoUpdateLabel')}
                    </span>
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
                    {t('subscriptions.skipVerifyLabel')}
                  </span>
                </label>
              </div>
            </>
          ) : (
            <div className="space-y-2">
              <div className="flex items-center justify-between">
                <label className="block text-[11px] font-semibold text-zinc-500 uppercase tracking-wider">
                  {t('subscriptions.contentLabel')}
                </label>
                <span className="hero-chip-success text-[10px] font-mono font-medium tabular-nums">
                  {t('subscriptions.recognizedCount', { count: localRecognizedCount })}
                </span>
              </div>

              <div
                onDragOver={(e) => {
                  e.preventDefault()
                  setIsDraggingFile(true)
                }}
                onDragLeave={() => setIsDraggingFile(false)}
                onDrop={handleFileDrop}
                className={`relative rounded-xl transition-all ${
                  isDraggingFile ? 'ring-2 ring-[#2563EB] bg-[#2563EB]/5' : ''
                }`}
              >
                <textarea
                  value={form.raw_content}
                  onChange={(e) => setForm((f) => ({ ...f, raw_content: e.target.value }))}
                  rows={10}
                  required
                  placeholder={t('subscriptions.contentPh')}
                  className="hero-input p-3.5 text-sm sm:text-xs font-mono text-zinc-900 dark:text-white leading-relaxed"
                />
                {isDraggingFile && (
                  <div className="absolute inset-0 rounded-xl flex items-center justify-center bg-[#2563EB]/10 backdrop-blur-[1px] pointer-events-none">
                    <span className="px-3 py-1.5 rounded-lg bg-[#2563EB] text-white text-xs font-semibold flex items-center gap-1.5 shadow-lg">
                      <Upload className="w-3.5 h-3.5" />
                      {t('subscriptions.dropFileHint')}
                    </span>
                  </div>
                )}
              </div>
            </div>
          )}
        </form>
      </Drawer>

      <NodesModal
        open={nodesModal.open}
        title={nodesModal.title}
        nodes={nodesModal.nodes}
        probing={probingModal}
        onProbe={handleProbeNodes}
        onClose={() => setNodesModal({ open: false, subId: '', title: '', nodes: [] })}
        onCopy={copyText}
      />

      <ConfirmDialog
        open={!!deleteTarget}
        title={t('subscriptions.deleteTitle')}
        description={deleteTarget ? t('subscriptions.deleteDesc', { name: deleteTarget.name }) : ''}
        confirmText={t('common.delete')}
        loading={deleting}
        onConfirm={confirmDelete}
        onCancel={() => setDeleteTarget(null)}
      />
    </div>
  )
}
