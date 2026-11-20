import React, { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { motion, AnimatePresence } from 'framer-motion'
import { AlertCircle, Eye, EyeOff, ShieldCheck, Sun, Moon } from 'lucide-react'
import { api, setToken, setUser } from '../api'
import SubDeckMark from '../components/SubDeckLogo'

export default function Login({ isDark, toggleTheme }) {
  const navigate = useNavigate()
  const [form, setForm] = useState({
    username: '',
    password: ''
  })
  const [loading, setLoading] = useState(false)
  const [showPassword, setShowPassword] = useState(false)
  const [errorMsg, setErrorMsg] = useState('')

  const handleLogin = async (e) => {
    e.preventDefault()
    setLoading(true)
    setErrorMsg('')
    try {
      const res = await api.login(form.username, form.password)
      setToken(res.token)
      setUser(res.username)
      navigate('/')
    } catch (err) {
      setErrorMsg(err.message || '登录失败，请检查账号密码')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="relative min-h-[100dvh] flex items-center justify-center p-4 bg-[#F6F7F9] dark:bg-[#090B0F] text-zinc-900 dark:text-zinc-100 font-sans ambient-grid transition-colors duration-150">
      {/* Theme Toggle */}
      {toggleTheme && (
        <button
          type="button"
          onClick={toggleTheme}
          title={isDark ? '切换浅色主题' : '切换深色主题'}
          className="fixed top-5 right-5 p-2.5 rounded-xl bg-white/90 dark:bg-[#11141B]/90 border border-zinc-200/80 dark:border-white/10 shadow-xs text-zinc-500 hover:text-zinc-900 dark:hover:text-white transition-all active:scale-95"
        >
          {isDark ? <Sun className="w-4 h-4 text-amber-400" /> : <Moon className="w-4 h-4 text-[#2563EB]" />}
        </button>
      )}

      <motion.div
        initial={{ opacity: 0, y: 12 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.22, ease: [0.16, 1, 0.3, 1] }}
        className="w-full max-w-sm bg-white dark:bg-[#11141B] rounded-2xl shadow-xl shadow-slate-900/[0.06] dark:shadow-black/50 border border-zinc-200/80 dark:border-white/10 p-8 space-y-6"
      >
        {/* Header */}
        <div className="space-y-3">
          <div className="flex items-center justify-between">
            <SubDeckMark className="w-11 h-11 rounded-xl shadow-sm" />
            <span className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full bg-emerald-500/10 text-emerald-700 dark:text-emerald-400 border border-emerald-500/20 text-[11px] font-medium">
              <ShieldCheck className="w-3.5 h-3.5" />
              <span>防暴破保护已启用</span>
            </span>
          </div>

          <div>
            <div className="flex items-center gap-2">
              <h1 className="text-xl font-extrabold tracking-tight text-zinc-900 dark:text-white">
                SubDeck
              </h1>
              <span className="px-1.5 py-0.5 rounded-md text-[10px] font-mono font-bold bg-zinc-100 dark:bg-white/10 text-zinc-600 dark:text-zinc-300 border border-zinc-200/70 dark:border-white/10 leading-none">
                CONSOLE
              </span>
            </div>
            <p className="text-xs text-zinc-500 dark:text-zinc-400 mt-1">
              登录控制台以管理订阅源、规则流水线与分发节点
            </p>
          </div>
        </div>

        {/* Error Alert */}
        <AnimatePresence>
          {errorMsg && (
            <motion.div
              initial={{ opacity: 0, y: -6 }}
              animate={{ opacity: 1, y: 0 }}
              exit={{ opacity: 0, y: -6 }}
              className="p-3 text-xs rounded-xl bg-rose-500/10 border border-rose-500/20 text-rose-600 dark:text-rose-400 font-medium flex items-center gap-2"
            >
              <AlertCircle className="w-4 h-4 shrink-0" />
              <span>{errorMsg}</span>
            </motion.div>
          )}
        </AnimatePresence>

        {/* Form */}
        <form onSubmit={handleLogin} className="space-y-4">
          <div className="space-y-1.5">
            <label
              htmlFor="login-username"
              className="block text-[11px] font-semibold text-zinc-600 dark:text-zinc-400 uppercase tracking-wider"
            >
              管理员账号
            </label>
            <input
              id="login-username"
              type="text"
              required
              autoFocus
              value={form.username}
              onChange={(e) => setForm({ ...form, username: e.target.value })}
              placeholder="请输入管理员账号"
              className="hero-input px-3.5 py-2.5 text-xs text-zinc-900 dark:text-white placeholder:text-zinc-400"
            />
          </div>

          <div className="space-y-1.5">
            <div className="flex items-center justify-between">
              <label
                htmlFor="login-password"
                className="block text-[11px] font-semibold text-zinc-600 dark:text-zinc-400 uppercase tracking-wider"
              >
                登录密码
              </label>
              <button
                type="button"
                onClick={() => setShowPassword(!showPassword)}
                className="text-[11px] text-zinc-400 hover:text-[#2563EB] dark:hover:text-[#60A5FA] flex items-center gap-1 transition-colors"
              >
                {showPassword ? <EyeOff className="w-3 h-3" /> : <Eye className="w-3 h-3" />}
                <span>{showPassword ? '隐藏' : '显示'}</span>
              </button>
            </div>
            <input
              id="login-password"
              type={showPassword ? 'text' : 'password'}
              required
              value={form.password}
              onChange={(e) => setForm({ ...form, password: e.target.value })}
              placeholder="请输入密码"
              className="hero-input px-3.5 py-2.5 text-xs text-zinc-900 dark:text-white placeholder:text-zinc-400"
            />
          </div>

          <button
            type="submit"
            disabled={loading}
            className="w-full hero-btn-primary py-2.5 text-xs font-semibold gap-2 disabled:opacity-50 mt-2"
          >
            {loading ? (
              <>
                <span className="animate-spin w-3.5 h-3.5 border-2 border-white border-t-transparent rounded-full" />
                <span>认证中...</span>
              </>
            ) : (
              <span>登录控制台</span>
            )}
          </button>
        </form>
      </motion.div>
    </div>
  )
}
