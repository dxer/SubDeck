const BASE_URL = ''

export function getToken() {
  return localStorage.getItem('sublite_token') || ''
}

export function setToken(token) {
  if (token) {
    localStorage.setItem('sublite_token', token)
  } else {
    localStorage.removeItem('sublite_token')
  }
}

export function getUser() {
  return localStorage.getItem('sublite_user') || 'admin'
}

export function setUser(user) {
  if (user) {
    localStorage.setItem('sublite_user', user)
  } else {
    localStorage.removeItem('sublite_user')
  }
}

export async function request(url, options = {}) {
  const token = getToken()
  const headers = {
    'Content-Type': 'application/json',
    ...(options.headers || {})
  }

  if (token) {
    headers['Authorization'] = `Bearer ${token}`
  }

  const response = await fetch(`${BASE_URL}${url}`, {
    ...options,
    headers
  })

  if (response.status === 401) {
    setToken('')
    if (window.location.pathname !== '/login') {
      window.location.href = '/login'
    }
    throw new Error('登录凭证已失效，请重新登录')
  }

  const data = await response.json().catch(() => ({}))

  if (!response.ok) {
    throw new Error(data.error || `请求失败 (${response.status})`)
  }

  return data
}

export const api = {
  // Auth
  login: (username, password) => request('/api/auth/login', {
    method: 'POST',
    body: JSON.stringify({ username, password })
  }),
  updatePassword: async (old_password, new_password) => {
    const res = await request('/api/auth/password', {
      method: 'POST',
      body: JSON.stringify({ old_password, new_password })
    })
    if (res?.token) {
      setToken(res.token)
    }
    return res
  },
  logoutAll: () => request('/api/auth/logout-all', { method: 'POST' }),
  getMe: () => request('/api/auth/me'),

  // Dashboard
  getStats: () => request('/api/dashboard/stats'),

  // Unified Subscriptions (Remote & Local)
  getSubscriptions: (type = '') => request(type ? `/api/subscriptions?type=${type}` : '/api/subscriptions'),
  createSubscription: (data) => request('/api/subscriptions', {
    method: 'POST',
    body: JSON.stringify(data)
  }),
  updateSubscription: (id, data) => request(`/api/subscriptions/${id}`, {
    method: 'PUT',
    body: JSON.stringify(data)
  }),
  deleteSubscription: (id) => request(`/api/subscriptions/${id}`, {
    method: 'DELETE'
  }),
  refreshSubscription: (id) => request(`/api/subscriptions/${id}/refresh`, {
    method: 'POST'
  }),
  probeSubscription: (id) => request(`/api/subscriptions/${id}/probe`, {
    method: 'POST'
  }),
  getSubscriptionNodes: (id) => request(`/api/subscriptions/${id}/nodes`),

  // Profiles (Combinations)
  getProfiles: () => request('/api/profiles'),
  createProfile: (data) => request('/api/profiles', {
    method: 'POST',
    body: JSON.stringify(data)
  }),
  updateProfile: (id, data) => request(`/api/profiles/${id}`, {
    method: 'PUT',
    body: JSON.stringify(data)
  }),
  deleteProfile: (id) => request(`/api/profiles/${id}`, {
    method: 'DELETE'
  }),
  resetProfileToken: (id) => request(`/api/profiles/${id}/reset-token`, {
    method: 'POST'
  }),
  probeProfile: (id) => request(`/api/profiles/${id}/probe`, {
    method: 'POST'
  }),
  previewProfile: (id) => request(`/api/profiles/${id}/preview`),
  previewDraftProfile: (data) => request('/api/profiles/preview', {
    method: 'POST',
    body: JSON.stringify(data)
  }),

  // System Settings & Webhook
  getSystemSettings: () => request('/api/system/settings'),
  updateSystemSettings: (data) => request('/api/system/settings', {
    method: 'PUT',
    body: JSON.stringify(data)
  }),
  testWebhook: (webhook_url) => request('/api/system/webhook-test', {
    method: 'POST',
    body: JSON.stringify({ webhook_url })
  }),

  // System Backup & Restore
  downloadBackup: async () => {
    const token = getToken()
    const res = await fetch('/api/system/backup', {
      headers: { 'Authorization': `Bearer ${token}` }
    })
    if (!res.ok) throw new Error('下载备份失败')
    const blob = await res.blob()
    const url = window.URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `subdeck-backup-${new Date().toISOString().slice(0, 10)}.db`
    document.body.appendChild(a)
    a.click()
    window.URL.revokeObjectURL(url)
    a.remove()
  },
  restoreBackup: async (file) => {
    const token = getToken()
    const formData = new FormData()
    formData.append('backup_file', file)
    const res = await fetch('/api/system/restore', {
      method: 'POST',
      headers: { 'Authorization': `Bearer ${token}` },
      body: formData
    })
    const data = await res.json().catch(() => ({}))
    if (!res.ok) throw new Error(data.error || '恢复备份失败')
    return data
  }
}
