import React, { useState, useRef } from 'react'
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
  EyeOff
} from 'lucide-react'
import { api } from '../api'
import ConfirmDialog from '../components/ConfirmDialog'

export default function Settings() {
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

  const handleDownloadBackup = async () => {
    setBackupLoading(true)
    setRestoreMsg('')
    setRestoreErr('')
    try {
      await api.downloadBackup()
    } catch (err) {
      setRestoreErr(err.message || '下载热备份失败')
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
      setRestoreMsg('数据库已成功恢复！系统正在重新加载...')
      setTimeout(() => {
        window.location.reload()
      }, 1500)
    } catch (err) {
      setPendingRestoreFile(null)
      setRestoreErr(err.message || '恢复数据库失败，请确保文件是有效的 SQLite 数据库')
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
    const labels = ['', '较弱', '一般', '良好', '强']
    return { score, label: labels[score] || '强' }
  })()

  const handleChangePassword = async (e) => {
    e.preventDefault()
    setPwdSuccess('')
    setPwdError('')

    if (pwdForm.newPassword.length < 6) {
      setPwdError('新密码长度不能少于 6 位')
      return
    }

    if (pwdForm.newPassword !== pwdForm.confirmPassword) {
      setPwdError('两次输入的新密码不一致')
      return
    }

    setPwdLoading(true)
    try {
      await api.updatePassword(pwdForm.oldPassword, pwdForm.newPassword)
      setPwdSuccess('管理员登录密码已成功更新！请妥善保管新密码。')
      setPwdForm({
        oldPassword: '',
        newPassword: '',
        confirmPassword: ''
      })
    } catch (err) {
      setPwdError(err.message || '修改密码失败')
    } finally {
      setPwdLoading(false)
    }
  }

  return (
    <div className="space-y-6 max-w-4xl mx-auto">
      {/* Header Banner */}
      <div className="hero-card p-5 flex items-center justify-between">
        <div>
          <h1 className="text-lg sm:text-xl font-bold tracking-tight text-zinc-900 dark:text-white">
            系统设置与安全
          </h1>
          <p className="text-xs text-zinc-400 mt-0.5">管理 SQLite 热备份快照、管理员凭证与安全运行策略。</p>
        </div>
      </div>

      {/* Backup & Restore Card */}
      <motion.div
        initial={{ opacity: 0, y: 10 }}
        animate={{ opacity: 1, y: 0 }}
        className="hero-card p-6 space-y-4"
      >
        <div className="flex items-center gap-3 pb-3.5 border-b border-zinc-100 dark:border-white/[0.06]">
          <div className="w-9 h-9 rounded-xl bg-[#2563EB]/10 text-[#2563EB] dark:text-[#60A5FA] flex items-center justify-center">
            <DatabaseBackup className="w-4 h-4" />
          </div>
          <div>
            <h2 className="text-sm font-bold text-zinc-900 dark:text-white">数据库热备份与恢复</h2>
            <p className="text-[11px] text-zinc-400">
              基于 SQLite WAL Checkpoint 机制，导出与原子恢复前均会自动校验数据库完整性。
            </p>
          </div>
        </div>

        <div className="flex flex-wrap items-center gap-3 pt-1">
          <button
            onClick={handleDownloadBackup}
            disabled={backupLoading}
            className="hero-btn-primary px-4 py-2.5 text-xs font-semibold gap-2 disabled:opacity-50"
          >
            <Download className="w-4 h-4" />
            <span>{backupLoading ? '正在打包...' : '下载数据库备份 (.db)'}</span>
          </button>

          <label className="hero-btn-bordered px-4 py-2.5 text-xs font-semibold gap-2 cursor-pointer">
            <Upload className="w-4 h-4 text-emerald-500" />
            <span>从备份文件恢复</span>
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

      {/* Password Change Card */}
      <motion.div
        initial={{ opacity: 0, y: 10 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ delay: 0.04 }}
        className="hero-card p-6 space-y-4"
      >
        <div className="flex items-center gap-3 pb-3.5 border-b border-zinc-100 dark:border-white/[0.06]">
          <div className="w-9 h-9 rounded-xl bg-zinc-100 dark:bg-white/10 text-zinc-700 dark:text-zinc-200 flex items-center justify-center">
            <KeyRound className="w-4 h-4" />
          </div>
          <div>
            <h2 className="text-sm font-bold text-zinc-900 dark:text-white">修改管理员登录密码</h2>
            <p className="text-[11px] text-zinc-400">建议使用 10 位以上包含大小写与数字的高强度密码。</p>
          </div>
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

        <form onSubmit={handleChangePassword} className="space-y-4 max-w-sm">
          <div className="space-y-3">
            <div>
              <div className="flex items-center justify-between mb-1.5">
                <label className="block text-[11px] font-semibold text-zinc-500 uppercase tracking-wider">
                  当前旧密码
                </label>
                <button
                  type="button"
                  onClick={() => setShowPasswords(!showPasswords)}
                  className="text-[11px] text-zinc-400 hover:text-[#2563EB] dark:hover:text-[#60A5FA] flex items-center gap-1 transition-colors"
                >
                  {showPasswords ? <EyeOff className="w-3 h-3" /> : <Eye className="w-3 h-3" />}
                  <span>{showPasswords ? '隐藏密码' : '显示密码'}</span>
                </button>
              </div>
              <input
                type={showPasswords ? 'text' : 'password'}
                required
                value={pwdForm.oldPassword}
                onChange={(e) => setPwdForm({ ...pwdForm, oldPassword: e.target.value })}
                placeholder="••••••••"
                className="hero-input px-3.5 py-2.5 text-xs text-zinc-900 dark:text-white font-mono"
              />
            </div>

            <div>
              <div className="flex items-center justify-between mb-1.5">
                <label className="block text-[11px] font-semibold text-zinc-500 uppercase tracking-wider">
                  新密码 (至少 6 位)
                </label>
                {pwdForm.newPassword && (
                  <span className="text-[11px] font-medium text-zinc-400">
                    强度: <span className="text-zinc-700 dark:text-zinc-200">{passwordStrength.label}</span>
                  </span>
                )}
              </div>
              <input
                type={showPasswords ? 'text' : 'password'}
                required
                value={pwdForm.newPassword}
                onChange={(e) => setPwdForm({ ...pwdForm, newPassword: e.target.value })}
                placeholder="••••••••"
                className="hero-input px-3.5 py-2.5 text-xs text-zinc-900 dark:text-white font-mono"
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
                确认新密码
              </label>
              <input
                type={showPasswords ? 'text' : 'password'}
                required
                value={pwdForm.confirmPassword}
                onChange={(e) => setPwdForm({ ...pwdForm, confirmPassword: e.target.value })}
                placeholder="••••••••"
                className="hero-input px-3.5 py-2.5 text-xs text-zinc-900 dark:text-white font-mono"
              />
            </div>
          </div>

          <button
            type="submit"
            disabled={pwdLoading}
            className="hero-btn-primary px-5 py-2.5 text-xs font-semibold disabled:opacity-50"
          >
            {pwdLoading ? '修改中...' : '提交修改'}
          </button>
        </form>
      </motion.div>

      {/* Security & Architecture Information Card */}
      <motion.div
        initial={{ opacity: 0, y: 10 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ delay: 0.08 }}
        className="hero-card p-6 space-y-4"
      >
        <div className="flex items-center justify-between pb-3.5 border-b border-zinc-100 dark:border-white/[0.06]">
          <div className="flex items-center gap-2">
            <ShieldCheck className="w-4 h-4 text-emerald-500" />
            <h3 className="text-xs font-bold uppercase tracking-wider text-zinc-700 dark:text-zinc-300">
              安全与访问策略
            </h3>
          </div>
          <span className="text-[11px] font-mono text-zinc-400">SubDeck v0.1.1</span>
        </div>

        <div className="grid sm:grid-cols-2 gap-3 text-xs">
          <div className="p-3.5 bg-zinc-50 dark:bg-white/[0.02] rounded-xl border border-zinc-200/60 dark:border-white/[0.06] flex items-center justify-between">
            <span className="text-zinc-500 dark:text-zinc-400">认证防爆破策略</span>
            <span className="font-mono font-medium text-zinc-800 dark:text-zinc-200">5次失败 / 锁定15m</span>
          </div>
          <div className="p-3.5 bg-zinc-50 dark:bg-white/[0.02] rounded-xl border border-zinc-200/60 dark:border-white/[0.06] flex items-center justify-between">
            <span className="text-zinc-500 dark:text-zinc-400">订阅拉取限流</span>
            <span className="font-mono font-medium text-zinc-800 dark:text-zinc-200">滑动窗口 20 req / 30s</span>
          </div>
          <div className="p-3.5 bg-zinc-50 dark:bg-white/[0.02] rounded-xl border border-zinc-200/60 dark:border-white/[0.06] flex items-center justify-between">
            <span className="text-zinc-500 dark:text-zinc-400">自适应分发端点</span>
            <span className="font-mono font-medium text-zinc-800 dark:text-zinc-200">/s/:token</span>
          </div>
          <div className="p-3.5 bg-zinc-50 dark:bg-white/[0.02] rounded-xl border border-zinc-200/60 dark:border-white/[0.06] flex items-center justify-between">
            <span className="text-zinc-500 dark:text-zinc-400">存储引擎与模式</span>
            <span className="font-mono font-medium text-zinc-800 dark:text-zinc-200">SQLite 3 (WAL)</span>
          </div>
        </div>
      </motion.div>

      <ConfirmDialog
        open={!!pendingRestoreFile}
        title="确认恢复数据库备份？"
        description={
          pendingRestoreFile
            ? `即将从 "${pendingRestoreFile.name}" 覆盖当前数据库，所有现存订阅与组合配置将被替换（系统会自动保留 .bak 备份）。`
            : ''
        }
        confirmText="确认恢复"
        loading={restoreLoading}
        onConfirm={confirmRestore}
        onCancel={cancelRestore}
      />
    </div>
  )
}
