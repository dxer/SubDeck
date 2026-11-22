import React, { useState, useEffect, useMemo } from 'react'
import { Link } from 'react-router-dom'
import { motion } from 'framer-motion'
import {
  Rss,
  Layers,
  Activity,
  Plus,
  Zap,
  ChevronDown,
  QrCode,
  Copy,
  ArrowRight,
  CheckCircle2,
  FileCode2,
  AlertTriangle,
  HeartPulse
} from 'lucide-react'
import { api } from '../api'
import { useI18n } from '../i18n'
import { Toast, useToast } from '../components/Toast'
import QrModal from '../components/QrModal'
import ConfigPreviewDrawer from '../components/ConfigPreviewDrawer'
import { MetricSkeleton, RowListSkeleton } from '../components/Skeleton'
import { getUniversalUrl, getExportUrl, copyToClipboard, formatUserInfo, quotaSeverity } from '../utils/subUrl'

const PROTO_COLORS = {
  vless: 'bg-emerald-500',
  trojan: 'bg-amber-500',
  hysteria2: 'bg-indigo-500',
  vmess: 'bg-blue-500',
  ss: 'bg-cyan-500',
  tuic: 'bg-sky-500',
  anytls: 'bg-teal-500',
  wireguard: 'bg-slate-500',
  snell: 'bg-zinc-500'
}

export default function Dashboard() {
  const { t, lang } = useI18n()
  const [stats, setStats] = useState({
    total_sub_count: 0,
    remote_sub_count: 0,
    local_sub_count: 0,
    profile_count: 0,
    total_node_count: 0
  })
  const [profiles, setProfiles] = useState([])
  const [subscriptions, setSubscriptions] = useState([])
  const [allNodes, setAllNodes] = useState([])
  const [loading, setLoading] = useState(true)
  const [activeDropdown, setActiveDropdown] = useState(null)
  const [copiedProfileId, setCopiedProfileId] = useState(null)
  const { toast, showToast } = useToast()

  const [qrState, setQrState] = useState({ open: false, token: '', name: '' })
  const [previewConfigProfile, setPreviewConfigProfile] = useState(null)

  const copyUrl = async (url, successMsg, profileId = null) => {
    const ok = await copyToClipboard(url)
    if (ok) {
      showToast(successMsg || t('dashboard.universalCopiedToast'))
      if (profileId) {
        setCopiedProfileId(profileId)
        setTimeout(() => setCopiedProfileId(null), 1800)
      }
    } else {
      showToast(t('common.copyFailed'), 'error')
    }
  }

  useEffect(() => {
    async function load() {
      setLoading(true)
      try {
        const [sRes, pRes, subsRes, nodesRes] = await Promise.all([
          api.getStats(),
          api.getProfiles(),
          api.getSubscriptions(),
          api.previewDraftProfile({ deduplicate: false }).catch(() => ({ nodes: [] }))
        ])
        setStats(sRes || {})
        setProfiles(Array.isArray(pRes) ? pRes : (pRes?.data || []))
        setSubscriptions(Array.isArray(subsRes) ? subsRes : (subsRes?.data || []))
        setAllNodes(Array.isArray(nodesRes?.nodes) ? nodesRes.nodes : [])
      } catch (err) {
        showToast(err.message || t('dashboard.loadFailed'), 'error')
      } finally {
        setLoading(false)
      }
    }
    load()
  }, [showToast, t])

  useEffect(() => {
    const handleOutsideClick = (e) => {
      if (!e.target.closest('.dashboard-dropdown-container')) {
        setActiveDropdown(null)
      }
    }
    window.addEventListener('click', handleOutsideClick)
    return () => window.removeEventListener('click', handleOutsideClick)
  }, [])

  const errorSubCount = useMemo(
    () => subscriptions.filter((s) => s.status === 'error').length,
    [subscriptions]
  )

  // Subscription quota health: only sources that report a quota header, sorted
  // so expiring/exhausted plans surface first.
  const quotaRows = useMemo(() => {
    const rank = { expired: 0, danger: 0, warn: 1, ok: 2 }
    return subscriptions
      .map((sub) => {
        const info = formatUserInfo(sub.user_info, lang)
        if (!info || !info.quota) return null
        const sev = quotaSeverity(info.quota)
        const worst = Math.min(
          ...(sev.traffic ? [rank[sev.traffic]] : [3]),
          ...(sev.expiry ? [rank[sev.expiry]] : [3])
        )
        return { sub, quota: info.quota, sev, trafficText: info.trafficText, expireText: info.expireText, worst }
      })
      .filter(Boolean)
      .sort((a, b) => a.worst - b.worst || (a.quota.daysLeft ?? 999) - (b.quota.daysLeft ?? 999))
  }, [subscriptions, lang])

  const quotaWarnCount = useMemo(
    () => quotaRows.filter((r) => r.worst <= 1).length,
    [quotaRows]
  )

  const protocolBreakdown = useMemo(() => {
    if (!allNodes.length) return []
    const counts = {}
    allNodes.forEach((n) => {
      const k = (n.type || 'other').toLowerCase()
      counts[k] = (counts[k] || 0) + 1
    })
    return Object.entries(counts)
      .sort((a, b) => b[1] - a[1])
      .map(([type, count]) => ({
        type,
        count,
        pct: Math.round((count / allNodes.length) * 100),
        color: PROTO_COLORS[type] || 'bg-zinc-400'
      }))
  }, [allNodes])

  const getSubNames = (ids) => {
    if (!ids || ids.length === 0) return []
    return ids.map((id) => {
      const sub = subscriptions.find((s) => s.id === id)
      return sub ? sub.name : `#${id.slice(0, 6)}`
    })
  }

  return (
    <div className="space-y-5 sm:space-y-6 max-w-7xl mx-auto">
      <Toast toast={toast} />

      {/* Header */}
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3 pb-1">
        <h1 className="text-xl sm:text-2xl font-bold tracking-tight text-zinc-900 dark:text-white">
          {t('dashboard.title')}
        </h1>

        <div className="grid grid-cols-2 sm:flex items-center gap-2 shrink-0">
          <Link to="/subscriptions" className="hero-btn-bordered px-3.5 py-2 text-xs font-medium gap-1.5">
            <Rss className="w-3.5 h-3.5 text-[#2563EB] dark:text-[#60A5FA]" />
            <span>{t('dashboard.addSub')}</span>
          </Link>
          <Link to="/profiles" className="hero-btn-primary px-3.5 py-2 text-xs font-semibold gap-1.5">
            <Plus className="w-4 h-4" />
            <span>{t('dashboard.newProfile')}</span>
          </Link>
        </div>
      </div>

      {/* Metrics */}
      {loading ? (
        <MetricSkeleton />
      ) : (
        <motion.div
          initial={{ opacity: 0, y: 6 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.18 }}
          className="grid grid-cols-1 md:grid-cols-3 gap-3.5 sm:gap-4"
        >
          {/* Card 1: Sources & Health */}
          <div className="hero-card p-4 sm:p-5 flex flex-col justify-between space-y-3.5 sm:space-y-4">
            <div className="flex items-center justify-between">
              <span className="text-xs font-semibold text-zinc-500 dark:text-zinc-400">
                {t('dashboard.cardSources')}
              </span>
              <div className="w-8 h-8 rounded-lg bg-[#2563EB]/10 text-[#2563EB] dark:text-[#60A5FA] flex items-center justify-center">
                <Rss className="w-4 h-4" />
              </div>
            </div>
            <div className="flex items-baseline gap-2">
              <span className="text-3xl font-extrabold font-mono tabular-nums tracking-tight text-zinc-900 dark:text-white">
                {stats.total_sub_count}
              </span>
              <span className="text-xs text-zinc-400">{t('dashboard.sourceUnit')}</span>
            </div>
            <div className="pt-3 border-t border-zinc-100 dark:border-white/[0.06] flex items-center justify-between text-xs tabular-nums">
              <div className="flex items-center gap-2.5 text-zinc-500 dark:text-zinc-400">
                <span>
                  {t('dashboard.remote')}{' '}
                  <strong className="font-mono text-zinc-800 dark:text-zinc-200">{stats.remote_sub_count}</strong>
                </span>
                <span className="text-zinc-300 dark:text-zinc-700">·</span>
                <span>
                  {t('dashboard.local')}{' '}
                  <strong className="font-mono text-zinc-800 dark:text-zinc-200">{stats.local_sub_count}</strong>
                </span>
              </div>
              {errorSubCount > 0 ? (
                <Link
                  to="/subscriptions"
                  className="inline-flex items-center gap-1 text-[11px] font-semibold text-rose-600 dark:text-rose-400 hover:underline"
                >
                  <AlertTriangle className="w-3 h-3" />
                  <span>{t('dashboard.abnormalCount', { count: errorSubCount })}</span>
                </Link>
              ) : (
                <span className="text-[11px] font-medium text-emerald-600 dark:text-emerald-400">
                  {t('dashboard.allHealthy')}
                </span>
              )}
            </div>
          </div>

          {/* Card 2: Profiles */}
          <div className="hero-card p-4 sm:p-5 flex flex-col justify-between space-y-3.5 sm:space-y-4">
            <div className="flex items-center justify-between">
              <span className="text-xs font-semibold text-zinc-500 dark:text-zinc-400">
                {t('dashboard.cardProfiles')}
              </span>
              <div className="w-8 h-8 rounded-lg bg-[#2563EB]/10 text-[#2563EB] dark:text-[#60A5FA] flex items-center justify-center">
                <Layers className="w-4 h-4" />
              </div>
            </div>
            <div className="flex items-baseline gap-2">
              <span className="text-3xl font-extrabold font-mono tabular-nums tracking-tight text-zinc-900 dark:text-white">
                {stats.profile_count}
              </span>
              <span className="text-xs text-zinc-400">{t('dashboard.profileUnit')}</span>
            </div>
            <div className="pt-3 border-t border-zinc-100 dark:border-white/[0.06] flex items-center justify-between text-xs">
              <span className="text-zinc-500 dark:text-zinc-400 flex items-center gap-1.5">
                <CheckCircle2 className="w-3.5 h-3.5 text-emerald-500 shrink-0" />
                <span>{t('dashboard.adaptiveEndpoint')}</span>
              </span>
              <Link to="/profiles" className="text-[#2563EB] dark:text-[#60A5FA] font-semibold hover:underline">
                {t('common.manage')}
              </Link>
            </div>
          </div>

          {/* Card 3: Node Pool + Protocol Distribution */}
          <div className="hero-card p-4 sm:p-5 flex flex-col justify-between space-y-3">
            <div className="flex items-center justify-between">
              <span className="text-xs font-semibold text-zinc-500 dark:text-zinc-400">
                {t('dashboard.cardProtocols')}
              </span>
              <div className="w-8 h-8 rounded-lg bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 flex items-center justify-center">
                <Activity className="w-4 h-4" />
              </div>
            </div>

            <div className="flex items-baseline gap-2">
              <span className="text-3xl font-extrabold font-mono tabular-nums tracking-tight text-zinc-900 dark:text-white">
                {stats.total_node_count}
              </span>
              <span className="text-xs text-zinc-400">{t('dashboard.nodeUnit')}</span>
            </div>

            <div className="space-y-2 pt-1">
              {protocolBreakdown.length > 0 ? (
                <>
                  <div className="h-2 w-full rounded-full bg-zinc-100 dark:bg-white/5 overflow-hidden flex gap-0.5">
                    {protocolBreakdown.map((item) => (
                      <div
                        key={item.type}
                        style={{ width: `${Math.max(item.pct, 4)}%` }}
                        title={`${item.type.toUpperCase()}: ${item.count} (${item.pct}%)`}
                        className={`${item.color} h-full transition-all`}
                      />
                    ))}
                  </div>
                  <div className="flex items-center gap-3 flex-wrap text-[11px] text-zinc-500 dark:text-zinc-400 tabular-nums">
                    {protocolBreakdown.slice(0, 4).map((item) => (
                      <span key={item.type} className="inline-flex items-center gap-1">
                        <span className={`w-1.5 h-1.5 rounded-full ${item.color}`} />
                        <span className="font-mono uppercase">{item.type}</span>
                        <span className="text-zinc-400">{item.count}</span>
                      </span>
                    ))}
                  </div>
                </>
              ) : (
                <div className="text-xs text-zinc-400 pt-1">{t('dashboard.emptyProtocols')}</div>
              )}
            </div>
          </div>
        </motion.div>
      )}

      {/* Subscription Health Panel */}
      {quotaRows.length > 0 && (
        <div className="hero-card overflow-visible">
          <div className="px-4 sm:px-6 py-3.5 sm:py-4 border-b border-zinc-100 dark:border-white/[0.06] flex items-center justify-between gap-2">
            <div className="flex items-center gap-2">
              <HeartPulse className="w-4 h-4 text-[#2563EB] dark:text-[#60A5FA]" />
              <h2 className="text-sm font-bold text-zinc-900 dark:text-white">{t('dashboard.healthTitle')}</h2>
              <span className="hero-chip-default text-xs font-medium tabular-nums">{quotaRows.length}</span>
            </div>
            <div className="flex items-center gap-2">
              {quotaWarnCount > 0 ? (
                <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full bg-rose-500/10 border border-rose-500/20 text-[11px] font-semibold text-rose-600 dark:text-rose-400 tabular-nums">
                  <AlertTriangle className="w-3 h-3" />
                  {t('dashboard.healthWarnCount', { count: quotaWarnCount })}
                </span>
              ) : (
                <span className="text-[11px] font-medium text-emerald-600 dark:text-emerald-400">
                  {t('dashboard.healthAllGood')}
                </span>
              )}
              <Link to="/subscriptions" className="text-[#2563EB] dark:text-[#60A5FA] font-semibold hover:underline text-xs">
                {t('common.manage')}
              </Link>
            </div>
          </div>
          <div className="p-3 sm:p-4 space-y-2.5">
            {quotaRows.slice(0, 6).map(({ sub, quota, sev, trafficText, expireText }) => (
              <div key={sub.id} className="flex items-center gap-3">
                <span className="text-xs font-semibold text-zinc-800 dark:text-zinc-200 truncate w-28 sm:w-40 shrink-0" title={sub.name}>
                  {sub.name}
                </span>
                <div className="flex-1 min-w-0 space-y-1">
                  {quota.total > 0 ? (
                    <>
                      <div className="flex items-center justify-between gap-2 text-[11px] font-mono tabular-nums">
                        <span className="truncate text-zinc-500 dark:text-zinc-400">{trafficText}</span>
                        {expireText && (
                          <span
                            className={`shrink-0 font-semibold ${
                              sev.expiry === 'expired' || sev.expiry === 'danger'
                                ? 'text-rose-600 dark:text-rose-400'
                                : sev.expiry === 'warn'
                                  ? 'text-amber-600 dark:text-amber-400'
                                  : 'text-zinc-500 dark:text-zinc-400'
                            }`}
                          >
                            {quota.expired
                              ? t('subscriptions.quotaExpired')
                              : quota.daysLeft === 0
                                ? t('subscriptions.quotaToday')
                                : t('dashboard.healthExpireDays', { days: quota.daysLeft })}
                          </span>
                        )}
                      </div>
                      <div className="h-1.5 rounded-full bg-zinc-200 dark:bg-white/10 overflow-hidden">
                        <div
                          className={`h-full rounded-full transition-all ${
                            sev.traffic === 'danger' ? 'bg-rose-500' : sev.traffic === 'warn' ? 'bg-amber-500' : 'bg-emerald-500'
                          }`}
                          style={{ width: `${quota.usedPercent}%` }}
                        />
                      </div>
                    </>
                  ) : (
                    <div className="text-[11px] font-mono text-zinc-400">
                      {expireText
                        ? quota.expired
                          ? t('subscriptions.quotaExpired')
                          : t('dashboard.healthExpireDays', { days: quota.daysLeft })
                        : t('subscriptions.trafficUnknown')}
                    </div>
                  )}
                </div>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Quick Distribution Panel */}
      <div className="hero-card overflow-visible">
        <div className="px-4 sm:px-6 py-3.5 sm:py-4 border-b border-zinc-100 dark:border-white/[0.06] flex items-center justify-between gap-2">
          <div className="flex items-center gap-2">
            <h2 className="text-sm font-bold text-zinc-900 dark:text-white">{t('dashboard.distTitle')}</h2>
            <span className="hero-chip-default text-xs font-medium tabular-nums">
              {profiles.length}
            </span>
          </div>
          <Link
            to="/profiles"
            className="text-xs text-[#2563EB] dark:text-[#60A5FA] font-semibold hover:underline flex items-center gap-1 shrink-0"
          >
            <span>{t('dashboard.manageRules')}</span>
            <ArrowRight className="w-3.5 h-3.5" />
          </Link>
        </div>

        {loading ? (
          <RowListSkeleton count={3} />
        ) : profiles.length === 0 ? (
          <div className="p-10 sm:p-12 text-center space-y-3">
            <div className="w-11 h-11 rounded-2xl bg-zinc-100 dark:bg-white/5 flex items-center justify-center text-zinc-400 mx-auto">
              <Layers className="w-5 h-5" />
            </div>
            <div className="text-sm font-semibold text-zinc-800 dark:text-zinc-200">{t('dashboard.emptyProfiles')}</div>
            <Link to="/profiles" className="hero-btn-primary px-4 py-2 text-xs font-semibold inline-flex">
              {t('dashboard.createFirst')}
            </Link>
          </div>
        ) : (
          <div className="divide-y divide-zinc-100 dark:divide-white/[0.06]">
            {profiles.map((p, idx) => {
              const subNames = getSubNames(p.subscription_ids)
              const openUp = idx >= profiles.length - 1 && profiles.length > 2
              return (
                <div
                  key={p.id}
                  className="p-4 sm:p-5 flex flex-col lg:flex-row lg:items-center justify-between gap-3.5 sm:gap-4 hover:bg-zinc-50/70 dark:hover:bg-white/[0.02] transition-colors"
                >
                  <div className="space-y-2 min-w-0">
                    <div className="flex items-center gap-2 flex-wrap">
                      <span className="font-bold text-[15px] text-zinc-900 dark:text-white tracking-tight">
                        {p.name}
                      </span>
                      <span className="hero-chip-primary text-xs tabular-nums">
                        <span className="font-mono font-bold mr-1">{p.node_count || 0}</span>
                        <span>{t('common.nodes')}</span>
                      </span>
                    </div>

                    <div className="flex items-center gap-1.5 flex-wrap text-xs">
                      <span className="text-zinc-400">{t('dashboard.sourcesLabel')}</span>
                      {subNames.length > 0 ? (
                        subNames.map((sname, sIdx) => (
                          <span key={sIdx} className="hero-chip-default text-[11px]">
                            {sname}
                          </span>
                        ))
                      ) : (
                        <span className="text-zinc-400 italic text-[11px]">{t('dashboard.allSourcesFallback')}</span>
                      )}
                    </div>

                    <div className="flex items-center gap-2 text-xs text-zinc-400">
                      <span className="font-mono bg-zinc-100 dark:bg-white/5 px-2 py-0.5 rounded-md border border-zinc-200/60 dark:border-white/10 truncate max-w-[220px] sm:max-w-none">
                        /s/{p.token.slice(0, 8)}...{p.token.slice(-4)}
                      </span>
                      <button
                        onClick={() => copyUrl(getUniversalUrl(p.token), t('dashboard.universalCopiedToast'))}
                        title={t('common.copy')}
                        className="hover:text-zinc-700 dark:hover:text-zinc-200 p-1 rounded transition-colors shrink-0"
                      >
                        <Copy className="w-3.5 h-3.5" />
                      </button>
                    </div>
                  </div>

                  <div className="flex items-center gap-2 flex-wrap w-full lg:w-auto shrink-0">
                    <button
                      onClick={() => copyUrl(getUniversalUrl(p.token), t('dashboard.universalCopiedToast'), p.id)}
                      className={`hero-btn-primary w-full sm:w-auto px-3.5 py-2 text-xs font-semibold gap-1.5 transition-all ${
                        copiedProfileId === p.id ? '!bg-emerald-600' : ''
                      }`}
                    >
                      {copiedProfileId === p.id ? (
                        <>
                          <CheckCircle2 className="w-3.5 h-3.5 text-white" />
                          <span>{t('dashboard.copiedUniversal')}</span>
                        </>
                      ) : (
                        <>
                          <Zap className="w-3.5 h-3.5 text-amber-300" />
                          <span>{t('dashboard.copyUniversal')}</span>
                        </>
                      )}
                    </button>

                    <div className="relative dashboard-dropdown-container">
                      <button
                        onClick={(e) => {
                          e.stopPropagation()
                          setActiveDropdown(activeDropdown === p.id ? null : p.id)
                        }}
                        className="hero-btn-bordered px-3 py-2 text-xs font-semibold gap-1"
                      >
                        <span>{t('dashboard.exportFormat')}</span>
                        <ChevronDown className="w-3.5 h-3.5 text-zinc-400" />
                      </button>

                      {activeDropdown === p.id && (
                        <div
                          className={`absolute left-0 sm:left-auto sm:right-0 ${
                            openUp ? 'bottom-full mb-2' : 'mt-2'
                          } w-52 rounded-xl bg-white dark:bg-[#181C26] border border-zinc-200 dark:border-white/10 shadow-xl py-1 z-30 text-xs`}
                        >
                          <button
                            onClick={() => {
                              copyUrl(getExportUrl(p.token, 'clash', true), t('dashboard.clashFullCopied'))
                              setActiveDropdown(null)
                            }}
                            className="w-full text-left px-3.5 py-2 hover:bg-zinc-100 dark:hover:bg-white/5 text-zinc-700 dark:text-zinc-200 flex items-center justify-between"
                          >
                            <span>{t('dashboard.clashFull')}</span>
                            <span className="text-[10px] font-mono text-zinc-400">YAML</span>
                          </button>
                          <button
                            onClick={() => {
                              copyUrl(getExportUrl(p.token, 'singbox', true), t('dashboard.singboxFullCopied'))
                              setActiveDropdown(null)
                            }}
                            className="w-full text-left px-3.5 py-2 hover:bg-zinc-100 dark:hover:bg-white/5 text-zinc-700 dark:text-zinc-200 flex items-center justify-between"
                          >
                            <span>{t('dashboard.singboxFull')}</span>
                            <span className="text-[10px] font-mono text-zinc-400">JSON</span>
                          </button>
                          <button
                            onClick={() => {
                              copyUrl(getExportUrl(p.token, 'clash', false), t('dashboard.clashProviderCopied'))
                              setActiveDropdown(null)
                            }}
                            className="w-full text-left px-3.5 py-2 hover:bg-zinc-100 dark:hover:bg-white/5 text-zinc-700 dark:text-zinc-200 flex items-center justify-between"
                          >
                            <span>{t('dashboard.clashProvider')}</span>
                            <span className="text-[10px] font-mono text-zinc-400">Provider</span>
                          </button>
                          <button
                            onClick={() => {
                              copyUrl(getExportUrl(p.token, 'base64', false), t('dashboard.base64Copied'))
                              setActiveDropdown(null)
                            }}
                            className="w-full text-left px-3.5 py-2 hover:bg-zinc-100 dark:hover:bg-white/5 text-zinc-700 dark:text-zinc-200 flex items-center justify-between"
                          >
                            <span>{t('dashboard.base64Uri')}</span>
                            <span className="text-[10px] font-mono text-zinc-400">Base64</span>
                          </button>
                        </div>
                      )}
                    </div>

                    <button
                      onClick={() => setPreviewConfigProfile(p)}
                      title={t('dashboard.viewConfig')}
                      className="hero-btn-bordered p-2 text-zinc-600 dark:text-zinc-300"
                    >
                      <FileCode2 className="w-4 h-4" />
                    </button>

                    <button
                      onClick={() => setQrState({ open: true, token: p.token, name: p.name })}
                      title={t('dashboard.qrCode')}
                      className="hero-btn-bordered p-2 text-zinc-600 dark:text-zinc-300"
                    >
                      <QrCode className="w-4 h-4" />
                    </button>
                  </div>
                </div>
              )
            })}
          </div>
        )}
      </div>

      <QrModal
        open={qrState.open}
        token={qrState.token}
        profileName={qrState.name}
        onClose={() => setQrState({ open: false, token: '', name: '' })}
        onCopy={copyUrl}
      />

      <ConfigPreviewDrawer
        open={!!previewConfigProfile}
        profile={previewConfigProfile}
        onClose={() => setPreviewConfigProfile(null)}
        onNotify={(msg, type) => showToast(msg, type)}
      />
    </div>
  )
}
