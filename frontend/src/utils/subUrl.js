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

// Parses a raw Subscription-Userinfo header into a structured quota object:
// { upload, download, total, used, usedPercent, expire, daysLeft, expired }.
// Returns null when the header carries no recognizable fields.
export function parseUserQuota(raw) {
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
  const usedPercent = total > 0 ? Math.min(100, Math.round((used / total) * 100)) : null

  let expire = null
  let daysLeft = null
  let expired = false
  if (map.expire && map.expire > 0) {
    expire = map.expire
    const msLeft = expire * 1000 - Date.now()
    expired = msLeft <= 0
    daysLeft = expired ? 0 : Math.max(0, Math.floor(msLeft / 86400000))
  }

  if (total <= 0 && expire === null) return null
  return { upload: map.upload || 0, download: map.download || 0, total, used, usedPercent, expire, daysLeft, expired }
}

export function formatUserInfo(raw, lang = 'zh') {
  const quota = parseUserQuota(raw)
  if (!quota) return null

  const formatBytes = (bytes) => {
    if (!bytes) return '0 GB'
    const gb = bytes / (1024 ** 3)
    return gb >= 100 ? `${gb.toFixed(0)} GB` : `${gb.toFixed(1)} GB`
  }

  let trafficText = null
  if (quota.total > 0) {
    trafficText = `${formatBytes(quota.used)} / ${formatBytes(quota.total)} (${quota.usedPercent}%)`
  }

  let expireText = null
  if (quota.expire) {
    const d = new Date(quota.expire * 1000)
    expireText = d.toLocaleDateString(lang === 'en' ? 'en-US' : 'zh-CN', { year: 'numeric', month: '2-digit', day: '2-digit' })
  }

  if (!trafficText && !expireText) return null
  return { trafficText, expireText, quota }
}

// Maps a parsed quota to display severities for traffic and expiry:
// traffic: null (no data) | 'ok' | 'warn' (>=80%) | 'danger' (>=90%)
// expiry:  null (no data) | 'ok' | 'warn' (<=7d) | 'danger' (<=3d) | 'expired'
export function quotaSeverity(quota) {
  if (!quota) return { traffic: null, expiry: null }
  const traffic =
    quota.usedPercent == null
      ? null
      : quota.usedPercent >= 90
        ? 'danger'
        : quota.usedPercent >= 80
          ? 'warn'
          : 'ok'
  const expiry =
    quota.expire == null
      ? null
      : quota.expired
        ? 'expired'
        : quota.daysLeft <= 3
          ? 'danger'
          : quota.daysLeft <= 7
            ? 'warn'
            : 'ok'
  return { traffic, expiry }
}

export async function copyToClipboard(text) {
  if (!text) return false

  if (navigator.clipboard && window.isSecureContext) {
    try {
      await navigator.clipboard.writeText(text)
      return true
    } catch {
      // Fall through to legacy textarea fallback
    }
  }

  const textarea = document.createElement('textarea')
  textarea.value = text
  textarea.setAttribute('readonly', '')
  textarea.style.position = 'fixed'
  textarea.style.top = '-9999px'
  textarea.style.left = '-9999px'
  textarea.style.opacity = '0'
  document.body.appendChild(textarea)

  const selection = document.getSelection()
  const prevRange = selection && selection.rangeCount > 0 ? selection.getRangeAt(0) : null

  textarea.focus()
  textarea.select()
  textarea.setSelectionRange(0, text.length)

  let ok = false
  try {
    ok = document.execCommand('copy')
  } catch {
    ok = false
  } finally {
    document.body.removeChild(textarea)
    if (selection && prevRange) {
      selection.removeAllRanges()
      selection.addRange(prevRange)
    }
  }

  return ok
}
