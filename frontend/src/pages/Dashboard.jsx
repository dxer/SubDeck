import React, { useState, useEffect } from 'react'
import { Link } from 'react-router-dom'
import { motion } from 'framer-motion'
import {
  Rss,
  Layers,
  Activity,
  ShieldCheck,
  Plus,
  Zap,
  ChevronDown,
  QrCode,
  Copy,
  ArrowRight,
  CheckCircle2,
  Eye
} from 'lucide-react'
import { api } from '../api'
import { Toast, useToast } from '../components/Toast'
import QrModal from '../components/QrModal'
import { MetricSkeleton, RowListSkeleton } from '../components/Skeleton'
import { getUniversalUrl, getExportUrl } from '../utils/subUrl'

export default function Dashboard() {
  const [stats, setStats] = useState({
    total_sub_count: 0,
    remote_sub_count: 0,
    local_sub_count: 0,
    profile_count: 0,
    total_node_count: 0
  })
  const [profiles, setProfiles] = useState([])
  const [subscriptions, setSubscriptions] = useState([])
  const [loading, setLoading] = useState(true)
  const [activeDropdown, setActiveDropdown] = useState(null)
  const [copiedProfileId, setCopiedProfileId] = useState(null)
  const { toast, showToast } = useToast()

  // QR Modal
  const [qrState, setQrState] = useState({ open: false, token: '', name: '' })

  const copyUrl = async (url, successMsg, profileId = null) => {
    try {
      await navigator.clipboard.writeText(url)
      showToast(successMsg || '自适应订阅链接已复制！')
      if (profileId) {
        setCopiedProfileId(profileId)
        setTimeout(() => setCopiedProfileId(null), 1800)
      }
    } catch {
      showToast('复制失败，请手动复制', 'error')
    }
  }

  useEffect(() => {
    async function load() {
      setLoading(true)
      try {
        const [sRes, pRes, subsRes] = await Promise.all([
          api.getStats(),
          api.getProfiles(),
          api.getSubscriptions()
        ])
        setStats(sRes || {})
        setProfiles(Array.isArray(pRes) ? pRes : (pRes?.data || []))
        setSubscriptions(Array.isArray(subsRes) ? subsRes : (subsRes?.data || []))
      } catch (err) {
        showToast(err.message || '加载仪表盘失败', 'error')
      } finally {
        setLoading(false)
      }
    }
    load()
  }, [showToast])

  useEffect(() => {
    const handleOutsideClick = (e) => {
      if (!e.target.closest('.dashboard-dropdown-container')) {
        setActiveDropdown(null)
      }
    }
    window.addEventListener('click', handleOutsideClick)
    return () => window.removeEventListener('click', handleOutsideClick)
  }, [])

  const getSubNames = (ids) => {
    if (!ids || ids.length === 0) return []
    return ids.map((id) => {
      const sub = subscriptions.find((s) => s.id === id)
      return sub ? sub.name : `源 #${id}`
    })
  }

  return (
    <div className="space-y-6 max-w-7xl mx-auto">
      <Toast toast={toast} />

      {/* Page Header */}
      <div className="hero-card flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4 p-5">
        <div className="flex items-center gap-3">
          <h1 className="text-lg sm:text-xl font-bold tracking-tight text-zinc-900 dark:text-white">
            概览仪表盘
          </h1>
          <span className="hero-chip-success text-[11px]">
            <span className="w-1.5 h-1.5 rounded-full bg-emerald-500 mr-1.5 animate-pulse" />
            运行就绪
          </span>
        </div>

        <div className="flex items-center gap-2.5 shrink-0">
          <Link to="/profiles" className="hero-btn-primary px-3.5 py-2 text-xs font-semibold gap-1.5">
            <Plus className="w-4 h-4" />
            <span>新建订阅组合</span>
          </Link>
          <Link to="/subscriptions" className="hero-btn-bordered px-3.5 py-2 text-xs font-medium gap-1.5">
            <Rss className="w-4 h-4 text-[#2563EB] dark:text-[#60A5FA]" />
            <span>添加订阅源</span>
          </Link>
        </div>
      </div>

      {/* 4 Metrics Ribbon Cards */}
      {loading ? (
        <MetricSkeleton />
      ) : (
        <motion.div
          initial={{ opacity: 0, y: 8 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.2 }}
          className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4"
        >
          <div className="hero-card p-5 space-y-3">
            <div className="flex items-center justify-between">
              <span className="text-xs font-semibold text-zinc-500 dark:text-zinc-400">有效订阅源</span>
              <div className="w-8 h-8 rounded-lg bg-[#2563EB]/10 text-[#2563EB] dark:text-[#60A5FA] flex items-center justify-center">
                <Rss className="w-4 h-4" />
              </div>
            </div>
            <div className="flex items-baseline gap-1.5">
              <span className="text-3xl font-extrabold font-mono tabular-nums tracking-tight text-zinc-900 dark:text-white">
                {stats.total_sub_count}
              </span>
              <span className="text-xs text-zinc-400">个源</span>
            </div>
            <div className="pt-2.5 border-t border-zinc-100 dark:border-white/[0.06] flex items-center gap-2.5 text-xs text-zinc-500 dark:text-zinc-400 tabular-nums">
              <span>
                远程 <strong className="font-mono text-zinc-800 dark:text-zinc-200">{stats.remote_sub_count}</strong>
              </span>
              <span className="text-zinc-300 dark:text-zinc-700">·</span>
              <span>
                本地 <strong className="font-mono text-zinc-800 dark:text-zinc-200">{stats.local_sub_count}</strong>
              </span>
            </div>
          </div>

          <div className="hero-card p-5 space-y-3">
            <div className="flex items-center justify-between">
              <span className="text-xs font-semibold text-zinc-500 dark:text-zinc-400">活跃订阅组合</span>
              <div className="w-8 h-8 rounded-lg bg-[#2563EB]/10 text-[#2563EB] dark:text-[#60A5FA] flex items-center justify-center">
                <Layers className="w-4 h-4" />
              </div>
            </div>
            <div className="flex items-baseline gap-1.5">
              <span className="text-3xl font-extrabold font-mono tabular-nums tracking-tight text-zinc-900 dark:text-white">
                {stats.profile_count}
              </span>
              <span className="text-xs text-zinc-400">组配置</span>
            </div>
            <div className="pt-2.5 border-t border-zinc-100 dark:border-white/[0.06] flex items-center gap-1.5 text-xs text-emerald-600 dark:text-emerald-400 font-medium">
              <CheckCircle2 className="w-3.5 h-3.5 shrink-0" />
              <span>跨源去重与规则清洗已启用</span>
            </div>
          </div>

          <div className="hero-card p-5 space-y-3">
            <div className="flex items-center justify-between">
              <span className="text-xs font-semibold text-zinc-500 dark:text-zinc-400">节点池总量</span>
              <div className="w-8 h-8 rounded-lg bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 flex items-center justify-center">
                <Activity className="w-4 h-4" />
              </div>
            </div>
            <div className="flex items-baseline gap-1.5">
              <span className="text-3xl font-extrabold font-mono tabular-nums tracking-tight text-zinc-900 dark:text-white">
                {stats.total_node_count}
              </span>
              <span className="text-xs text-zinc-400">个节点</span>
            </div>
            <div className="pt-2.5 border-t border-zinc-100 dark:border-white/[0.06] text-xs text-zinc-500 dark:text-zinc-400">
              <span>支持 9 种主流代理协议聚合</span>
            </div>
          </div>

          <div className="hero-card p-5 space-y-3">
            <div className="flex items-center justify-between">
              <span className="text-xs font-semibold text-zinc-500 dark:text-zinc-400">系统防护引擎</span>
              <div className="w-8 h-8 rounded-lg bg-zinc-100 dark:bg-white/10 text-zinc-700 dark:text-zinc-300 flex items-center justify-center">
                <ShieldCheck className="w-4 h-4" />
              </div>
            </div>
            <div className="flex items-baseline gap-1.5 h-9">
              <span className="text-lg font-bold font-mono tracking-tight text-zinc-900 dark:text-white">
                RateLimit + WAL
              </span>
            </div>
            <div className="pt-2.5 border-t border-zinc-100 dark:border-white/[0.06] flex items-center justify-between text-xs text-zinc-500 dark:text-zinc-400">
              <span>防暴破与请求频控</span>
              <span className="text-emerald-600 dark:text-emerald-400 font-medium">运行中</span>
            </div>
          </div>
        </motion.div>
      )}

      {/* Quick Distribution Panel */}
      <div className="hero-card overflow-hidden">
        <div className="px-6 py-4 border-b border-zinc-100 dark:border-white/[0.06] flex items-center justify-between">
          <div className="flex items-center gap-2.5">
            <h2 className="text-sm font-bold text-zinc-900 dark:text-white">客户端订阅分发</h2>
            <span className="hero-chip-default text-xs font-medium tabular-nums">
              共 <span className="font-mono font-bold text-zinc-700 dark:text-zinc-200 mx-0.5">{profiles.length}</span> 组
            </span>
          </div>
          <Link
            to="/profiles"
            className="text-xs text-[#2563EB] dark:text-[#60A5FA] font-semibold hover:underline flex items-center gap-1"
          >
            <span>管理全部组合</span>
            <ArrowRight className="w-3.5 h-3.5" />
          </Link>
        </div>

        {loading ? (
          <RowListSkeleton count={3} />
        ) : profiles.length === 0 ? (
          <div className="p-12 text-center space-y-3">
            <div className="w-12 h-12 rounded-2xl bg-zinc-100 dark:bg-white/5 flex items-center justify-center text-zinc-400 mx-auto">
              <Layers className="w-6 h-6" />
            </div>
            <div className="text-sm font-semibold text-zinc-800 dark:text-zinc-200">尚未创建任何订阅组合</div>
            <p className="text-xs text-zinc-400 max-w-sm mx-auto leading-relaxed">
              添加订阅组合后，系统将自动汇总去重节点并生成对应的客户端订阅链接。
            </p>
            <Link to="/profiles" className="hero-btn-primary px-4 py-2 text-xs font-semibold inline-flex">
              创建第一个组合
            </Link>
          </div>
        ) : (
          <div className="divide-y divide-zinc-100 dark:divide-white/[0.06]">
            {profiles.map((p) => {
              const subNames = getSubNames(p.subscription_ids)
              return (
                <div
                  key={p.id}
                  className="p-5 flex flex-col lg:flex-row lg:items-center justify-between gap-4 hover:bg-zinc-50/70 dark:hover:bg-white/[0.02] transition-colors"
                >
                  {/* Left Column */}
                  <div className="space-y-2 min-w-0">
                    <div className="flex items-center gap-2.5 flex-wrap">
                      <span className="font-bold text-[15px] text-zinc-900 dark:text-white tracking-tight">
                        {p.name}
                      </span>
                      <span className="hero-chip-primary text-xs tabular-nums">
                        <span className="font-mono font-bold mr-1">{p.node_count || 0}</span>
                        <span>节点</span>
                      </span>
                      <span className="inline-flex items-center gap-1.5 text-xs text-emerald-600 dark:text-emerald-400 font-medium">
                        <span className="w-1.5 h-1.5 rounded-full bg-emerald-500" />
                        <span>正常</span>
                      </span>
                    </div>

                    <div className="flex items-center gap-1.5 flex-wrap text-xs">
                      <span className="text-zinc-400">聚合源:</span>
                      {subNames.length > 0 ? (
                        subNames.map((sname, idx) => (
                          <span key={idx} className="hero-chip-default text-[11px]">
                            {sname}
                          </span>
                        ))
                      ) : (
                        <span className="text-zinc-400 italic text-[11px]">(全部可用订阅源)</span>
                      )}
                    </div>

                    <div className="flex items-center gap-2 text-xs text-zinc-400">
                      <span className="font-mono bg-zinc-100 dark:bg-white/5 px-2 py-0.5 rounded-md border border-zinc-200/60 dark:border-white/10">
                        Token: {p.token.slice(0, 8)}...{p.token.slice(-6)}
                      </span>
                      <button
                        onClick={() => copyUrl(getUniversalUrl(p.token), '自适应订阅链接已复制！')}
                        title="复制完整链接"
                        className="hover:text-zinc-700 dark:hover:text-zinc-200 p-1 rounded transition-colors"
                      >
                        <Copy className="w-3.5 h-3.5" />
                      </button>
                    </div>
                  </div>

                  {/* Right Column */}
                  <div className="flex items-center gap-2 flex-wrap shrink-0">
                    <button
                      onClick={() => copyUrl(getUniversalUrl(p.token), '万能自适应链接已复制，可直接粘贴进客户端！', p.id)}
                      className={`hero-btn-primary px-3.5 py-2 text-xs font-semibold gap-1.5 transition-all ${
                        copiedProfileId === p.id ? '!bg-emerald-600' : ''
                      }`}
                      title="智能自适应 Clash / Sing-box / 浏览器引导页"
                    >
                      {copiedProfileId === p.id ? (
                        <>
                          <CheckCircle2 className="w-3.5 h-3.5 text-white" />
                          <span>已复制链接</span>
                        </>
                      ) : (
                        <>
                          <Zap className="w-3.5 h-3.5 text-amber-300" />
                          <span>复制自适应链接</span>
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
                        <span>格式导出</span>
                        <ChevronDown className="w-3.5 h-3.5 text-zinc-400" />
                      </button>

                      {activeDropdown === p.id && (
                        <div className="absolute right-0 mt-2 w-52 rounded-xl bg-white dark:bg-[#181C26] border border-zinc-200 dark:border-white/10 shadow-xl py-1 z-30 text-xs">
                          <button
                            onClick={() => {
                              copyUrl(getExportUrl(p.token, 'clash', true), 'Clash 全配置订阅链接已复制！')
                              setActiveDropdown(null)
                            }}
                            className="w-full text-left px-3.5 py-2 hover:bg-zinc-100 dark:hover:bg-white/5 text-zinc-700 dark:text-zinc-200 flex items-center justify-between"
                          >
                            <span>Clash 全配置</span>
                            <span className="text-[10px] font-mono text-zinc-400">YAML</span>
                          </button>
                          <button
                            onClick={() => {
                              copyUrl(getExportUrl(p.token, 'singbox', true), 'Sing-box 全配置订阅链接已复制！')
                              setActiveDropdown(null)
                            }}
                            className="w-full text-left px-3.5 py-2 hover:bg-zinc-100 dark:hover:bg-white/5 text-zinc-700 dark:text-zinc-200 flex items-center justify-between"
                          >
                            <span>Sing-box 全配置</span>
                            <span className="text-[10px] font-mono text-zinc-400">JSON</span>
                          </button>
                          <button
                            onClick={() => {
                              copyUrl(getExportUrl(p.token, 'clash', false), 'Clash Provider 订阅链接已复制！')
                              setActiveDropdown(null)
                            }}
                            className="w-full text-left px-3.5 py-2 hover:bg-zinc-100 dark:hover:bg-white/5 text-zinc-700 dark:text-zinc-200 flex items-center justify-between"
                          >
                            <span>Clash Provider 节点</span>
                            <span className="text-[10px] font-mono text-zinc-400">Provider</span>
                          </button>
                          <button
                            onClick={() => {
                              copyUrl(getExportUrl(p.token, 'base64', false), 'Base64 订阅链接已复制！')
                              setActiveDropdown(null)
                            }}
                            className="w-full text-left px-3.5 py-2 hover:bg-zinc-100 dark:hover:bg-white/5 text-zinc-700 dark:text-zinc-200 flex items-center justify-between"
                          >
                            <span>通用 Base64 / URI</span>
                            <span className="text-[10px] font-mono text-zinc-400">Base64</span>
                          </button>
                        </div>
                      )}
                    </div>

                    <button
                      onClick={() => setQrState({ open: true, token: p.token, name: p.name })}
                      title="扫码添加订阅"
                      className="hero-btn-bordered p-2 text-zinc-600 dark:text-zinc-300"
                    >
                      <QrCode className="w-4 h-4" />
                    </button>

                    <Link
                      to="/profiles"
                      title="管理或编辑该组合"
                      className="hero-btn-bordered p-2 text-zinc-600 dark:text-zinc-300"
                    >
                      <Eye className="w-4 h-4" />
                    </Link>
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
    </div>
  )
}
