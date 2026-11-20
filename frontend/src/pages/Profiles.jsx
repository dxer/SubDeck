import React, { useState, useEffect, useMemo, useCallback } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import {
  Layers,
  Plus,
  Zap,
  ChevronDown,
  QrCode,
  RotateCcw,
  Eye,
  Pencil,
  Trash2,
  Copy,
  CheckCircle2,
  X
} from 'lucide-react'
import { api } from '../api'
import { Toast, useToast } from '../components/Toast'
import ConfirmDialog from '../components/ConfirmDialog'
import QrModal from '../components/QrModal'
import NodesModal from '../components/NodesModal'
import { RowListSkeleton } from '../components/Skeleton'
import { getUniversalUrl, getExportUrl } from '../utils/subUrl'

const SUPPORTED_PROTOCOLS = ['ss', 'vmess', 'vless', 'trojan', 'hysteria2', 'tuic', 'anytls', 'wireguard', 'snell']

export default function Profiles() {
  const [profiles, setProfiles] = useState([])
  const [allSubscriptions, setAllSubscriptions] = useState([])
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [activeDropdown, setActiveDropdown] = useState(null)
  const [copiedProfileId, setCopiedProfileId] = useState(null)
  const { toast, showToast } = useToast()

  // Create / Edit Modal State
  const [showModal, setShowModal] = useState(false)
  const [isEditing, setIsEditing] = useState(false)
  const [editingId, setEditingId] = useState('')
  const [modalTab, setModalTab] = useState('sources')
  const [form, setForm] = useState({
    name: '',
    subscription_ids: [],
    deduplicate: true,
    allowed_protocols: [],
    include_regex: '',
    exclude_regex: '',
    rename_prefix: '',
    rename_suffix: '',
    rename_regex_search: '',
    rename_regex_replace: ''
  })

  // QR & Preview & Confirm Modal State
  const [qrState, setQrState] = useState({ open: false, token: '', name: '' })
  const [previewState, setPreviewState] = useState({ open: false, name: '', nodes: [], loading: false })
  const [confirmState, setConfirmState] = useState({ open: false, type: '', profile: null, loading: false })

  const copyText = async (text, msg, profileId = null) => {
    try {
      await navigator.clipboard.writeText(text)
      showToast(msg || '已复制到剪贴板！')
      if (profileId) {
        setCopiedProfileId(profileId)
        setTimeout(() => setCopiedProfileId(null), 1800)
      }
    } catch {
      showToast('复制失败，请手动长按复制', 'error')
    }
  }

  const loadData = useCallback(async () => {
    setLoading(true)
    try {
      const [profilesRes, subsRes] = await Promise.all([
        api.getProfiles(),
        api.getSubscriptions()
      ])
      setProfiles(Array.isArray(profilesRes) ? profilesRes : (profilesRes?.data || []))
      setAllSubscriptions(Array.isArray(subsRes) ? subsRes : (subsRes?.data || []))
    } catch (err) {
      showToast(err.message || '加载组合数据失败', 'error')
    } finally {
      setLoading(false)
    }
  }, [showToast])

  useEffect(() => {
    loadData()
  }, [loadData])

  useEffect(() => {
    const handleOutsideClick = (e) => {
      if (!e.target.closest('.format-dropdown-container')) {
        setActiveDropdown(null)
      }
    }
    window.addEventListener('click', handleOutsideClick)
    return () => window.removeEventListener('click', handleOutsideClick)
  }, [])

  const getSubNames = (ids) => {
    if (!ids || ids.length === 0) return []
    return ids.map((id) => {
      const sub = allSubscriptions.find((s) => s.id === id)
      return sub ? sub.name : `源 #${id}`
    })
  }

  const previewRenamedText = useMemo(() => {
    let sample = 'IEPL-香港 01'
    if (form.rename_regex_search) {
      try {
        const re = new RegExp(form.rename_regex_search)
        sample = sample.replace(re, form.rename_regex_replace || '')
      } catch {
        // invalid regex, ignore
      }
    }
    return `${form.rename_prefix || ''}${sample}${form.rename_suffix || ''}`
  }, [form.rename_prefix, form.rename_suffix, form.rename_regex_search, form.rename_regex_replace])

  const openModal = (p = null) => {
    if (p) {
      setIsEditing(true)
      setEditingId(p.id)
      setForm({
        name: p.name || '',
        subscription_ids: p.subscription_ids ? [...p.subscription_ids] : [],
        deduplicate: p.deduplicate !== false,
        allowed_protocols: p.allowed_protocols ? [...p.allowed_protocols] : [],
        include_regex: p.include_regex || '',
        exclude_regex: p.exclude_regex || '',
        rename_prefix: p.rename_prefix || '',
        rename_suffix: p.rename_suffix || '',
        rename_regex_search: p.rename_regex_search || '',
        rename_regex_replace: p.rename_regex_replace || ''
      })
    } else {
      setIsEditing(false)
      setEditingId('')
      setForm({
        name: '',
        subscription_ids: allSubscriptions.map((s) => s.id),
        deduplicate: true,
        allowed_protocols: [],
        include_regex: '',
        exclude_regex: '',
        rename_prefix: '',
        rename_suffix: '',
        rename_regex_search: '',
        rename_regex_replace: ''
      })
    }
    setModalTab('sources')
    setShowModal(true)
  }

  const toggleSubscription = (id) => {
    const list = [...form.subscription_ids]
    const idx = list.indexOf(id)
    if (idx > -1) list.splice(idx, 1)
    else list.push(id)
    setForm({ ...form, subscription_ids: list })
  }

  const toggleProtocol = (proto) => {
    const list = [...form.allowed_protocols]
    const idx = list.indexOf(proto)
    if (idx > -1) list.splice(idx, 1)
    else list.push(proto)
    setForm({ ...form, allowed_protocols: list })
  }

  const handleSubmit = async (e) => {
    e.preventDefault()
    if (!form.name.trim()) {
      showToast('请输入组合名称', 'error')
      return
    }

    setSaving(true)
    try {
      if (isEditing) {
        await api.updateProfile(editingId, form)
        showToast('订阅组合已更新')
      } else {
        await api.createProfile(form)
        showToast('订阅组合创建成功')
      }
      setShowModal(false)
      await loadData()
    } catch (err) {
      showToast(err.message || '保存失败', 'error')
    } finally {
      setSaving(false)
    }
  }

  const handleConfirmAction = async () => {
    const { type, profile } = confirmState
    if (!profile) return
    setConfirmState((prev) => ({ ...prev, loading: true }))
    try {
      if (type === 'delete') {
        await api.deleteProfile(profile.id)
        showToast('订阅组合已删除')
      } else if (type === 'reset') {
        await api.resetProfileToken(profile.id)
        showToast('安全 Token 已重新生成并生效')
      }
      setConfirmState({ open: false, type: '', profile: null, loading: false })
      await loadData()
    } catch (err) {
      showToast(err.message || '操作失败', 'error')
      setConfirmState((prev) => ({ ...prev, loading: false }))
    }
  }

  const openPreview = async (p) => {
    setPreviewState({ open: true, name: `预览聚合节点: ${p.name}`, nodes: [], loading: true })
    try {
      const res = await api.previewProfile(p.id)
      const nodes = Array.isArray(res) ? res : (res?.nodes || [])
      setPreviewState({ open: true, name: `预览聚合节点: ${p.name}`, nodes, loading: false })
    } catch (err) {
      showToast(err.message || '获取节点预览失败', 'error')
      setPreviewState({ open: true, name: `预览聚合节点: ${p.name}`, nodes: [], loading: false })
    }
  }

  return (
    <div className="space-y-6 max-w-7xl mx-auto">
      <Toast toast={toast} />

      {/* Header Banner */}
      <div className="hero-card flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4 p-5">
        <div className="flex items-center gap-3">
          <h1 className="text-lg sm:text-xl font-bold tracking-tight text-zinc-900 dark:text-white">
            订阅组合管理
          </h1>
          <span className="hero-chip-default text-xs font-medium tabular-nums">
            共 <span className="font-mono font-bold text-zinc-700 dark:text-zinc-200 mx-0.5">{profiles.length}</span> 组配置
          </span>
        </div>
        <button
          onClick={() => openModal()}
          className="hero-btn-primary px-4 py-2 text-xs font-semibold gap-1.5 shrink-0"
        >
          <Plus className="w-4 h-4" />
          <span>新建订阅组合</span>
        </button>
      </div>

      {/* Loading / Empty / List */}
      {loading ? (
        <div className="hero-card">
          <RowListSkeleton count={3} />
        </div>
      ) : profiles.length === 0 ? (
        <div className="p-16 hero-card text-center space-y-3">
          <div className="w-12 h-12 rounded-2xl bg-zinc-100 dark:bg-white/5 flex items-center justify-center text-zinc-400 mx-auto">
            <Layers className="w-6 h-6" />
          </div>
          <div>
            <h3 className="text-sm font-bold text-zinc-900 dark:text-white">尚未创建任何订阅组合</h3>
            <p className="text-xs text-zinc-400 max-w-sm mx-auto mt-1 leading-relaxed">
              订阅组合可勾选多个订阅源，自动完成跨订阅去重与正则过滤，生成稳定唯一的客户端订阅。
            </p>
          </div>
          <button
            onClick={() => openModal()}
            className="hero-btn-primary px-4 py-2 text-xs font-semibold"
          >
            立即新建
          </button>
        </div>
      ) : (
        <div className="space-y-4">
          {profiles.map((p) => {
            const subNames = getSubNames(p.subscription_ids)
            return (
              <motion.div
                key={p.id}
                layout
                initial={{ opacity: 0, y: 10 }}
                animate={{ opacity: 1, y: 0 }}
                className="hero-card hero-card-interactive p-6 space-y-4"
              >
                {/* Top Info Header */}
                <div className="flex flex-col sm:flex-row sm:items-start justify-between gap-3">
                  <div className="space-y-2 min-w-0">
                    <div className="flex items-center gap-3 flex-wrap">
                      <h3 className="font-bold text-base text-zinc-900 dark:text-white tracking-tight">{p.name}</h3>
                      <span className="hero-chip-primary text-xs tabular-nums">
                        <span className="font-mono font-bold mr-1">{p.node_count || 0}</span>
                        <span>节点</span>
                      </span>
                      <span className="inline-flex items-center gap-1.5 text-xs text-emerald-600 dark:text-emerald-400 font-medium">
                        <span className="w-1.5 h-1.5 rounded-full bg-emerald-500" />
                        <span>正常在线</span>
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
                        <span className="text-zinc-400 italic text-[11px]">(聚合全部可用订阅源)</span>
                      )}
                    </div>

                    <div className="flex items-center gap-1.5 flex-wrap text-[11px] pt-0.5">
                      {p.deduplicate && <span className="hero-chip-default">智能去重</span>}
                      {p.allowed_protocols && p.allowed_protocols.length > 0 && (
                        <span className="hero-chip-default">
                          协议: <span className="font-mono ml-1">{p.allowed_protocols.join(', ')}</span>
                        </span>
                      )}
                      {p.include_regex && (
                        <span className="hero-chip-default">
                          包含: <span className="font-mono ml-1">/{p.include_regex}/</span>
                        </span>
                      )}
                      {p.exclude_regex && (
                        <span className="hero-chip-default">
                          排除: <span className="font-mono ml-1">/{p.exclude_regex}/</span>
                        </span>
                      )}
                      {(p.rename_prefix || p.rename_suffix || p.rename_regex_search) && (
                        <span className="hero-chip-default">已启用重命名</span>
                      )}
                    </div>
                  </div>

                  {/* Token & Quick Actions */}
                  <div className="flex items-center gap-1.5 shrink-0">
                    <div className="flex items-center gap-1.5 text-xs font-mono text-zinc-400 px-3 py-1.5 rounded-xl bg-zinc-50 dark:bg-white/[0.04] border border-zinc-200/80 dark:border-white/10">
                      <span>Token: {p.token ? `${p.token.slice(0, 6)}...${p.token.slice(-4)}` : '------'}</span>
                      <button
                        onClick={() => copyText(getUniversalUrl(p.token), '自适应订阅链接已复制！')}
                        title="复制完整 Token 订阅链接"
                        className="hover:text-zinc-700 dark:hover:text-white transition-colors"
                      >
                        <Copy className="w-3.5 h-3.5" />
                      </button>
                    </div>
                    <button
                      onClick={() => setConfirmState({ open: true, type: 'reset', profile: p, loading: false })}
                      title="吊销并重新生成安全 Token"
                      className="hero-btn-default p-2 text-zinc-400 hover:text-amber-500"
                    >
                      <RotateCcw className="w-4 h-4" />
                    </button>
                    <button
                      onClick={() => openPreview(p)}
                      title="预览组合节点"
                      className="hero-btn-default p-2 text-zinc-400 hover:text-zinc-900 dark:hover:text-white"
                    >
                      <Eye className="w-4 h-4" />
                    </button>
                    <button
                      onClick={() => openModal(p)}
                      title="编辑组合配置"
                      className="hero-btn-default p-2 text-zinc-400 hover:text-zinc-900 dark:hover:text-white"
                    >
                      <Pencil className="w-4 h-4" />
                    </button>
                    <button
                      onClick={() => setConfirmState({ open: true, type: 'delete', profile: p, loading: false })}
                      title="删除该组合"
                      className="hero-btn-default p-2 text-zinc-400 hover:text-rose-600 hover:bg-rose-500/10"
                    >
                      <Trash2 className="w-4 h-4" />
                    </button>
                  </div>
                </div>

                {/* Action Ribbon */}
                <div className="pt-3 border-t border-zinc-100 dark:border-white/[0.06] flex flex-col sm:flex-row sm:items-center justify-between gap-3">
                  <div className="flex items-center gap-2.5 flex-wrap">
                    <button
                      onClick={() => copyText(getUniversalUrl(p.token), '万能自适应链接已复制，可直接粘贴进客户端！', p.id)}
                      className={`hero-btn-primary px-4 py-2 text-xs font-semibold gap-1.5 transition-all ${
                        copiedProfileId === p.id ? '!bg-emerald-600' : ''
                      }`}
                      title="智能自适应下发 Clash 或 Sing-box 配置，浏览器打开显示引导页"
                    >
                      {copiedProfileId === p.id ? (
                        <>
                          <CheckCircle2 className="w-4 h-4 text-white" />
                          <span>已复制链接</span>
                        </>
                      ) : (
                        <>
                          <Zap className="w-4 h-4 text-amber-300" />
                          <span>复制自适应链接 (推荐)</span>
                        </>
                      )}
                    </button>

                    <div className="relative format-dropdown-container">
                      <button
                        onClick={(e) => {
                          e.stopPropagation()
                          setActiveDropdown(activeDropdown === p.id ? null : p.id)
                        }}
                        className="hero-btn-bordered px-3.5 py-2 text-xs font-semibold gap-1"
                      >
                        <span>格式导出</span>
                        <ChevronDown className="w-3.5 h-3.5 text-zinc-400" />
                      </button>

                      {activeDropdown === p.id && (
                        <div className="absolute left-0 mt-2 w-52 rounded-xl bg-white dark:bg-[#181C26] border border-zinc-200 dark:border-white/10 shadow-xl py-1 z-30 text-xs">
                          <button
                            onClick={() => {
                              copyText(getExportUrl(p.token, 'clash', true), 'Clash 全配置订阅链接已复制！')
                              setActiveDropdown(null)
                            }}
                            className="w-full text-left px-3.5 py-2 hover:bg-zinc-100 dark:hover:bg-white/5 text-zinc-700 dark:text-zinc-200 flex items-center justify-between"
                          >
                            <span>Clash 全配置</span>
                            <span className="text-[10px] font-mono text-zinc-400">YAML</span>
                          </button>
                          <button
                            onClick={() => {
                              copyText(getExportUrl(p.token, 'singbox', true), 'Sing-box 全配置订阅链接已复制！')
                              setActiveDropdown(null)
                            }}
                            className="w-full text-left px-3.5 py-2 hover:bg-zinc-100 dark:hover:bg-white/5 text-zinc-700 dark:text-zinc-200 flex items-center justify-between"
                          >
                            <span>Sing-box 全配置</span>
                            <span className="text-[10px] font-mono text-zinc-400">JSON</span>
                          </button>
                          <button
                            onClick={() => {
                              copyText(getExportUrl(p.token, 'clash', false), 'Clash Provider 订阅链接已复制！')
                              setActiveDropdown(null)
                            }}
                            className="w-full text-left px-3.5 py-2 hover:bg-zinc-100 dark:hover:bg-white/5 text-zinc-700 dark:text-zinc-200 flex items-center justify-between"
                          >
                            <span>Clash Provider 节点</span>
                            <span className="text-[10px] font-mono text-zinc-400">Provider</span>
                          </button>
                          <button
                            onClick={() => {
                              copyText(getExportUrl(p.token, 'base64', false), 'Base64 订阅链接已复制！')
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
                      className="hero-btn-bordered px-3 py-2 text-xs font-medium gap-1.5"
                    >
                      <QrCode className="w-3.5 h-3.5 text-zinc-500" />
                      <span>扫码</span>
                    </button>
                  </div>

                  <div className="text-xs text-zinc-400">
                    建议更新周期 <span className="font-mono text-zinc-600 dark:text-zinc-300">24h</span>
                  </div>
                </div>
              </motion.div>
            )
          })}
        </div>
      )}

      {/* Create / Edit Modal */}
      <AnimatePresence>
        {showModal && (
          <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
            <motion.div
              initial={{ opacity: 0 }}
              animate={{ opacity: 1 }}
              exit={{ opacity: 0 }}
              onClick={() => setShowModal(false)}
              className="fixed inset-0 bg-black/55 backdrop-blur-sm"
            />
            <motion.div
              initial={{ opacity: 0, scale: 0.96, y: 10 }}
              animate={{ opacity: 1, scale: 1, y: 0 }}
              exit={{ opacity: 0, scale: 0.96, y: 10 }}
              transition={{ duration: 0.16, ease: [0.16, 1, 0.3, 1] }}
              className="relative z-10 w-full max-w-2xl bg-white dark:bg-[#11141B] rounded-2xl shadow-2xl border border-zinc-200/80 dark:border-white/10 overflow-hidden flex flex-col max-h-[90vh]"
            >
              <div className="px-6 py-4 border-b border-zinc-100 dark:border-white/[0.06] flex items-center justify-between">
                <h2 className="text-base font-bold text-zinc-900 dark:text-white">
                  {isEditing ? '编辑订阅组合' : '创建新订阅组合'}
                </h2>
                <button
                  onClick={() => setShowModal(false)}
                  className="p-1 rounded-lg text-zinc-400 hover:text-zinc-600 dark:hover:text-white transition-colors"
                >
                  <X className="w-4 h-4" />
                </button>
              </div>

              <div className="flex p-2 border-b border-zinc-100 dark:border-white/[0.06] bg-zinc-50/70 dark:bg-white/[0.02] gap-1 text-xs">
                {[
                  { key: 'sources', label: `1. 订阅源选择 (${form.subscription_ids.length})` },
                  { key: 'filters', label: '2. 协议与规则过滤' },
                  { key: 'rename', label: '3. 节点重命名与高级' }
                ].map((tab) => (
                  <button
                    key={tab.key}
                    type="button"
                    onClick={() => setModalTab(tab.key)}
                    className={`relative flex-1 py-2 px-3 rounded-xl font-bold transition-colors text-center ${
                      modalTab === tab.key
                        ? 'text-[#2563EB] dark:text-white'
                        : 'text-zinc-500 hover:text-zinc-900 dark:hover:text-white'
                    }`}
                  >
                    {modalTab === tab.key && (
                      <motion.span
                        layoutId="profile-modal-tab"
                        transition={{ type: 'spring', stiffness: 400, damping: 32 }}
                        className="absolute inset-0 rounded-xl bg-white dark:bg-[#1E2330] shadow-xs border border-zinc-200/60 dark:border-white/10"
                      />
                    )}
                    <span className="relative z-10">{tab.label}</span>
                  </button>
                ))}
              </div>

              <form onSubmit={handleSubmit} className="p-6 overflow-y-auto space-y-4 flex-1">
                <div>
                  <label className="block text-[11px] font-semibold text-zinc-500 uppercase tracking-wider mb-1.5">
                    组合名称
                  </label>
                  <input
                    type="text"
                    required
                    value={form.name}
                    onChange={(e) => setForm({ ...form, name: e.target.value })}
                    placeholder="例如：主力自选聚合 / 港日美高速专线"
                    className="hero-input px-3.5 py-2.5 text-xs text-zinc-900 dark:text-white"
                  />
                </div>

                {modalTab === 'sources' && (
                  <div className="space-y-3">
                    <div className="flex items-center justify-between">
                      <label className="block text-[11px] font-semibold text-zinc-500 uppercase tracking-wider">
                        选择参与聚合的订阅源
                      </label>
                      <div className="space-x-3 text-xs font-semibold">
                        <button
                          type="button"
                          onClick={() => setForm({ ...form, subscription_ids: allSubscriptions.map((s) => s.id) })}
                          className="text-[#2563EB] dark:text-[#60A5FA] hover:underline"
                        >
                          全选
                        </button>
                        <button
                          type="button"
                          onClick={() => setForm({ ...form, subscription_ids: [] })}
                          className="text-zinc-400 hover:underline"
                        >
                          清空
                        </button>
                      </div>
                    </div>

                    {allSubscriptions.length === 0 ? (
                      <div className="p-8 text-center text-xs text-zinc-400 border border-dashed border-zinc-200 dark:border-white/10 rounded-2xl">
                        暂无可用的订阅源，请先前往“订阅管理”添加远程或本地订阅。
                      </div>
                    ) : (
                      <div className="space-y-2 max-h-60 overflow-y-auto pr-1">
                        {allSubscriptions.map((sub) => {
                          const isSelected = form.subscription_ids.includes(sub.id)
                          return (
                            <label
                              key={sub.id}
                              onClick={() => toggleSubscription(sub.id)}
                              className={`flex items-center justify-between p-3 rounded-xl border cursor-pointer transition-all ${
                                isSelected
                                  ? 'border-[#2563EB]/50 bg-[#2563EB]/[0.05] dark:bg-[#2563EB]/12'
                                  : 'border-zinc-200/80 dark:border-white/10 hover:bg-zinc-50 dark:hover:bg-white/[0.03]'
                              }`}
                            >
                              <div className="flex items-center gap-3 min-w-0">
                                <input
                                  type="checkbox"
                                  checked={isSelected}
                                  onChange={() => {}}
                                  className="rounded text-[#2563EB] focus:ring-[#2563EB] w-4 h-4 pointer-events-none"
                                />
                                <span
                                  className={`px-2 py-0.5 rounded-md text-[10px] font-bold uppercase border ${
                                    sub.type === 'local'
                                      ? 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border-emerald-500/20'
                                      : 'bg-[#2563EB]/10 text-[#2563EB] dark:text-[#60A5FA] border-[#2563EB]/20'
                                  }`}
                                >
                                  {sub.type === 'local' ? '本地' : '远程'}
                                </span>
                                <span className="text-xs font-bold text-zinc-800 dark:text-zinc-200 truncate">
                                  {sub.name}
                                </span>
                              </div>
                              <span className="text-xs text-zinc-400 shrink-0 tabular-nums">
                                <span className="font-mono mr-1">{sub.node_count || 0}</span>
                                <span>节点</span>
                              </span>
                            </label>
                          )
                        })}
                      </div>
                    )}
                  </div>
                )}

                {modalTab === 'filters' && (
                  <div className="space-y-4">
                    <label className="flex items-center justify-between p-3.5 rounded-xl border border-zinc-200/80 dark:border-white/10 bg-zinc-50 dark:bg-white/[0.03] cursor-pointer">
                      <div>
                        <span className="text-xs font-bold text-zinc-900 dark:text-white">跨源智能节点去重</span>
                        <p className="text-[11px] text-zinc-400 mt-0.5">
                          依据 IP:Port:Protocol 特征自动消除多订阅源中的重复节点
                        </p>
                      </div>
                      <input
                        type="checkbox"
                        checked={form.deduplicate}
                        onChange={(e) => setForm({ ...form, deduplicate: e.target.checked })}
                        className="rounded text-[#2563EB] focus:ring-[#2563EB] w-4 h-4 cursor-pointer"
                      />
                    </label>

                    <div>
                      <label className="block text-[11px] font-semibold text-zinc-500 uppercase tracking-wider mb-2">
                        协议类型白名单 (未选则默认保留全部)
                      </label>
                      <div className="flex flex-wrap gap-2">
                        {SUPPORTED_PROTOCOLS.map((proto) => {
                          const isAllowed = form.allowed_protocols.includes(proto)
                          return (
                            <button
                              key={proto}
                              type="button"
                              onClick={() => toggleProtocol(proto)}
                              className={`px-3 py-1.5 rounded-xl text-xs font-mono uppercase transition-all ${
                                isAllowed
                                  ? 'bg-[#2563EB] text-white font-bold shadow-xs'
                                  : 'bg-zinc-100 dark:bg-white/5 text-zinc-500 hover:bg-zinc-200 dark:hover:bg-white/10'
                              }`}
                            >
                              {proto}
                            </button>
                          )
                        })}
                      </div>
                    </div>

                    <div className="space-y-3 pt-1">
                      <div>
                        <div className="flex items-center justify-between mb-1.5">
                          <label className="block text-[11px] font-semibold text-zinc-500 uppercase tracking-wider">
                            包含正则表达式 (白名单)
                          </label>
                          <button
                            type="button"
                            onClick={() => setForm({ ...form, include_regex: '香港|日本|新加坡|美国' })}
                            className="text-[11px] text-[#2563EB] dark:text-[#60A5FA] font-semibold hover:underline"
                          >
                            快捷填入: 港/日/新/美
                          </button>
                        </div>
                        <input
                          type="text"
                          value={form.include_regex}
                          onChange={(e) => setForm({ ...form, include_regex: e.target.value })}
                          placeholder="例如：香港|日本|HK|JP"
                          className="hero-input px-3.5 py-2 text-xs font-mono text-zinc-900 dark:text-white"
                        />
                      </div>

                      <div>
                        <div className="flex items-center justify-between mb-1.5">
                          <label className="block text-[11px] font-semibold text-zinc-500 uppercase tracking-wider">
                            排除正则表达式 (黑名单)
                          </label>
                          <button
                            type="button"
                            onClick={() => setForm({ ...form, exclude_regex: '官网|到期|剩余|流量|通知|群' })}
                            className="text-[11px] text-[#2563EB] dark:text-[#60A5FA] font-semibold hover:underline"
                          >
                            快捷填入: 过滤官网与到期
                          </button>
                        </div>
                        <input
                          type="text"
                          value={form.exclude_regex}
                          onChange={(e) => setForm({ ...form, exclude_regex: e.target.value })}
                          placeholder="例如：官网|到期|剩余流量|通知"
                          className="hero-input px-3.5 py-2 text-xs font-mono text-zinc-900 dark:text-white"
                        />
                      </div>
                    </div>
                  </div>
                )}

                {modalTab === 'rename' && (
                  <div className="space-y-4">
                    <div className="grid grid-cols-2 gap-3">
                      <div>
                        <label className="block text-[11px] font-semibold text-zinc-500 uppercase tracking-wider mb-1.5">
                          统一前缀
                        </label>
                        <input
                          type="text"
                          value={form.rename_prefix}
                          onChange={(e) => setForm({ ...form, rename_prefix: e.target.value })}
                          placeholder="例如：[专线] "
                          className="hero-input px-3.5 py-2 text-xs text-zinc-900 dark:text-white"
                        />
                      </div>
                      <div>
                        <label className="block text-[11px] font-semibold text-zinc-500 uppercase tracking-wider mb-1.5">
                          统一后缀
                        </label>
                        <input
                          type="text"
                          value={form.rename_suffix}
                          onChange={(e) => setForm({ ...form, rename_suffix: e.target.value })}
                          placeholder="例如： (主力)"
                          className="hero-input px-3.5 py-2 text-xs text-zinc-900 dark:text-white"
                        />
                      </div>
                    </div>

                    <div className="grid grid-cols-2 gap-3">
                      <div>
                        <label className="block text-[11px] font-semibold text-zinc-500 uppercase tracking-wider mb-1.5">
                          正则查找内容
                        </label>
                        <input
                          type="text"
                          value={form.rename_regex_search}
                          onChange={(e) => setForm({ ...form, rename_regex_search: e.target.value })}
                          placeholder="例如：IEPL-(.*)"
                          className="hero-input px-3.5 py-2 text-xs font-mono text-zinc-900 dark:text-white"
                        />
                      </div>
                      <div>
                        <label className="block text-[11px] font-semibold text-zinc-500 uppercase tracking-wider mb-1.5">
                          替换为
                        </label>
                        <input
                          type="text"
                          value={form.rename_regex_replace}
                          onChange={(e) => setForm({ ...form, rename_regex_replace: e.target.value })}
                          placeholder="例如：香港专线-$1"
                          className="hero-input px-3.5 py-2 text-xs font-mono text-zinc-900 dark:text-white"
                        />
                      </div>
                    </div>

                    <div className="p-3.5 rounded-xl bg-zinc-50 dark:bg-white/[0.03] border border-zinc-200/80 dark:border-white/10 text-xs space-y-1">
                      <span className="text-[11px] text-zinc-400 font-semibold">重命名效果预览:</span>
                      <div className="font-mono text-zinc-600 dark:text-zinc-300">
                        <span>IEPL-香港 01 </span>
                        <span className="text-zinc-400">&rarr;</span>
                        <span className="font-bold text-[#2563EB] dark:text-[#60A5FA] ml-1">
                          {previewRenamedText}
                        </span>
                      </div>
                    </div>
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

      <QrModal
        open={qrState.open}
        token={qrState.token}
        profileName={qrState.name}
        onClose={() => setQrState({ open: false, token: '', name: '' })}
        onCopy={copyText}
      />

      <NodesModal
        open={previewState.open}
        title={previewState.name}
        nodes={previewState.nodes}
        loading={previewState.loading}
        emptyText="该组合当前未产生任何可用节点，请检查订阅源或过滤规则。"
        onClose={() => setPreviewState({ open: false, name: '', nodes: [], loading: false })}
        onCopy={copyText}
      />

      <ConfirmDialog
        open={confirmState.open}
        title={confirmState.type === 'delete' ? '确认删除订阅组合？' : '确认吊销并重置 Token？'}
        description={
          confirmState.type === 'delete'
            ? `确定要删除组合 "${confirmState.profile?.name}" 吗？删除后对应的分发链接与 Token 将永久失效。`
            : `吊销组合 "${confirmState.profile?.name}" 的 Token 后，旧的客户端订阅链接将立即失效，需重新导入客户端。`
        }
        confirmText={confirmState.type === 'delete' ? '确认删除' : '确认重置'}
        variant={confirmState.type === 'delete' ? 'danger' : 'warning'}
        loading={confirmState.loading}
        onConfirm={handleConfirmAction}
        onCancel={() => setConfirmState({ open: false, type: '', profile: null, loading: false })}
      />
    </div>
  )
}
