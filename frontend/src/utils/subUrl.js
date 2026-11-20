export function getUniversalUrl(token) {
  return `${window.location.origin}/s/${token}`
}

export function getExportUrl(token, format, full) {
  let url = `${window.location.origin}/s/${token}?format=${format}&target=${format}`
  if (full) {
    url += '&type=full&full=1'
  } else {
    url += '&type=provider&full=0'
  }
  return url
}

export function getProtocolBadge(type) {
  const p = (type || '').toLowerCase()
  if (p === 'hysteria2' || p === 'hy2') {
    return 'bg-indigo-500/10 text-indigo-600 dark:text-indigo-400 border-indigo-500/20'
  }
  if (p === 'vless') {
    return 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border-emerald-500/20'
  }
  if (p === 'vmess') {
    return 'bg-blue-500/10 text-blue-600 dark:text-blue-400 border-blue-500/20'
  }
  if (p === 'trojan') {
    return 'bg-amber-500/10 text-amber-600 dark:text-amber-400 border-amber-500/20'
  }
  if (p === 'ss' || p === 'shadowsocks') {
    return 'bg-cyan-500/10 text-cyan-600 dark:text-cyan-400 border-cyan-500/20'
  }
  if (p === 'tuic') {
    return 'bg-sky-500/10 text-sky-600 dark:text-sky-400 border-sky-500/20'
  }
  if (p === 'anytls') {
    return 'bg-teal-500/10 text-teal-600 dark:text-teal-400 border-teal-500/20'
  }
  return 'bg-zinc-500/10 text-zinc-600 dark:text-zinc-400 border-zinc-500/20'
}

export function formatUserInfo(raw) {
  if (!raw) return null
  const map = {}
  raw.split(';').forEach((part) => {
    const [k, v] = part.split('=').map((s) => s.trim().toLowerCase())
    if (k && v && !Number.isNaN(Number(v))) {
      map[k] = Number(v)
    }
  })

  const total = map.total || 0
  const used = (map.upload || 0) + (map.download || 0)
  const formatBytes = (bytes) => {
    if (!bytes) return '0 GB'
    const gb = bytes / (1024 ** 3)
    return gb >= 100 ? `${gb.toFixed(0)} GB` : `${gb.toFixed(1)} GB`
  }

  let trafficText = null
  if (total > 0) {
    const pct = Math.min(100, Math.round((used / total) * 100))
    trafficText = `${formatBytes(used)} / ${formatBytes(total)} (${pct}%)`
  }

  let expireText = null
  if (map.expire && map.expire > 0) {
    const d = new Date(map.expire * 1000)
    expireText = d.toLocaleDateString('zh-CN', { year: 'numeric', month: '2-digit', day: '2-digit' })
  }

  if (!trafficText && !expireText) return null
  return { trafficText, expireText }
}
