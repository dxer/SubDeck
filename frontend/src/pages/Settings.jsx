import React, { useState, useRef, useEffect } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import {
  DatabaseBackup,
  Download,
  Upload,
  KeyRound,
  ShieldCheck,
  CheckCircle2,
  AlertCircle,
  Eye,
  EyeOff,
  Sliders,
  Bell,
  LogOut
} from 'lucide-react'
import { api, setToken } from '../api'
import { useI18n } from '../i18n'
import ConfirmDialog from '../components/ConfirmDialog'

export default function Settings() {
  const { t } = useI18n()
  const [backupLoading, setBackupLoading] = useState(false)
  const [restoreLoading, setRestoreLoading] = useState(false)
  const [pendingRestoreFile, setPendingRestoreFile] = useState(null)
  const [restoreMsg, setRestoreMsg] = useState('')
  const [restoreErr, setRestoreErr] = useState('')
  const fileInputRef = useRef(null)

  const [pwdForm, setPwdForm] = useState({
    oldPassword: '',
    newPassword: '',
    confirmPassword: ''
  })
  const [pwdLoading, setPwdLoading] = useState(false)
  const [showPasswords, setShowPasswords] = useState(false)
  const [pwdSuccess, setPwdSuccess] = useState('')
  const [pwdError, setPwdError] = useState('')

  const [sysSettings, setSysSettings] = useState({
    webhook_url: '',
    ruleset_mirror: '',
    allow_private_fetch: false
  })
  const [sysLoading, setSysLoading] = useState(false)
  const [sysTesting, setSysTesting] = useState(false)
  const [sysMsg, setSysMsg] = useState('')
  const [sysErr, setSysErr] = useState('')

  useEffect(() => {
    api.getSystemSettings()
      .then((res) => {
        if (res) {
          setSysSettings({
            webhook_url: res.webhook_url || '',
            ruleset_mirror: res.ruleset_mirror || '',
            allow_private_fetch: Boolean(res.allow_private_fetch)
          })
        }
      })
      .catch(() => {})
  }, [])

  const handleSaveSystemSettings = async (e) => {
    e.preventDefault()
    setSysLoading(true)
    setSysMsg('')
    setSysErr('')
    try {
      const res = await api.updateSystemSettings(sysSettings)
      if (res?.settings) {
        setSysSettings(res.settings)
      }
      setSysMsg(res?.message || t('common.save'))
    } catch (err) {
      setSysErr(err.message || 'Save failed')
    } finally {
      setSysLoading(false)
    }
  }

  const handleTestWebhook = async () => {
    setSysTesting(true)
    setSysMsg('')
    setSysErr('')
    try {
      const res = await api.testWebhook(sysSettings.webhook_url)
      setSysMsg(res?.message || 'OK')
    } catch (err) {
      setSysErr(err.message || 'Webhook failed')
    } finally {
      setSysTesting(false)
    }
  }

  const handleLogoutAll = async () => {
    try {
      await api.logoutAll()
      setToken('')
      window.location.href = '/login'
    } catch (err) {
      setPwdError(err.message || 'Failed')
    }
  }

  const handleDownloadBackup = async () => {
    setBackupLoading(true)
    setRestoreMsg('')
    setRestoreErr('')
    try {
      await api.downloadBackup()
    } catch (err) {
      setRestoreErr(err.message || 'Export failed')
    } finally {
      setBackupLoading(false)
    }
  }

  const handleFileSelected = (e) => {
    const file = e.target.files?.[0]
    if (!file) return
    setPendingRestoreFile(file)
  }

  const cancelRestore = () => {
    setPendingRestoreFile(null)
    if (fileInputRef.current) fileInputRef.current.value = ''
  }

  const confirmRestore = async () => {
    if (!pendingRestoreFile) return
    setRestoreLoading(true)
    setRestoreMsg('')
    setRestoreErr('')
    try {
      await api.restoreBackup(pendingRestoreFile)
      setPendingRestoreFile(null)
      setRestoreMsg(t('settings.restoreSuccess'))
      setTimeout(() => {
        window.location.reload()
      }, 1400)
    } catch (err) {
      setPendingRestoreFile(null)
      setRestoreErr(err.message || 'Restore failed')
    } finally {
      setRestoreLoading(false)
      if (fileInputRef.current) fileInputRef.current.value = ''
    }
  }

  const passwordStrength = (() => {
    const p = pwdForm.newPassword
    if (!p) return { score: 0, label: '' }
    let score = 1
    if (p.length >= 10) score++
    if (/[A-Z]/.test(p) && /[a-z]/.test(p)) score++
    if (/[0-9]/.test(p) || /[^A-Za-z0-9]/.test(p)) score++
    const labels = [
      '',
      t('settings.strengthWeak'),
      t('settings.strengthFair'),
      t('settings.strengthGood'),
      t('settings.strengthStrong')
    ]
    return { score, label: labels[score] || t('settings.strengthStrong') }
  })()

  const handleChangePassword = async (e) => {
    e.preventDefault()
    setPwdSuccess('')
    setPwdError('')

    if (pwdForm.newPassword.length < 6) {
      setPwdError(t('settings.pwdMinLen'))
      return
    }

    if (pwdForm.newPassword !== pwdForm.confirmPassword) {
      setPwdError(t('settings.pwdMismatch'))
      return
    }

    setPwdLoading(true)
    try {
      await api.updatePassword(pwdForm.oldPassword, pwdForm.newPassword)
      setPwdSuccess(t('settings.pwdSuccess'))
      setPwdForm({
        oldPassword: '',
        newPassword: '',
        confirmPassword: ''
      })
    } catch (err) {
      setPwdError(err.message || 'Failed')
    } finally {
      setPwdLoading(false)
    }
  }

  return (
    <div className="space-y-5 sm:space-y-6 max-w-4xl mx-auto">
      <div className="pb-1">
        <h1 className="text-xl sm:text-2xl font-bold tracking-tight text-zinc-900 dark:text-white">
          {t('settings.title')}
        </h1>
      </div>

      {/* Backup & Restore Card */}
      <motion.div
        initial={{ opacity: 0, y: 10 }}
        animate={{ opacity: 1, y: 0 }}
        className="hero-card p-4 sm:p-6 space-y-4"
      >
        <div className="flex items-center gap-3 pb-3.5 border-b border-zinc-100 dark:border-white/[0.06]">
          <div className="w-9 h-9 rounded-xl bg-[#2563EB]/10 text-[#2563EB] dark:text-[#60A5FA] flex items-center justify-center shrink-0">
            <DatabaseBackup className="w-4 h-4" />
          </div>
          <h2 className="text-sm font-bold text-zinc-900 dark:text-white">{t('settings.backupTitle')}</h2>
        </div>

        <div className="flex flex-col sm:flex-row sm:items-center gap-2.5 sm:gap-3 pt-1">
          <button
            onClick={handleDownloadBackup}
            disabled={backupLoading}
            className="hero-btn-primary px-4 py-2.5 text-xs font-semibold gap-2 disabled:opacity-50 w-full sm:w-auto"
          >
            <Download className="w-4 h-4" />
            <span>{backupLoading ? t('settings.downloading') : t('settings.downloadBackup')}</span>
          </button>

          <label className="hero-btn-bordered px-4 py-2.5 text-xs font-semibold gap-2 cursor-pointer w-full sm:w-auto justify-center">
            <Upload className="w-4 h-4 text-emerald-500" />
            <span>{t('settings.restoreBackup')}</span>
            <input
              ref={fileInputRef}
              type="file"
              accept=".db,.sqlite,.sqlite3"
              className="hidden"
              onChange={handleFileSelected}
            />
          </label>
        </div>

        <AnimatePresence>
          {restoreMsg && (
            <motion.div
              initial={{ opacity: 0, y: -6 }}
              animate={{ opacity: 1, y: 0 }}
              exit={{ opacity: 0, y: -6 }}
              className="p-3 text-xs rounded-xl bg-emerald-500/10 border border-emerald-500/20 text-emerald-600 dark:text-emerald-400 font-medium flex items-center gap-2"
            >
              <CheckCircle2 className="w-4 h-4 shrink-0" />
              <span>{restoreMsg}</span>
            </motion.div>
          )}

          {restoreErr && (
            <motion.div
              initial={{ opacity: 0, y: -6 }}
              animate={{ opacity: 1, y: 0 }}
              exit={{ opacity: 0, y: -6 }}
              className="p-3 text-xs rounded-xl bg-rose-500/10 border border-rose-500/20 text-rose-600 dark:text-rose-400 font-medium flex items-center gap-2"
            >
              <AlertCircle className="w-4 h-4 shrink-0" />
              <span>{restoreErr}</span>
            </motion.div>
          )}
        </AnimatePresence>
      </motion.div>

      {/* Runtime Policies Card */}
      <motion.div
        initial={{ opacity: 0, y: 10 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ delay: 0.02 }}
        className="hero-card p-4 sm:p-6 space-y-4"
      >
        <div className="flex items-center gap-3 pb-3.5 border-b border-zinc-100 dark:border-white/[0.06]">
          <div className="w-9 h-9 rounded-xl bg-amber-500/10 text-amber-600 dark:text-amber-400 flex items-center justify-center shrink-0">
            <Sliders className="w-4 h-4" />
          </div>
          <h2 className="text-sm font-bold text-zinc-900 dark:text-white">{t('settings.opsTitle')}</h2>
        </div>

        <AnimatePresence>
          {sysMsg && (
            <motion.div
              initial={{ opacity: 0, y: -6 }}
              animate={{ opacity: 1, y: 0 }}
              exit={{ opacity: 0, y: -6 }}
              className="p-3 text-xs rounded-xl bg-emerald-500/10 border border-emerald-500/20 text-emerald-600 dark:text-emerald-400 font-medium flex items-center gap-2"
            >
              <CheckCircle2 className="w-4 h-4 shrink-0" />
              <span>{sysMsg}</span>
            </motion.div>
          )}
          {sysErr && (
            <motion.div
              initial={{ opacity: 0, y: -6 }}
              animate={{ opacity: 1, y: 0 }}
              exit={{ opacity: 0, y: -6 }}
              className="p-3 text-xs rounded-xl bg-rose-500/10 border border-rose-500/20 text-rose-600 dark:text-rose-400 font-medium flex items-center gap-2"
            >
              <AlertCircle className="w-4 h-4 shrink-0" />
              <span>{sysErr}</span>
            </motion.div>
          )}
        </AnimatePresence>

        <form onSubmit={handleSaveSystemSettings} className="space-y-4 max-w-2xl">
          <div>
            <div className="flex items-center justify-between gap-2 mb-1.5">
              <label className="block text-[11px] font-semibold text-zinc-500 uppercase tracking-wider">
                {t('settings.webhookLabel')}
              </label>
              <button
                type="button"
                disabled={sysTesting || !sysSettings.webhook_url.trim()}
                onClick={handleTestWebhook}
                className="text-[11px] font-semibold text-[#2563EB] dark:text-[#60A5FA] hover:underline disabled:opacity-40 flex items-center gap-1 shrink-0"
              >
                <Bell className="w-3 h-3" />
                <span>{sysTesting ? t('settings.testingWebhook') : t('settings.testWebhook')}</span>
              </button>
            </div>
            <input
              type="url"
              value={sysSettings.webhook_url}
              onChange={(e) => setSysSettings((s) => ({ ...s, webhook_url: e.target.value }))}
              placeholder={t('settings.webhookPh')}
              className="hero-input px-3.5 py-2.5 text-sm sm:text-xs font-mono text-zinc-900 dark:text-white"
            />
            <p className="text-[11px] text-zinc-400 mt-1">{t('settings.webhookHint')}</p>
          </div>

          <div>
            <div className="flex items-center justify-between gap-2 mb-1.5">
              <label className="block text-[11px] font-semibold text-zinc-500 uppercase tracking-wider">
                {t('settings.mirrorLabel')}
              </label>
              <div className="flex items-center gap-2 text-[11px] shrink-0">
                <button
                  type="button"
                  onClick={() => setSysSettings((s) => ({ ...s, ruleset_mirror: '' }))}
                  className="text-zinc-500 hover:text-zinc-900 dark:hover:text-white"
                >
                  {t('settings.mirrorDirect')}
                </button>
                <span className="text-zinc-300 dark:text-zinc-700">|</span>
                <button
                  type="button"
                  onClick={() => setSysSettings((s) => ({ ...s, ruleset_mirror: 'https://ghproxy.net/' }))}
                  className="text-[#2563EB] dark:text-[#60A5FA] hover:underline font-medium"
                >
                  ghproxy.net
                </button>
              </div>
            </div>
            <input
              type="url"
              value={sysSettings.ruleset_mirror}
              onChange={(e) => setSysSettings((s) => ({ ...s, ruleset_mirror: e.target.value }))}
              placeholder={t('settings.mirrorPh')}
              className="hero-input px-3.5 py-2.5 text-sm sm:text-xs font-mono text-zinc-900 dark:text-white"
            />
          </div>

          <label className="flex items-center justify-between p-3.5 rounded-xl border border-zinc-200/80 dark:border-white/10 bg-zinc-50 dark:bg-white/[0.03] cursor-pointer gap-3">
            <div className="min-w-0">
              <span className="text-xs font-bold text-zinc-900 dark:text-white">{t('settings.allowPrivateTitle')}</span>
              <p className="text-[11px] text-zinc-400 mt-0.5">{t('settings.allowPrivateDesc')}</p>
            </div>
            <input
              type="checkbox"
              checked={!!sysSettings.allow_private_fetch}
              onChange={(e) => setSysSettings((s) => ({ ...s, allow_private_fetch: e.target.checked }))}
              className="w-4 h-4 rounded text-[#2563EB] focus:ring-[#2563EB] shrink-0"
            />
          </label>

          <button
            type="submit"
            disabled={sysLoading}
            className="hero-btn-primary px-5 py-2.5 text-xs font-semibold disabled:opacity-50 w-full sm:w-auto"
          >
            {sysLoading ? t('common.saving') : t('settings.saveOps')}
          </button>
        </form>
      </motion.div>

      {/* Password Change Card */}
      <motion.div
        initial={{ opacity: 0, y: 10 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ delay: 0.04 }}
        className="hero-card p-4 sm:p-6 space-y-4"
      >
        <div className="flex items-center gap-3 pb-3.5 border-b border-zinc-100 dark:border-white/[0.06]">
          <div className="w-9 h-9 rounded-xl bg-zinc-100 dark:bg-white/10 text-zinc-700 dark:text-zinc-200 flex items-center justify-center shrink-0">
            <KeyRound className="w-4 h-4" />
          </div>
          <h2 className="text-sm font-bold text-zinc-900 dark:text-white">{t('settings.passwordTitle')}</h2>
        </div>

        <AnimatePresence>
          {pwdError && (
            <motion.div
              initial={{ opacity: 0, y: -6 }}
              animate={{ opacity: 1, y: 0 }}
              exit={{ opacity: 0, y: -6 }}
              className="p-3 text-xs rounded-xl bg-rose-500/10 border border-rose-500/20 text-rose-600 dark:text-rose-400 font-medium flex items-center gap-2"
            >
              <AlertCircle className="w-4 h-4 shrink-0" />
              <span>{pwdError}</span>
            </motion.div>
          )}

          {pwdSuccess && (
            <motion.div
              initial={{ opacity: 0, y: -6 }}
              animate={{ opacity: 1, y: 0 }}
              exit={{ opacity: 0, y: -6 }}
              className="p-3 text-xs rounded-xl bg-emerald-500/10 border border-emerald-500/20 text-emerald-600 dark:text-emerald-400 font-medium flex items-center gap-2"
            >
              <CheckCircle2 className="w-4 h-4 shrink-0" />
              <span>{pwdSuccess}</span>
            </motion.div>
          )}
        </AnimatePresence>

        <form onSubmit={handleChangePassword} className="space-y-4 w-full sm:max-w-md">
          <div className="space-y-3">
            <div>
              <div className="flex items-center justify-between mb-1.5">
                <label className="block text-[11px] font-semibold text-zinc-500 uppercase tracking-wider">
                  {t('settings.oldPwd')}
                </label>
                <button
                  type="button"
                  onClick={() => setShowPasswords(!showPasswords)}
                  className="text-[11px] text-zinc-400 hover:text-[#2563EB] dark:hover:text-[#60A5FA] flex items-center gap-1 transition-colors"
                >
                  {showPasswords ? <EyeOff className="w-3 h-3" /> : <Eye className="w-3 h-3" />}
                  <span>{showPasswords ? t('login.hide') : t('login.show')}</span>
                </button>
              </div>
              <input
                type={showPasswords ? 'text' : 'password'}
                required
                value={pwdForm.oldPassword}
                onChange={(e) => setPwdForm({ ...pwdForm, oldPassword: e.target.value })}
                placeholder="••••••••"
                className="hero-input px-3.5 py-2.5 text-sm sm:text-xs text-zinc-900 dark:text-white font-mono"
              />
            </div>

            <div>
              <div className="flex items-center justify-between mb-1.5">
                <label className="block text-[11px] font-semibold text-zinc-500 uppercase tracking-wider">
                  {t('settings.newPwd')}
                </label>
                {pwdForm.newPassword && (
                  <span className="text-[11px] font-medium text-zinc-400">
                    {t('settings.pwdStrength', { label: passwordStrength.label })}
                  </span>
                )}
              </div>
              <input
                type={showPasswords ? 'text' : 'password'}
                required
                value={pwdForm.newPassword}
                onChange={(e) => setPwdForm({ ...pwdForm, newPassword: e.target.value })}
                placeholder="••••••••"
                className="hero-input px-3.5 py-2.5 text-sm sm:text-xs text-zinc-900 dark:text-white font-mono"
              />
              {pwdForm.newPassword && (
                <div className="grid grid-cols-4 gap-1 mt-1.5">
                  {[1, 2, 3, 4].map((level) => (
                    <span
                      key={level}
                      className={`h-1 rounded-full transition-colors ${
                        passwordStrength.score >= level
                          ? passwordStrength.score >= 3
                            ? 'bg-emerald-500'
                            : 'bg-amber-500'
                          : 'bg-zinc-200 dark:bg-white/10'
                      }`}
                    />
                  ))}
                </div>
              )}
            </div>

            <div>
              <label className="block text-[11px] font-semibold text-zinc-500 uppercase tracking-wider mb-1.5">
                {t('settings.confirmPwd')}
              </label>
              <input
                type={showPasswords ? 'text' : 'password'}
                required
                value={pwdForm.confirmPassword}
                onChange={(e) => setPwdForm({ ...pwdForm, confirmPassword: e.target.value })}
                placeholder="••••••••"
                className="hero-input px-3.5 py-2.5 text-sm sm:text-xs text-zinc-900 dark:text-white font-mono"
              />
            </div>
          </div>

          <div className="flex flex-col sm:flex-row sm:items-center gap-2.5 sm:gap-3">
            <button
              type="submit"
              disabled={pwdLoading}
              className="hero-btn-primary px-5 py-2.5 text-xs font-semibold disabled:opacity-50 w-full sm:w-auto"
            >
              {pwdLoading ? t('settings.submittingPwd') : t('settings.submitPwd')}
            </button>
            <button
              type="button"
              onClick={handleLogoutAll}
              className="hero-btn-bordered px-4 py-2.5 text-xs font-medium text-rose-600 dark:text-rose-400 gap-1.5 w-full sm:w-auto"
            >
              <LogOut className="w-3.5 h-3.5" />
              <span>{t('settings.logoutAll')}</span>
            </button>
          </div>
        </form>
      </motion.div>

      {/* Runtime Information Card */}
      <motion.div
        initial={{ opacity: 0, y: 10 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ delay: 0.08 }}
        className="hero-card p-4 sm:p-6 space-y-4"
      >
        <div className="flex items-center justify-between pb-3.5 border-b border-zinc-100 dark:border-white/[0.06]">
          <div className="flex items-center gap-2">
            <ShieldCheck className="w-4 h-4 text-emerald-500" />
            <h3 className="text-xs font-bold uppercase tracking-wider text-zinc-700 dark:text-zinc-300">
              {t('settings.infoTitle')}
            </h3>
          </div>
          <span className="text-[11px] font-mono text-zinc-400">SubDeck</span>
        </div>

        <div className="grid sm:grid-cols-2 gap-3 text-xs">
          <div className="p-3.5 bg-zinc-50 dark:bg-white/[0.02] rounded-xl border border-zinc-200/60 dark:border-white/[0.06] flex items-center justify-between">
            <span className="text-zinc-500 dark:text-zinc-400">{t('settings.infoRateLogin')}</span>
            <span className="font-mono font-medium text-zinc-800 dark:text-zinc-200">{t('settings.infoRateLoginVal')}</span>
          </div>
          <div className="p-3.5 bg-zinc-50 dark:bg-white/[0.02] rounded-xl border border-zinc-200/60 dark:border-white/[0.06] flex items-center justify-between">
            <span className="text-zinc-500 dark:text-zinc-400">{t('settings.infoRateSub')}</span>
            <span className="font-mono font-medium text-zinc-800 dark:text-zinc-200">{t('settings.infoRateSubVal')}</span>
          </div>
          <div className="p-3.5 bg-zinc-50 dark:bg-white/[0.02] rounded-xl border border-zinc-200/60 dark:border-white/[0.06] flex items-center justify-between">
            <span className="text-zinc-500 dark:text-zinc-400">{t('settings.infoEndpoint')}</span>
            <span className="font-mono font-medium text-zinc-800 dark:text-zinc-200">/s/:token</span>
          </div>
          <div className="p-3.5 bg-zinc-50 dark:bg-white/[0.02] rounded-xl border border-zinc-200/60 dark:border-white/[0.06] flex items-center justify-between">
            <span className="text-zinc-500 dark:text-zinc-400">{t('settings.infoStorage')}</span>
            <span className="font-mono font-medium text-zinc-800 dark:text-zinc-200">SQLite 3 (WAL)</span>
          </div>
        </div>
      </motion.div>

      <ConfirmDialog
        open={!!pendingRestoreFile}
        title={t('settings.restoreDialogTitle')}
        description={
          pendingRestoreFile
            ? t('settings.restoreDialogDesc', { name: pendingRestoreFile.name })
            : ''
        }
        confirmText={t('settings.confirmRestore')}
        loading={restoreLoading}
        onConfirm={confirmRestore}
        onCancel={cancelRestore}
      />
    </div>
  )
}
