import React, { useState, useEffect, useMemo, useCallback } from 'react'
import { motion } from 'framer-motion'
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
  FileCode2,
  AlertCircle,
  Search,
  Sparkles
} from 'lucide-react'
import { api } from '../api'
import { useI18n } from '../i18n'
import { Toast, useToast } from '../components/Toast'
import ConfirmDialog from '../components/ConfirmDialog'
import QrModal from '../components/QrModal'
import NodesModal from '../components/NodesModal'
import ConfigPreviewDrawer from '../components/ConfigPreviewDrawer'
import Drawer from '../components/Drawer'
import { RowListSkeleton } from '../components/Skeleton'
import { getUniversalUrl, getExportUrl, getProtocolBadge, copyToClipboard } from '../utils/subUrl'

const SUPPORTED_PROTOCOLS = ['ss', 'vmess', 'vless', 'trojan', 'hysteria2', 'tuic', 'anytls', 'wireguard', 'snell']

function validateRegex(pattern) {
  if (!pattern || !pattern.trim()) return ''
  try {
    new RegExp(pattern)
    return ''
  } catch (err) {
    return err.message || 'Invalid regex'
  }
}

export default function Profiles() {
  const { t, lang } = useI18n()
  const [profiles, setProfiles] = useState([])
  const [allSubscriptions, setAllSubscriptions] = useState([])
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [activeDropdown, setActiveDropdown] = useState(null)
  const [copiedProfileId, setCopiedProfileId] = useState(null)
  const { toast, showToast } = useToast()

  // Drawer Create / Edit State
  const [showDrawer, setShowDrawer] = useState(false)
  const [isEditing, setIsEditing] = useState(false)
  const [editingId, setEditingId] = useState('')
  const [modalTab, setModalTab] = useState('sources')
  const [form, setForm] = useState({
    name: '',
    subscription_ids: [],
    subscription_selection_mode: 'explicit',
    deduplicate: true,
    filter_info_nodes: false,
    only_healthy: false,
    allowed_protocols: [],
    include_regex: '',
    exclude_regex: '',
    rename_prefix: '',
    rename_suffix: '',
    rename_regex_search: '',
    rename_regex_replace: '',
    sort_mode: '',
    regex_sort: '',
    region_include: '',
    region_exclude: '',
    region_groups: false,
    rule_preset: 'cn_direct',
    export_mode: 'full',
    selected_nodes: {},
    selected_node_keys: {},
    override_sni: '',
    override_sni_enabled: false,
    override_insecure: 'unset',
    override_udp: 'unset',
    clash_template: '',
    clash_template_url: '',
    singbox_template: '',
    singbox_template_url: '',
    template_status: ''
  })

  // Live Draft Preview inside Drawer
  const [draftPreview, setDraftPreview] = useState({ count: 0, nodes: [], loading: false })
  const [showDraftSamples, setShowDraftSamples] = useState(false)

  // Per-subscription node selection in the sources tab (lazy-loaded node lists)
  const [expandedSubs, setExpandedSubs] = useState({})
  const [subNodes, setSubNodes] = useState({})
  const [loadingNodes, setLoadingNodes] = useState({})
  const [nodeFilter, setNodeFilter] = useState({})

  // Modals / Drawers
  const [qrState, setQrState] = useState({ open: false, token: '', name: '' })
  const [previewState, setPreviewState] = useState({ open: false, profileId: '', name: '', nodes: [], loading: false })
  const [probingModal, setProbingModal] = useState(false)
  const [previewConfigProfile, setPreviewConfigProfile] = useState(null)
  const [confirmState, setConfirmState] = useState({ open: false, type: '', profile: null, loading: false })

  const copyText = async (text, msg, profileId = null) => {
    const ok = await copyToClipboard(text)
    if (ok) {
      showToast(msg || t('common.copied'))
      if (profileId) {
        setCopiedProfileId(profileId)
        setTimeout(() => setCopiedProfileId(null), 1800)
      }
    } else {
      showToast(t('common.copyFailed'), 'error')
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
      showToast(err.message || 'Load failed', 'error')
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

  const regexErrors = useMemo(() => {
    const sortLines = (form.regex_sort || '').split('\n')
    let sortErr = ''
    for (const line of sortLines) {
      const trimmed = line.trim()
      if (trimmed) {
        const err = validateRegex(trimmed)
        if (err) {
          sortErr = err
          break
        }
      }
    }
    return {
      include: validateRegex(form.include_regex),
      exclude: validateRegex(form.exclude_regex),
      rename: validateRegex(form.rename_regex_search),
      sort: sortErr
    }
  }, [form.include_regex, form.exclude_regex, form.rename_regex_search, form.regex_sort])
  const hasRegexError = Boolean(regexErrors.include || regexErrors.exclude || regexErrors.rename || regexErrors.sort)

  useEffect(() => {
    if (!showDrawer || hasRegexError) return
    let cancelled = false
    setDraftPreview((prev) => ({ ...prev, loading: true }))
    const timer = setTimeout(async () => {
      try {
        const res = await api.previewDraftProfile(buildProfilePayload())
        if (!cancelled) {
          setDraftPreview({
            count: res?.count ?? 0,
            nodes: Array.isArray(res?.nodes) ? res.nodes : [],
            loading: false
          })
        }
      } catch {
        if (!cancelled) setDraftPreview((prev) => ({ ...prev, loading: false }))
      }
    }, 240)

    return () => {
      cancelled = true
      clearTimeout(timer)
    }
  }, [showDrawer, form, hasRegexError])

  const getSubNames = (ids) => {
    if (!ids || ids.length === 0) return []
    return ids.map((id) => {
      const sub = allSubscriptions.find((s) => s.id === id)
      return sub ? sub.name : `#${id.slice(0, 6)}`
    })
  }

  const realSampleNodeName = draftPreview.nodes?.[0]?.name || ''

  const openDrawer = (p = null) => {
    if (p) {
      setIsEditing(true)
      setEditingId(p.id)
      setForm({
        name: p.name || '',
        subscription_ids: p.subscription_ids ? [...p.subscription_ids] : [],
        subscription_selection_mode: p.subscription_selection_mode === 'explicit' ? 'explicit' : 'all',
        deduplicate: p.deduplicate !== false,
        filter_info_nodes: Boolean(p.filter_info_nodes),
        only_healthy: Boolean(p.only_healthy),
        allowed_protocols: p.allowed_protocols ? [...p.allowed_protocols] : [],
        include_regex: p.include_regex || '',
        exclude_regex: p.exclude_regex || '',
        rename_prefix: p.rename_prefix || '',
        rename_suffix: p.rename_suffix || '',
        rename_regex_search: p.rename_regex_search || '',
        rename_regex_replace: p.rename_regex_replace || '',
        sort_mode: p.sort_mode || '',
        regex_sort: Array.isArray(p.regex_sort) ? p.regex_sort.join('\n') : '',
        region_include: Array.isArray(p.region_include) ? p.region_include.join(', ') : '',
        region_exclude: Array.isArray(p.region_exclude) ? p.region_exclude.join(', ') : '',
        region_groups: Boolean(p.region_groups),
        rule_preset: p.rule_preset || 'cn_direct',
        export_mode: p.export_mode === 'provider' ? 'provider' : 'full',
        selected_nodes: p.selected_nodes ? { ...p.selected_nodes } : {},
        selected_node_keys: p.selected_node_keys ? { ...p.selected_node_keys } : {},
        override_sni_enabled: p.override_sni != null,
        override_sni: p.override_sni != null ? p.override_sni : '',
        override_insecure: p.override_insecure == null ? 'unset' : (p.override_insecure ? 'on' : 'off'),
        override_udp: p.override_udp == null ? 'unset' : (p.override_udp ? 'on' : 'off'),
        clash_template: p.clash_template || '',
        clash_template_url: p.clash_template_url || '',
        singbox_template: p.singbox_template || '',
        singbox_template_url: p.singbox_template_url || '',
        template_status: p.template_status || ''
      })
    } else {
      setIsEditing(false)
      setEditingId('')
      setForm({
        name: '',
        subscription_ids: allSubscriptions.map((s) => s.id),
        subscription_selection_mode: 'explicit',
        deduplicate: true,
        filter_info_nodes: false,
        only_healthy: false,
        allowed_protocols: [],
        include_regex: '',
        exclude_regex: '',
        rename_prefix: '',
        rename_suffix: '',
        rename_regex_search: '',
        rename_regex_replace: '',
        sort_mode: '',
        regex_sort: '',
        region_include: '',
        region_exclude: '',
        region_groups: false,
        rule_preset: 'cn_direct',
        export_mode: 'full',
        selected_nodes: {},
        selected_node_keys: {},
        override_sni_enabled: false,
        override_sni: '',
        override_insecure: 'unset',
        override_udp: 'unset',
        clash_template: '',
        clash_template_url: '',
        singbox_template: '',
        singbox_template_url: '',
        template_status: ''
      })
    }
    setModalTab('sources')
    setShowDraftSamples(false)
    setShowDrawer(true)
  }

  const toggleSubscription = (id) => {
    const list = [...form.subscription_ids]
    const idx = list.indexOf(id)
    const nextSelected = { ...(form.selected_nodes || {}) }
    const nextKeys = { ...(form.selected_node_keys || {}) }
    if (idx > -1) {
      list.splice(idx, 1)
      delete nextSelected[id]
      delete nextKeys[id]
    } else {
      list.push(id)
    }
    setForm({ ...form, subscription_ids: list, selected_nodes: nextSelected, selected_node_keys: nextKeys })
  }

  const toggleSubExpand = async (sub) => {
    const isOpen = !!expandedSubs[sub.id]
    setExpandedSubs((prev) => ({ ...prev, [sub.id]: !isOpen }))
    if (!isOpen && !subNodes[sub.id]) {
      setLoadingNodes((prev) => ({ ...prev, [sub.id]: true }))
      try {
        const nodes = await api.getSubscriptionNodes(sub.id)
        setSubNodes((prev) => ({ ...prev, [sub.id]: Array.isArray(nodes) ? nodes : [] }))
      } catch (err) {
        showToast(err.message || 'Load nodes failed', 'error')
        setExpandedSubs((prev) => ({ ...prev, [sub.id]: false }))
      } finally {
        setLoadingNodes((prev) => ({ ...prev, [sub.id]: false }))
      }
    }
  }

  // No explicit entry means "every node of this source"; entries match by node
  // name (legacy) or stable selection key, so same-name nodes stay independent.
  const isNodeSelected = (subId, node) => {
    const entry = form.selected_nodes?.[subId]
    const keys = form.selected_node_keys?.[subId]
    if (!Array.isArray(entry) && !Array.isArray(keys)) return true
    const key = node.selection_key || ''
    if (Array.isArray(keys) && key && keys.includes(key)) return true
    if (Array.isArray(entry) && entry.includes(node.name)) return true
    return false
  }

  const toggleSubNode = (subId, node) => {
    const nodes = subNodes[subId] || []
    const checked = new Set(nodes.filter((n) => isNodeSelected(subId, n)).map((n) => n.name))
    if (checked.has(node.name)) {
      checked.delete(node.name)
    } else {
      checked.add(node.name)
    }

    const nextSelected = { ...(form.selected_nodes || {}) }
    const nextKeys = { ...(form.selected_node_keys || {}) }
    let nextSubIds = form.subscription_ids

    if (checked.size === 0) {
      // Nothing left: stop using this source entirely
      delete nextSelected[subId]
      delete nextKeys[subId]
      nextSubIds = form.subscription_ids.filter((id) => id !== subId)
      showToast(t('profiles.sourceClearedToast'))
    } else if (nodes.length > 0 && checked.size === nodes.length) {
      // Fully selected again: fall back to whole-source semantics so future nodes join too
      delete nextSelected[subId]
      delete nextKeys[subId]
    } else {
      // Names only for uniquely-named nodes; duplicate names rely on stable keys.
      const nameCounts = {}
      for (const n of nodes) {
        nameCounts[n.name] = (nameCounts[n.name] || 0) + 1
      }
      const checkedNodes = nodes.filter((n) => checked.has(n.name))
      nextSelected[subId] = [...new Set(checkedNodes.filter((n) => nameCounts[n.name] === 1).map((n) => n.name))]
      nextKeys[subId] = checkedNodes.map((n) => n.selection_key).filter(Boolean)
    }

    setForm({ ...form, subscription_ids: nextSubIds, selected_nodes: nextSelected, selected_node_keys: nextKeys })
  }

  const toggleProtocol = (proto) => {
    const list = [...form.allowed_protocols]
    const idx = list.indexOf(proto)
    if (idx > -1) list.splice(idx, 1)
    else list.push(proto)
    setForm({ ...form, allowed_protocols: list })
  }

  const buildProfilePayload = () => {
    const splitLines = (text) => (text || '').split('\n').map((l) => l.trim()).filter(Boolean)
    const splitTokens = (text) => (text || '').split(',').map((l) => l.trim()).filter(Boolean)
    const payload = {
      ...form,
      subscription_selection_mode: form.subscription_selection_mode || 'explicit',
      regex_sort: splitLines(form.regex_sort),
      region_include: splitTokens(form.region_include),
      region_exclude: splitTokens(form.region_exclude)
    }
    // Tri-state overrides: only send when the user opted in.
    if (form.override_sni_enabled) {
      payload.override_sni = form.override_sni
    } else {
      delete payload.override_sni
    }
    if (form.override_insecure !== 'unset') {
      payload.override_insecure = form.override_insecure === 'on'
    } else {
      delete payload.override_insecure
    }
    if (form.override_udp !== 'unset') {
      payload.override_udp = form.override_udp === 'on'
    } else {
      delete payload.override_udp
    }
    delete payload.override_sni_enabled
    return payload
  }

  const handleSubmit = async (e) => {
    e.preventDefault()
    if (!form.name.trim()) {
      showToast(t('profiles.needNameError'), 'error')
      return
    }
    if (hasRegexError) {
      showToast(t('profiles.fixRegexError'), 'error')
      return
    }

    setSaving(true)
    try {
      const payload = buildProfilePayload()
      if (isEditing) {
        await api.updateProfile(editingId, payload)
        showToast(t('profiles.savedUpdate'))
      } else {
        await api.createProfile(payload)
        showToast(t('profiles.savedCreate'))
      }
      setShowDrawer(false)
      await loadData()
    } catch (err) {
      showToast(err.message || 'Save failed', 'error')
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
        showToast(t('profiles.deleted'))
      } else if (type === 'reset') {
        await api.resetProfileToken(profile.id)
        showToast(t('profiles.tokenResetOk'))
      }
      setConfirmState({ open: false, type: '', profile: null, loading: false })
      await loadData()
    } catch (err) {
      showToast(err.message || 'Failed', 'error')
      setConfirmState((prev) => ({ ...prev, loading: false }))
    }
  }

  const openPreview = async (p) => {
    setPreviewState({
      open: true,
      profileId: p.id,
      name: t('profiles.previewNodesTitle', { name: p.name }),
      nodes: [],
      loading: true
    })
    try {
      const res = await api.previewProfile(p.id)
      const nodes = Array.isArray(res) ? res : (res?.nodes || [])
      setPreviewState({
        open: true,
        profileId: p.id,
        name: t('profiles.previewNodesTitle', { name: p.name }),
        nodes,
        loading: false
      })
    } catch (err) {
      showToast(err.message || 'Preview failed', 'error')
      setPreviewState({
        open: true,
        profileId: p.id,
        name: t('profiles.previewNodesTitle', { name: p.name }),
        nodes: [],
        loading: false
      })
    }
  }

  const handleProbeProfile = async () => {
    if (!previewState.profileId) return
    setProbingModal(true)
    try {
      const res = await api.probeProfile(previewState.profileId)
      const nodes = Array.isArray(res?.nodes) ? res.nodes : []
      setPreviewState((prev) => ({ ...prev, nodes }))
      showToast(t('profiles.probeOk', { healthy: res?.healthy ?? 0, failed: res?.failed ?? 0 }))
      await loadData()
    } catch (err) {
      showToast(err.message || t('profiles.probeFailed'), 'error')
    } finally {
      setProbingModal(false)
    }
  }

  const formatPullTime = (val) => {
    if (!val || val.startsWith('0001')) return ''
    const diffSec = Math.floor((Date.now() - new Date(val).getTime()) / 1000)
    if (diffSec < 60) return t('subscriptions.justNow')
    if (diffSec < 3600) return t('subscriptions.minsAgo', { n: Math.floor(diffSec / 60) })
    if (diffSec < 86400) return t('subscriptions.hoursAgo', { n: Math.floor(diffSec / 3600) })
    return new Date(val).toLocaleDateString(lang === 'en' ? 'en-US' : 'zh-CN', { month: '2-digit', day: '2-digit' })
  }

  return (
    <div className="space-y-4 sm:space-y-5 max-w-7xl mx-auto">
      <Toast toast={toast} />

      {/* Header */}
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3 pb-1">
        <div className="flex items-center gap-2.5">
          <h1 className="text-xl sm:text-2xl font-bold tracking-tight text-zinc-900 dark:text-white">
            {t('profiles.title')}
          </h1>
          <span className="hero-chip-default text-xs font-medium tabular-nums">
            {t('profiles.countBadge', { count: profiles.length })}
          </span>
        </div>

        <button
          onClick={() => openDrawer()}
          className="hero-btn-primary px-4 py-2 text-xs font-semibold gap-1.5 shrink-0 w-full sm:w-auto"
        >
          <Plus className="w-4 h-4" />
          <span>{t('profiles.createBtn')}</span>
        </button>
      </div>

      {/* Loading / Empty / List */}
      {loading ? (
        <div className="hero-card">
          <RowListSkeleton count={3} />
        </div>
      ) : profiles.length === 0 ? (
        <div className="p-12 sm:p-16 hero-card text-center space-y-3">
          <div className="w-11 h-11 rounded-2xl bg-zinc-100 dark:bg-white/5 flex items-center justify-center text-zinc-400 mx-auto">
            <Layers className="w-5 h-5" />
          </div>
          <h3 className="text-sm font-bold text-zinc-900 dark:text-white">{t('profiles.emptyTitle')}</h3>
          <button onClick={() => openDrawer()} className="hero-btn-primary px-4 py-2 text-xs font-semibold">
            {t('profiles.createNow')}
          </button>
        </div>
      ) : (
        <div className="space-y-3.5 sm:space-y-4">
          {profiles.map((p, idx) => {
            const subNames = getSubNames(p.subscription_ids)
            const openUp = idx >= profiles.length - 1 && profiles.length > 2
            return (
              <motion.div
                key={p.id}
                layout
                initial={{ opacity: 0, y: 6 }}
                animate={{ opacity: 1, y: 0 }}
                className="hero-card hero-card-interactive p-4 sm:p-6 space-y-3.5 sm:space-y-4"
              >
                <div className="flex flex-col sm:flex-row sm:items-start justify-between gap-3">
                  <div className="space-y-2 min-w-0">
                    <div className="flex items-center gap-2.5 flex-wrap">
                      <h3 className="font-bold text-base text-zinc-900 dark:text-white tracking-tight">{p.name}</h3>
                      <span className="hero-chip-primary text-xs tabular-nums">
                        <span className="font-mono font-bold mr-1">{p.node_count || 0}</span>
                        <span>{t('common.nodes')}</span>
                      </span>
                      <span className="inline-flex items-center gap-1.5 text-xs text-emerald-600 dark:text-emerald-400 font-medium">
                        <span className="w-1.5 h-1.5 rounded-full bg-emerald-500" />
                        <span>{t('profiles.onlineBadge')}</span>
                      </span>
                    </div>

                    <div className="flex items-center gap-1.5 flex-wrap text-xs">
                      <span className="text-zinc-400">{t('profiles.sourcesLabel')}</span>
                      {subNames.length > 0 ? (
                        subNames.map((sname, sIdx) => (
                          <span key={sIdx} className="hero-chip-default text-[11px]">
                            {sname}
                          </span>
                        ))
                      ) : (
                        <span className="text-zinc-400 italic text-[11px]">{t('profiles.allSources')}</span>
                      )}
                    </div>

                    <div className="flex items-center gap-1.5 flex-wrap text-[11px] pt-0.5">
                      {p.deduplicate && <span className="hero-chip-default">{t('profiles.tagDedup')}</span>}
                      {p.filter_info_nodes && <span className="hero-chip-default">{t('profiles.tagFilterInfo')}</span>}
                      {p.only_healthy && <span className="hero-chip-default">{t('profiles.tagOnlyHealthy')}</span>}
                      {p.allowed_protocols && p.allowed_protocols.length > 0 && (
                        <span className="hero-chip-default">
                          {t('profiles.tagProto', { list: p.allowed_protocols.join(', ') })}
                        </span>
                      )}
                      {p.include_regex && (
                        <span className="hero-chip-default font-mono">
                          {t('profiles.tagInclude', { re: p.include_regex })}
                        </span>
                      )}
                      {p.exclude_regex && (
                        <span className="hero-chip-default font-mono">
                          {t('profiles.tagExclude', { re: p.exclude_regex })}
                        </span>
                      )}
                      {(p.rename_prefix || p.rename_suffix || p.rename_regex_search) && (
                        <span className="hero-chip-default">{t('profiles.tagRenamed')}</span>
                      )}
                      {p.sort_mode === 'region' && (
                        <span className="hero-chip-default">{t('profiles.tagSortRegion')}</span>
                      )}
                      {p.sort_mode === 'name' && (
                        <span className="hero-chip-default">{t('profiles.tagSortName')}</span>
                      )}
                      {p.region_groups && (
                        <span className="hero-chip-primary text-[11px]">{t('profiles.tagRegionGroups')}</span>
                      )}
                      {p.rule_preset === 'ai_streaming' && (
                        <span className="hero-chip-primary text-[11px]">{t('profiles.tagAiStreaming')}</span>
                      )}
                      {p.rule_preset === 'global' && (
                        <span className="hero-chip-default">{t('profiles.tagGlobal')}</span>
                      )}
                      {p.export_mode === 'provider' && (
                        <span className="hero-chip-default">{t('profiles.tagProviderDefault')}</span>
                      )}
                      {p.selected_nodes && Object.keys(p.selected_nodes).length > 0 && (
                        <span className="hero-chip-default">{t('profiles.tagSelectedNodes')}</span>
                      )}
                    </div>
                  </div>

                  <div className="flex items-center justify-between sm:justify-end gap-1.5 shrink-0">
                    <div className="flex items-center gap-1.5 text-xs font-mono text-zinc-400 px-2.5 py-1.5 rounded-xl bg-zinc-50 dark:bg-white/[0.04] border border-zinc-200/80 dark:border-white/10">
                      <span>/s/{p.token ? `${p.token.slice(0, 6)}...${p.token.slice(-4)}` : '------'}</span>
                      <button
                        onClick={() => copyText(getUniversalUrl(p.token), t('dashboard.universalCopiedToast'))}
                        title={t('profiles.copyTokenTip')}
                        className="hover:text-zinc-700 dark:hover:text-white transition-colors"
                      >
                        <Copy className="w-3.5 h-3.5" />
                      </button>
                    </div>
                    <div className="flex items-center gap-0.5 sm:gap-1">
                      <button
                        onClick={() => setConfirmState({ open: true, type: 'reset', profile: p, loading: false })}
                        title={t('profiles.resetTitle')}
                        className="hero-btn-default p-2 text-zinc-400 hover:text-amber-500"
                      >
                        <RotateCcw className="w-4 h-4" />
                      </button>
                      <button
                        onClick={() => openPreview(p)}
                        title={t('profiles.nodesBtn')}
                        className="hero-btn-default p-2 text-zinc-400 hover:text-zinc-900 dark:hover:text-white"
                      >
                        <Eye className="w-4 h-4" />
                      </button>
                      <button
                        onClick={() => openDrawer(p)}
                        title={t('common.edit')}
                        className="hero-btn-default p-2 text-zinc-400 hover:text-zinc-900 dark:hover:text-white"
                      >
                        <Pencil className="w-4 h-4" />
                      </button>
                      <button
                        onClick={() => setConfirmState({ open: true, type: 'delete', profile: p, loading: false })}
                        title={t('common.delete')}
                        className="hero-btn-default p-2 text-zinc-400 hover:text-rose-600 hover:bg-rose-500/10"
                      >
                        <Trash2 className="w-4 h-4" />
                      </button>
                    </div>
                  </div>
                </div>

                {/* Action Ribbon */}
                <div className="pt-3 border-t border-zinc-100 dark:border-white/[0.06] flex flex-col lg:flex-row lg:items-center justify-between gap-3">
                  <div className="flex items-center gap-2 flex-wrap w-full lg:w-auto">
                    <button
                      onClick={() => copyText(getUniversalUrl(p.token), t('dashboard.universalCopiedToast'), p.id)}
                      className={`hero-btn-primary w-full sm:w-auto px-4 py-2 text-xs font-semibold gap-1.5 transition-all ${
                        copiedProfileId === p.id ? '!bg-emerald-600' : ''
                      }`}
                    >
                      {copiedProfileId === p.id ? (
                        <>
                          <CheckCircle2 className="w-4 h-4 text-white" />
                          <span>{t('profiles.copied')}</span>
                        </>
                      ) : (
                        <>
                          <Zap className="w-4 h-4 text-amber-300" />
                          <span>{t('profiles.copyUniversal')}</span>
                        </>
                      )}
                    </button>

                    <div className="relative format-dropdown-container">
                      <button
                        onClick={(e) => {
                          e.stopPropagation()
                          setActiveDropdown(activeDropdown === p.id ? null : p.id)
                        }}
                        className="hero-btn-bordered px-3 py-2 text-xs font-semibold gap-1"
                      >
                        <span>{t('profiles.exportBtn')}</span>
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
                              copyText(getExportUrl(p.token, 'clash', true), t('dashboard.clashFullCopied'))
                              setActiveDropdown(null)
                            }}
                            className="w-full text-left px-3.5 py-2 hover:bg-zinc-100 dark:hover:bg-white/5 text-zinc-700 dark:text-zinc-200 flex items-center justify-between"
                          >
                            <span>{t('dashboard.clashFull')}</span>
                            <span className="text-[10px] font-mono text-zinc-400">YAML</span>
                          </button>
                          <button
                            onClick={() => {
                              copyText(getExportUrl(p.token, 'singbox', true), t('dashboard.singboxFullCopied'))
                              setActiveDropdown(null)
                            }}
                            className="w-full text-left px-3.5 py-2 hover:bg-zinc-100 dark:hover:bg-white/5 text-zinc-700 dark:text-zinc-200 flex items-center justify-between"
                          >
                            <span>{t('dashboard.singboxFull')}</span>
                            <span className="text-[10px] font-mono text-zinc-400">JSON</span>
                          </button>
                          <button
                            onClick={() => {
                              copyText(getExportUrl(p.token, 'clash', false), t('dashboard.clashProviderCopied'))
                              setActiveDropdown(null)
                            }}
                            className="w-full text-left px-3.5 py-2 hover:bg-zinc-100 dark:hover:bg-white/5 text-zinc-700 dark:text-zinc-200 flex items-center justify-between"
                          >
                            <span>{t('dashboard.clashProvider')}</span>
                            <span className="text-[10px] font-mono text-zinc-400">Provider</span>
                          </button>
                          <button
                            onClick={() => {
                              copyText(getExportUrl(p.token, 'base64', false), t('dashboard.base64Copied'))
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
                      className="hero-btn-bordered px-3 py-2 text-xs font-medium gap-1.5"
                    >
                      <FileCode2 className="w-3.5 h-3.5 text-[#2563EB] dark:text-[#60A5FA]" />
                      <span className="hidden sm:inline">{t('profiles.configBtn')}</span>
                    </button>

                    <button
                      onClick={() => setQrState({ open: true, token: p.token, name: p.name })}
                      className="hero-btn-bordered px-3 py-2 text-xs font-medium gap-1.5"
                    >
                      <QrCode className="w-3.5 h-3.5 text-zinc-500" />
                      <span className="hidden sm:inline">{t('profiles.qrBtn')}</span>
                    </button>
                  </div>

                  <div className="text-xs text-zinc-400 flex items-center gap-3 flex-wrap">
                    {p.pull_count > 0 && p.last_pull_at ? (
                      <span className="font-mono text-[11px] tabular-nums">
                        {t('profiles.pullStats', {
                          time: formatPullTime(p.last_pull_at),
                          client: p.last_pull_client || '-',
                          count: p.pull_count
                        })}
                      </span>
                    ) : (
                      <span>{t('profiles.updateCycle')}</span>
                    )}
                  </div>
                </div>
              </motion.div>
            )
          })}
        </div>
      )}

      {/* Slide-over Drawer for Create / Edit Profile */}
      <Drawer
        open={showDrawer}
        onClose={() => setShowDrawer(false)}
        size="lg"
        title={isEditing ? t('profiles.drawerEditTitle') : t('profiles.drawerCreateTitle')}
        headerExtra={
          <span className="hero-chip-primary font-mono text-[11px] tabular-nums">
            {draftPreview.loading ? t('profiles.calculating') : t('profiles.hitCount', { count: draftPreview.count })}
          </span>
        }
        footer={
          <div className="flex items-center justify-end gap-2.5">
            <button
              type="button"
              onClick={() => setShowDrawer(false)}
              className="hero-btn-bordered px-4 py-2 text-xs font-medium"
            >
              {t('common.cancel')}
            </button>
            <button
              type="submit"
              form="profile-form"
              disabled={saving || hasRegexError}
              className="hero-btn-primary px-5 py-2 text-xs font-semibold disabled:opacity-50"
            >
              {saving ? t('common.saving') : t('common.save')}
            </button>
          </div>
        }
      >
        <div className="flex flex-col h-full">
          {/* Segmented Tabs */}
          <div className="flex p-2 border-b border-zinc-100 dark:border-white/[0.06] bg-zinc-50/70 dark:bg-white/[0.02] gap-1 text-xs shrink-0">
            {[
              { key: 'sources', label: t('profiles.tabSources', { count: form.subscription_ids.length }) },
              { key: 'filters', label: t('profiles.tabFilters') },
              { key: 'rename', label: t('profiles.tabRules') },
              { key: 'template', label: t('profiles.tabTemplate') }
            ].map((tab) => (
              <button
                key={tab.key}
                type="button"
                onClick={() => setModalTab(tab.key)}
                className={`relative flex-1 py-2 px-2 sm:px-3 rounded-xl font-bold transition-colors text-center whitespace-nowrap ${
                  modalTab === tab.key
                    ? 'text-[#2563EB] dark:text-white'
                    : 'text-zinc-500 hover:text-zinc-900 dark:hover:text-white'
                }`}
              >
                {modalTab === tab.key && (
                  <motion.span
                    layoutId="profile-drawer-tab"
                    transition={{ type: 'spring', stiffness: 400, damping: 32 }}
                    className="absolute inset-0 rounded-xl bg-white dark:bg-[#1E2330] shadow-xs border border-zinc-200/60 dark:border-white/10"
                  />
                )}
                <span className="relative z-10">{tab.label}</span>
              </button>
            ))}
          </div>

          <form id="profile-form" onSubmit={handleSubmit} className="p-4 sm:p-6 space-y-5 flex-1">
            {/* Live Pipeline Preview Banner */}
            <div className="rounded-xl border border-zinc-200/70 dark:border-white/[0.08] bg-zinc-50/80 dark:bg-white/[0.02] p-3.5 space-y-2.5">
              <div className="flex items-center justify-between gap-2 text-xs">
                <div className="flex items-center gap-2">
                  <Sparkles className="w-3.5 h-3.5 text-[#2563EB] dark:text-[#60A5FA]" />
                  <span className="font-semibold text-zinc-700 dark:text-zinc-200">
                    {t('profiles.liveHitsLabel')}{' '}
                    <span className="font-mono font-bold text-[#2563EB] dark:text-[#60A5FA] tabular-nums">
                      {draftPreview.count}
                    </span>{' '}
                    {t('common.nodes')}
                  </span>
                </div>
                {draftPreview.nodes.length > 0 && (
                  <button
                    type="button"
                    onClick={() => setShowDraftSamples((v) => !v)}
                    className="text-[11px] font-semibold text-[#2563EB] dark:text-[#60A5FA] hover:underline shrink-0"
                  >
                    {showDraftSamples
                      ? t('profiles.collapseSample')
                      : t('profiles.expandSample', { n: Math.min(10, draftPreview.nodes.length) })}
                  </button>
                )}
              </div>

              {showDraftSamples && draftPreview.nodes.length > 0 && (
                <div className="pt-2 border-t border-zinc-200/60 dark:border-white/[0.06] space-y-1.5 max-h-40 overflow-y-auto">
                  {draftPreview.nodes.slice(0, 10).map((n, sIdx) => (
                    <div key={sIdx} className="flex items-center justify-between gap-2 text-[11px]">
                      <span className="text-zinc-700 dark:text-zinc-300 truncate">{n.name}</span>
                      <span
                        className={`px-1.5 py-0.2 rounded-full text-[9px] font-mono font-bold uppercase border shrink-0 ${getProtocolBadge(
                          n.type
                        )}`}
                      >
                        {n.type}
                      </span>
                    </div>
                  ))}
                </div>
              )}
            </div>

            {modalTab === 'sources' && (
              <div className="space-y-4">
                <div>
                  <label className="block text-[11px] font-semibold text-zinc-500 uppercase tracking-wider mb-1.5">
                    {t('profiles.profileNameLabel')}
                  </label>
                  <input
                    type="text"
                    required
                    value={form.name}
                    onChange={(e) => setForm({ ...form, name: e.target.value })}
                    placeholder={t('profiles.profileNamePh')}
                    className="hero-input px-3.5 py-2.5 text-sm sm:text-xs text-zinc-900 dark:text-white"
                  />
                </div>

                <div className="flex items-center justify-between pt-2">
                  <label className="block text-[11px] font-semibold text-zinc-500 uppercase tracking-wider">
                    {t('profiles.selectSourcesLabel')}
                  </label>
                  <div className="flex gap-2.5 text-xs shrink-0">
                    <button
                      type="button"
                      onClick={() =>
                        setForm({ ...form, subscription_ids: allSubscriptions.map((s) => s.id), selected_nodes: {}, selected_node_keys: {} })
                      }
                      className="text-[#2563EB] dark:text-[#60A5FA] font-semibold hover:underline"
                    >
                      {t('common.selectAll')}
                    </button>
                    <span className="text-zinc-300 dark:text-zinc-600">|</span>
                    <button
                      type="button"
                      onClick={() => setForm({ ...form, subscription_ids: [], selected_nodes: {}, selected_node_keys: {} })}
                      className="text-zinc-400 hover:text-zinc-700 dark:hover:text-zinc-200"
                    >
                      {t('common.clear')}
                    </button>
                  </div>
                </div>

                <div className="flex p-1 bg-zinc-100 dark:bg-white/[0.04] rounded-xl gap-1 text-xs">
                  {[
                    { value: 'explicit', label: t('profiles.modeExplicit') },
                    { value: 'all', label: t('profiles.modeAll') }
                  ].map((opt) => (
                    <button
                      key={opt.value}
                      type="button"
                      onClick={() => setForm({ ...form, subscription_selection_mode: opt.value })}
                      className={`flex-1 py-1.5 rounded-lg font-semibold transition-all ${
                        form.subscription_selection_mode === opt.value
                          ? 'bg-white dark:bg-[#1E2330] text-[#2563EB] dark:text-[#60A5FA] shadow-xs'
                          : 'text-zinc-500'
                      }`}
                    >
                      {opt.label}
                    </button>
                  ))}
                </div>
                <p className="text-[10px] text-zinc-400 -mt-1.5">
                  {form.subscription_selection_mode === 'explicit' ? t('profiles.modeExplicitHint') : t('profiles.modeAllHint')}
                </p>

                {allSubscriptions.length === 0 ? (
                  <div className="p-8 text-center text-xs text-zinc-400 border border-dashed border-zinc-200 dark:border-white/10 rounded-2xl">
                    {t('profiles.noSourcesHint')}
                  </div>
                ) : (
                  <div className="space-y-2">
                    {allSubscriptions.map((sub) => {
                      const isSelected = form.subscription_ids.includes(sub.id)
                      const isExpanded = !!expandedSubs[sub.id]
                      const nodes = subNodes[sub.id] || []
                      const entry = form.selected_nodes?.[sub.id]
                      const hasExplicitSelection = Array.isArray(entry)
                      const filteredNodes = (() => {
                        const q = (nodeFilter[sub.id] || '').trim().toLowerCase()
                        if (!q) return nodes
                        return nodes.filter(
                          (n) =>
                            (n.name || '').toLowerCase().includes(q) ||
                            (n.server || '').toLowerCase().includes(q) ||
                            (n.type || '').toLowerCase().includes(q)
                        )
                      })()
                      return (
                        <div
                          key={sub.id}
                          className={`rounded-xl border transition-all ${
                            isSelected
                              ? 'border-[#2563EB]/50 bg-[#2563EB]/[0.05] dark:bg-[#2563EB]/12'
                              : 'border-zinc-200/80 dark:border-white/10 hover:bg-zinc-50 dark:hover:bg-white/[0.03]'
                          }`}
                        >
                          <label className="flex items-center justify-between p-3 cursor-pointer gap-2">
                            <div className="flex items-center gap-3 min-w-0">
                              <input
                                type="checkbox"
                                checked={isSelected}
                                onChange={() => toggleSubscription(sub.id)}
                                className="rounded text-[#2563EB] focus:ring-[#2563EB] w-4 h-4 cursor-pointer shrink-0"
                              />
                              <span
                                className={`px-2 py-0.5 rounded-md text-[10px] font-bold uppercase border shrink-0 ${
                                  sub.type === 'local'
                                    ? 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border-emerald-500/20'
                                    : 'bg-[#2563EB]/10 text-[#2563EB] dark:text-[#60A5FA] border-[#2563EB]/20'
                                }`}
                              >
                                {sub.type === 'local' ? t('dashboard.local') : t('dashboard.remote')}
                              </span>
                              <span className="text-xs font-bold text-zinc-800 dark:text-zinc-200 truncate">
                                {sub.name}
                              </span>
                            </div>
                            <div className="flex items-center gap-1.5 shrink-0">
                              {isSelected && hasExplicitSelection && (
                                <span className="hero-chip-primary text-[10px] font-mono font-bold tabular-nums">
                                  {t('profiles.selectedCountBadge', {
                                    count: entry.length,
                                    total: nodes.length || sub.node_count || entry.length
                                  })}
                                </span>
                              )}
                              <span className="text-xs text-zinc-400 shrink-0 tabular-nums font-mono">
                                {sub.node_count || 0}
                              </span>
                              <button
                                type="button"
                                onClick={(e) => {
                                  e.stopPropagation()
                                  toggleSubExpand(sub)
                                }}
                                title={isExpanded ? t('profiles.collapseNodes') : t('profiles.expandNodes')}
                                className="p-1.5 rounded-lg text-zinc-400 hover:text-zinc-700 dark:hover:text-white hover:bg-zinc-100 dark:hover:bg-white/5 transition-all active:scale-95"
                              >
                                <ChevronDown className={`w-3.5 h-3.5 transition-transform ${isExpanded ? 'rotate-180' : ''}`} />
                              </button>
                            </div>
                          </label>

                          {isExpanded && (
                            <div className="px-3 pb-3">
                              <div className="rounded-xl border border-zinc-200/70 dark:border-white/[0.08] bg-white dark:bg-white/[0.02] p-2.5 space-y-2">
                                {loadingNodes[sub.id] ? (
                                  <div className="py-4 flex items-center justify-center gap-2 text-[11px] text-zinc-400">
                                    <span className="animate-spin w-3 h-3 border-2 border-[#2563EB] border-t-transparent rounded-full" />
                                    <span>{t('common.loading')}</span>
                                  </div>
                                ) : nodes.length === 0 ? (
                                  <div className="py-3 text-center text-[11px] text-zinc-400">
                                    {t('profiles.noCachedNodes')}
                                  </div>
                                ) : (
                                  <>
                                    <div className="flex items-center gap-2">
                                      <div className="relative flex-1">
                                        <Search className="w-3 h-3 text-zinc-400 absolute left-2.5 top-1/2 -translate-y-1/2" />
                                        <input
                                          value={nodeFilter[sub.id] || ''}
                                          onChange={(e) =>
                                            setNodeFilter((prev) => ({ ...prev, [sub.id]: e.target.value }))
                                          }
                                          type="text"
                                          placeholder={t('profiles.nodesSearchPh')}
                                          className="hero-input pl-7 pr-2 py-1.5 text-[11px] text-zinc-800 dark:text-zinc-200"
                                        />
                                      </div>
                                      <span className="text-[11px] font-mono text-zinc-400 tabular-nums shrink-0">
                                        {hasExplicitSelection
                                          ? t('profiles.selectedCountBadge', {
                                              count: entry.length,
                                              total: nodes.length
                                            })
                                          : t('profiles.nodesAllShort')}
                                      </span>
                                    </div>

                                    <div className="max-h-52 overflow-y-auto overscroll-contain rounded-lg border border-zinc-100 dark:border-white/[0.05] divide-y divide-zinc-100 dark:divide-white/[0.05]">
                                      {filteredNodes.length === 0 ? (
                                        <div className="py-3 text-center text-[11px] text-zinc-400">
                                          {t('nodesModal.emptySearch')}
                                        </div>
                                      ) : (
                                        filteredNodes.map((node, idx) => (
                                          <label
                                            key={`${node.name}-${idx}`}
                                            className="flex items-center gap-2.5 px-2.5 py-1.5 cursor-pointer hover:bg-zinc-50 dark:hover:bg-white/[0.03]"
                                          >
                                            <input
                                              type="checkbox"
                                              checked={isNodeSelected(sub.id, node)}
                                              onChange={() => toggleSubNode(sub.id, node)}
                                              className="rounded text-[#2563EB] focus:ring-[#2563EB] w-3.5 h-3.5 cursor-pointer shrink-0"
                                            />
                                            <span
                                              className="text-[11px] font-medium text-zinc-800 dark:text-zinc-200 truncate flex-1"
                                              title={node.name}
                                            >
                                              {node.name}
                                            </span>
                                            <span
                                              className={`px-1.5 py-0.2 rounded-full text-[9px] font-mono font-bold uppercase border shrink-0 ${getProtocolBadge(node.type)}`}
                                            >
                                              {node.type}
                                            </span>
                                          </label>
                                        ))
                                      )}
                                    </div>

                                    <p className="text-[10px] text-zinc-400 leading-relaxed">
                                      {t('profiles.nodeSelectionHint')}
                                    </p>
                                  </>
                                )}
                              </div>
                            </div>
                          )}
                        </div>
                      )
                    })}
                  </div>
                )}
              </div>
            )}

            {modalTab === 'filters' && (
              <div className="space-y-4">
                <div className="space-y-2.5">
                  <label className="flex items-center justify-between p-3.5 rounded-xl border border-zinc-200/80 dark:border-white/10 bg-zinc-50 dark:bg-white/[0.03] cursor-pointer gap-3">
                    <div className="min-w-0">
                      <span className="text-xs font-bold text-zinc-900 dark:text-white">{t('profiles.dedupTitle')}</span>
                      <p className="text-[11px] text-zinc-400 mt-0.5">{t('profiles.dedupDesc')}</p>
                    </div>
                    <input
                      type="checkbox"
                      checked={form.deduplicate}
                      onChange={(e) => setForm({ ...form, deduplicate: e.target.checked })}
                      className="rounded text-[#2563EB] focus:ring-[#2563EB] w-4 h-4 cursor-pointer shrink-0"
                    />
                  </label>

                  <label className="flex items-center justify-between p-3.5 rounded-xl border border-zinc-200/80 dark:border-white/10 bg-zinc-50 dark:bg-white/[0.03] cursor-pointer gap-3">
                    <div className="min-w-0">
                      <span className="text-xs font-bold text-zinc-900 dark:text-white">{t('profiles.filterInfoTitle')}</span>
                      <p className="text-[11px] text-zinc-400 mt-0.5">{t('profiles.filterInfoDesc')}</p>
                    </div>
                    <input
                      type="checkbox"
                      checked={!!form.filter_info_nodes}
                      onChange={(e) => setForm({ ...form, filter_info_nodes: e.target.checked })}
                      className="rounded text-[#2563EB] focus:ring-[#2563EB] w-4 h-4 cursor-pointer shrink-0"
                    />
                  </label>

                  <label className="flex items-center justify-between p-3.5 rounded-xl border border-zinc-200/80 dark:border-white/10 bg-zinc-50 dark:bg-white/[0.03] cursor-pointer gap-3">
                    <div className="min-w-0">
                      <span className="text-xs font-bold text-zinc-900 dark:text-white">{t('profiles.onlyHealthyTitle')}</span>
                      <p className="text-[11px] text-zinc-400 mt-0.5">{t('profiles.onlyHealthyDesc')}</p>
                    </div>
                    <input
                      type="checkbox"
                      checked={!!form.only_healthy}
                      onChange={(e) => setForm({ ...form, only_healthy: e.target.checked })}
                      className="rounded text-[#2563EB] focus:ring-[#2563EB] w-4 h-4 cursor-pointer shrink-0"
                    />
                  </label>
                </div>

                <div>
                  <label className="block text-[11px] font-semibold text-zinc-500 uppercase tracking-wider mb-2">
                    {t('profiles.protoWhitelistLabel')}
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
                        {t('profiles.includeRegexLabel')}
                      </label>
                      <button
                        type="button"
                        onClick={() => setForm({ ...form, include_regex: '香港|日本|新加坡|美国|HK|JP|SG|US' })}
                        className="text-[11px] text-[#2563EB] dark:text-[#60A5FA] font-semibold hover:underline"
                      >
                        {t('profiles.presetCjkRegions')}
                      </button>
                    </div>
                    <input
                      type="text"
                      value={form.include_regex}
                      onChange={(e) => setForm({ ...form, include_regex: e.target.value })}
                      placeholder={t('profiles.includeRegexPh')}
                      className={`hero-input px-3.5 py-2 text-sm sm:text-xs font-mono text-zinc-900 dark:text-white ${
                        regexErrors.include ? '!border-rose-500' : ''
                      }`}
                    />
                    {regexErrors.include && (
                      <p className="text-[11px] text-rose-500 mt-1 flex items-center gap-1">
                        <AlertCircle className="w-3 h-3 shrink-0" />
                        <span>{t('profiles.regexError', { err: regexErrors.include })}</span>
                      </p>
                    )}
                  </div>

                  <div>
                    <div className="flex items-center justify-between mb-1.5">
                      <label className="block text-[11px] font-semibold text-zinc-500 uppercase tracking-wider">
                        {t('profiles.excludeRegexLabel')}
                      </label>
                      <button
                        type="button"
                        onClick={() => setForm({ ...form, exclude_regex: '官网|到期|剩余|流量|通知|群|Expire|Traffic' })}
                        className="text-[11px] text-[#2563EB] dark:text-[#60A5FA] font-semibold hover:underline"
                      >
                        {t('profiles.presetInfoWords')}
                      </button>
                    </div>
                    <input
                      type="text"
                      value={form.exclude_regex}
                      onChange={(e) => setForm({ ...form, exclude_regex: e.target.value })}
                      placeholder={t('profiles.excludeRegexPh')}
                      className={`hero-input px-3.5 py-2 text-sm sm:text-xs font-mono text-zinc-900 dark:text-white ${
                        regexErrors.exclude ? '!border-rose-500' : ''
                      }`}
                    />
                    {regexErrors.exclude && (
                      <p className="text-[11px] text-rose-500 mt-1 flex items-center gap-1">
                        <AlertCircle className="w-3 h-3 shrink-0" />
                        <span>{t('profiles.regexError', { err: regexErrors.exclude })}</span>
                      </p>
                    )}
                  </div>
                </div>

                <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                  <div>
                    <div className="flex items-center justify-between mb-1.5">
                      <label className="block text-[11px] font-semibold text-zinc-500 uppercase tracking-wider">
                        {t('profiles.regionIncludeLabel')}
                      </label>
                      <button
                        type="button"
                        onClick={() => setForm({ ...form, region_include: 'HK, US, JP, SG, TW, KR' })}
                        className="text-[11px] text-[#2563EB] dark:text-[#60A5FA] font-semibold hover:underline"
                      >
                        {t('profiles.regionPresets')}
                      </button>
                    </div>
                    <input
                      type="text"
                      value={form.region_include}
                      onChange={(e) => setForm({ ...form, region_include: e.target.value })}
                      placeholder={t('profiles.regionIncludePh')}
                      className="hero-input px-3.5 py-2 text-sm sm:text-xs font-mono text-zinc-900 dark:text-white"
                    />
                    <p className="text-[10px] text-zinc-400 mt-1">{t('profiles.regionHint')}</p>
                  </div>
                  <div>
                    <label className="block text-[11px] font-semibold text-zinc-500 uppercase tracking-wider mb-1.5">
                      {t('profiles.regionExcludeLabel')}
                    </label>
                    <input
                      type="text"
                      value={form.region_exclude}
                      onChange={(e) => setForm({ ...form, region_exclude: e.target.value })}
                      placeholder={t('profiles.regionExcludePh')}
                      className="hero-input px-3.5 py-2 text-sm sm:text-xs font-mono text-zinc-900 dark:text-white"
                    />
                  </div>
                </div>

                <div>
                  <label className="block text-[11px] font-semibold text-zinc-500 uppercase tracking-wider mb-1.5">
                    {t('profiles.regexSortLabel')}
                  </label>
                  <textarea
                    value={form.regex_sort}
                    onChange={(e) => setForm({ ...form, regex_sort: e.target.value })}
                    rows={3}
                    placeholder={'US\nHK\n(IEPL|专线)'}
                    className={`hero-input p-3 text-sm sm:text-xs font-mono text-zinc-900 dark:text-white leading-relaxed ${
                      regexErrors.sort ? '!border-rose-500' : ''
                    }`}
                  />
                  {regexErrors.sort && (
                    <p className="text-[11px] text-rose-500 mt-1 flex items-center gap-1">
                      <AlertCircle className="w-3 h-3 shrink-0" />
                      <span>{t('profiles.regexError', { err: regexErrors.sort })}</span>
                    </p>
                  )}
                  <p className="text-[10px] text-zinc-400 mt-1">{t('profiles.regexSortHint')}</p>
                </div>

                <div className="rounded-xl border border-zinc-200/70 dark:border-white/[0.08] bg-zinc-50/80 dark:bg-white/[0.02] p-3.5 space-y-3">
                  <div className="text-[11px] font-bold text-zinc-700 dark:text-zinc-200 uppercase tracking-wider">
                    {t('profiles.overridesTitle')}
                  </div>

                  <div className="flex items-center gap-3">
                    <label className="flex items-center gap-2 cursor-pointer shrink-0">
                      <input
                        type="checkbox"
                        checked={form.override_sni_enabled}
                        onChange={(e) => setForm({ ...form, override_sni_enabled: e.target.checked })}
                        className="rounded text-[#2563EB] focus:ring-[#2563EB] w-4 h-4 cursor-pointer shrink-0"
                      />
                      <span className="text-xs text-zinc-700 dark:text-zinc-300 font-medium whitespace-nowrap">
                        {t('profiles.overrideSniLabel')}
                      </span>
                    </label>
                    <input
                      type="text"
                      disabled={!form.override_sni_enabled}
                      value={form.override_sni}
                      onChange={(e) => setForm({ ...form, override_sni: e.target.value })}
                      placeholder={t('profiles.overrideSniPh')}
                      className="hero-input px-3 py-1.5 text-xs font-mono flex-1 disabled:opacity-40"
                    />
                  </div>
                  <p className="text-[10px] text-zinc-400 -mt-1.5">{t('profiles.overrideSniHint')}</p>

                  {[
                    { key: 'override_insecure', label: t('profiles.overrideInsecureLabel'), hint: t('profiles.overrideInsecureHint') },
                    { key: 'override_udp', label: t('profiles.overrideUdpLabel'), hint: t('profiles.overrideUdpHint') }
                  ].map((item) => (
                    <div key={item.key} className="flex items-center justify-between gap-3">
                      <div className="min-w-0">
                        <span className="text-xs text-zinc-700 dark:text-zinc-300 font-medium">{item.label}</span>
                        <p className="text-[10px] text-zinc-400 mt-0.5">{item.hint}</p>
                      </div>
                      <div className="flex p-0.5 bg-zinc-100 dark:bg-white/[0.05] rounded-lg gap-0.5 shrink-0">
                        {[
                          { value: 'unset', label: t('profiles.triDefault') },
                          { value: 'on', label: t('common.on') },
                          { value: 'off', label: t('common.off') }
                        ].map((opt) => (
                          <button
                            key={opt.value}
                            type="button"
                            onClick={() => setForm({ ...form, [item.key]: opt.value })}
                            className={`px-2.5 py-1 rounded-md text-[11px] font-semibold transition-all ${
                              form[item.key] === opt.value
                                ? 'bg-white dark:bg-[#1E2330] text-[#2563EB] dark:text-[#60A5FA] shadow-xs'
                                : 'text-zinc-500'
                            }`}
                          >
                            {opt.label}
                          </button>
                        ))}
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            )}

            {modalTab === 'rename' && (
              <div className="space-y-4">
                <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                  <div>
                    <label className="block text-[11px] font-semibold text-zinc-500 uppercase tracking-wider mb-1.5">
                      {t('profiles.prefixLabel')}
                    </label>
                    <input
                      type="text"
                      value={form.rename_prefix}
                      onChange={(e) => setForm({ ...form, rename_prefix: e.target.value })}
                      placeholder={t('profiles.prefixPh')}
                      className="hero-input px-3.5 py-2 text-sm sm:text-xs text-zinc-900 dark:text-white"
                    />
                  </div>
                  <div>
                    <label className="block text-[11px] font-semibold text-zinc-500 uppercase tracking-wider mb-1.5">
                      {t('profiles.suffixLabel')}
                    </label>
                    <input
                      type="text"
                      value={form.rename_suffix}
                      onChange={(e) => setForm({ ...form, rename_suffix: e.target.value })}
                      placeholder={t('profiles.suffixPh')}
                      className="hero-input px-3.5 py-2 text-sm sm:text-xs text-zinc-900 dark:text-white"
                    />
                  </div>
                </div>

                <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                  <div>
                    <label className="block text-[11px] font-semibold text-zinc-500 uppercase tracking-wider mb-1.5">
                      {t('profiles.regexSearchLabel')}
                    </label>
                    <input
                      type="text"
                      value={form.rename_regex_search}
                      onChange={(e) => setForm({ ...form, rename_regex_search: e.target.value })}
                      placeholder="IEPL-(.*)"
                      className={`hero-input px-3.5 py-2 text-sm sm:text-xs font-mono text-zinc-900 dark:text-white ${
                        regexErrors.rename ? '!border-rose-500' : ''
                      }`}
                    />
                    {regexErrors.rename && (
                      <p className="text-[11px] text-rose-500 mt-1">{regexErrors.rename}</p>
                    )}
                  </div>
                  <div>
                    <label className="block text-[11px] font-semibold text-zinc-500 uppercase tracking-wider mb-1.5">
                      {t('profiles.regexReplaceLabel')}
                    </label>
                    <input
                      type="text"
                      value={form.rename_regex_replace}
                      onChange={(e) => setForm({ ...form, rename_regex_replace: e.target.value })}
                      placeholder="$1"
                      className="hero-input px-3.5 py-2 text-sm sm:text-xs font-mono text-zinc-900 dark:text-white"
                    />
                  </div>
                </div>

                {realSampleNodeName && (
                  <div className="p-3 rounded-xl bg-zinc-50 dark:bg-white/[0.03] border border-zinc-200/80 dark:border-white/10 text-xs">
                    <span className="text-[11px] text-zinc-400 font-semibold mr-2">
                      {t('profiles.renamePreviewLabel')}:
                    </span>
                    <span className="font-mono font-bold text-[#2563EB] dark:text-[#60A5FA] break-all">
                      {realSampleNodeName}
                    </span>
                  </div>
                )}

                <div className="pt-2 border-t border-zinc-200/70 dark:border-white/[0.08] space-y-4">
                  <div>
                    <label className="block text-[11px] font-semibold text-zinc-500 uppercase tracking-wider mb-2">
                      {t('profiles.sortLabel')}
                    </label>
                    <div className="grid grid-cols-3 gap-2">
                      {[
                        { value: '', label: t('profiles.sortDefault') },
                        { value: 'region', label: t('profiles.sortRegion') },
                        { value: 'name', label: t('profiles.sortName') }
                      ].map((opt) => (
                        <button
                          key={opt.value || 'default'}
                          type="button"
                          onClick={() => setForm({ ...form, sort_mode: opt.value })}
                          className={`py-2 px-2 sm:px-3 rounded-xl text-xs font-semibold border transition-all ${
                            (form.sort_mode || '') === opt.value
                              ? 'border-[#2563EB] bg-[#2563EB]/10 text-[#2563EB] dark:text-[#60A5FA]'
                              : 'border-zinc-200/80 dark:border-white/10 text-zinc-600 dark:text-zinc-300 hover:bg-zinc-50 dark:hover:bg-white/[0.03]'
                          }`}
                        >
                          {opt.label}
                        </button>
                      ))}
                    </div>
                  </div>

                  <label className="flex items-center justify-between p-3.5 sm:p-4 rounded-2xl bg-zinc-50 dark:bg-white/[0.03] border border-zinc-200/80 dark:border-white/10 cursor-pointer gap-3">
                    <div className="space-y-0.5 min-w-0">
                      <div className="text-xs sm:text-sm font-bold text-zinc-900 dark:text-white">
                        {t('profiles.regionGroupsTitle')}
                      </div>
                      <div className="text-[11px] text-zinc-500">{t('profiles.regionGroupsDesc')}</div>
                    </div>
                    <input
                      type="checkbox"
                      checked={!!form.region_groups}
                      onChange={(e) => setForm({ ...form, region_groups: e.target.checked })}
                      className="w-5 h-5 accent-[#2563EB] rounded-lg cursor-pointer shrink-0"
                    />
                  </label>

                  <div>
                    <label className="block text-[11px] font-semibold text-zinc-500 uppercase tracking-wider mb-2">
                      {t('profiles.rulePresetLabel')}
                    </label>
                    <div className="grid grid-cols-1 sm:grid-cols-3 gap-2">
                      {[
                        { value: 'cn_direct', label: t('profiles.presetCnDirect'), desc: t('profiles.presetCnDirectDesc') },
                        { value: 'ai_streaming', label: t('profiles.presetAiStreaming'), desc: t('profiles.presetAiStreamingDesc') },
                        { value: 'global', label: t('profiles.presetGlobal'), desc: t('profiles.presetGlobalDesc') }
                      ].map((opt) => (
                        <button
                          key={opt.value}
                          type="button"
                          onClick={() => setForm({ ...form, rule_preset: opt.value })}
                          className={`py-2.5 px-3 rounded-xl text-left border transition-all ${
                            (form.rule_preset || 'cn_direct') === opt.value
                              ? 'border-[#2563EB] bg-[#2563EB]/10 text-[#2563EB] dark:text-[#60A5FA]'
                              : 'border-zinc-200/80 dark:border-white/10 text-zinc-600 dark:text-zinc-300 hover:bg-zinc-50 dark:hover:bg-white/[0.03]'
                          }`}
                        >
                          <div className="text-xs font-semibold">{opt.label}</div>
                          <div className="text-[10px] opacity-75 mt-0.5">{opt.desc}</div>
                        </button>
                      ))}
                    </div>
                  </div>

                  <div>
                    <label className="block text-[11px] font-semibold text-zinc-500 uppercase tracking-wider mb-2">
                      {t('profiles.exportModeLabel')}
                    </label>
                    <div className="grid grid-cols-1 sm:grid-cols-2 gap-2">
                      {[
                        { value: 'full', label: t('profiles.modeFull'), desc: t('profiles.modeFullDesc') },
                        { value: 'provider', label: t('profiles.modeProvider'), desc: t('profiles.modeProviderDesc') }
                      ].map((opt) => (
                        <button
                          key={opt.value}
                          type="button"
                          onClick={() => setForm({ ...form, export_mode: opt.value })}
                          className={`py-2.5 px-3 rounded-xl text-left border transition-all ${
                            (form.export_mode || 'full') === opt.value
                              ? 'border-[#2563EB] bg-[#2563EB]/10 text-[#2563EB] dark:text-[#60A5FA]'
                              : 'border-zinc-200/80 dark:border-white/10 text-zinc-600 dark:text-zinc-300 hover:bg-zinc-50 dark:hover:bg-white/[0.03]'
                          }`}
                        >
                          <div className="text-xs font-semibold">{opt.label}</div>
                          <div className="text-[10px] opacity-75 mt-0.5">{opt.desc}</div>
                        </button>
                      ))}
                    </div>
                  </div>
                </div>
              </div>
            )}

            {modalTab === 'template' && (
              <div className="space-y-4">
                <p className="text-[11px] text-zinc-400 leading-relaxed">
                  {t('profiles.templateIntro')}
                </p>

                {[
                  {
                    formatKey: 'clash',
                    title: t('profiles.clashTemplateTitle'),
                    contentKey: 'clash_template',
                    urlKey: 'clash_template_url'
                  },
                  {
                    formatKey: 'singbox',
                    title: t('profiles.singboxTemplateTitle'),
                    contentKey: 'singbox_template',
                    urlKey: 'singbox_template_url'
                  }
                ].map((sec) => (
                  <div key={sec.formatKey} className="rounded-xl border border-zinc-200/70 dark:border-white/[0.08] bg-zinc-50/80 dark:bg-white/[0.02] p-3.5 space-y-2.5">
                    <div className="flex items-center justify-between">
                      <span className="text-xs font-bold text-zinc-900 dark:text-white">{sec.title}</span>
                      <span className="hero-chip-default text-[10px] font-mono">
                        {sec.formatKey === 'clash' ? 'YAML' : 'JSON'}
                      </span>
                    </div>

                    <textarea
                      value={form[sec.contentKey]}
                      onChange={(e) => setForm({ ...form, [sec.contentKey]: e.target.value })}
                      rows={5}
                      placeholder={sec.formatKey === 'clash' ? t('profiles.clashTemplatePh') : t('profiles.singboxTemplatePh')}
                      className="hero-input p-3 text-[11px] font-mono text-zinc-900 dark:text-white leading-relaxed"
                    />

                    <div>
                      <label className="block text-[10px] font-semibold text-zinc-500 uppercase tracking-wider mb-1">
                        {t('profiles.templateUrlLabel')}
                      </label>
                      <input
                        type="text"
                        value={form[sec.urlKey]}
                        onChange={(e) => setForm({ ...form, [sec.urlKey]: e.target.value })}
                        placeholder="https://example.com/template"
                        className="hero-input px-3 py-1.5 text-[11px] font-mono"
                      />
                      <p className="text-[10px] text-zinc-400 mt-1">{t('profiles.templateUrlHint')}</p>
                    </div>
                  </div>
                ))}

                {form.template_status && (
                  <div className="p-3 rounded-xl bg-amber-500/10 border border-amber-500/20 text-[11px] text-amber-600 dark:text-amber-400 break-all leading-relaxed">
                    {form.template_status}
                  </div>
                )}

                <p className="text-[10px] text-zinc-400 leading-relaxed">{t('profiles.templateMergeHint')}</p>
              </div>
            )}
          </form>
        </div>
      </Drawer>

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
        probing={probingModal}
        onProbe={handleProbeProfile}
        emptyText={t('profiles.emptyNodes')}
        onClose={() => setPreviewState({ open: false, profileId: '', name: '', nodes: [], loading: false })}
        onCopy={copyText}
      />

      <ConfigPreviewDrawer
        open={!!previewConfigProfile}
        profile={previewConfigProfile}
        onClose={() => setPreviewConfigProfile(null)}
        onNotify={(msg, type) => showToast(msg, type)}
      />

      <ConfirmDialog
        open={confirmState.open}
        title={confirmState.type === 'delete' ? t('profiles.deleteTitle') : t('profiles.resetTitle')}
        description={
          confirmState.type === 'delete'
            ? t('profiles.deleteDesc', { name: confirmState.profile?.name || '' })
            : t('profiles.resetDesc', { name: confirmState.profile?.name || '' })
        }
        confirmText={confirmState.type === 'delete' ? t('common.delete') : t('profiles.confirmReset')}
        variant={confirmState.type === 'delete' ? 'danger' : 'warning'}
        loading={confirmState.loading}
        onConfirm={handleConfirmAction}
        onCancel={() => setConfirmState({ open: false, type: '', profile: null, loading: false })}
      />
    </div>
  )
}
