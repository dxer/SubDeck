import React, { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { motion, AnimatePresence } from 'framer-motion'
import { AlertCircle, Eye, EyeOff, Sun, Moon, Languages } from 'lucide-react'
import { api, setToken, setUser } from '../api'
import { useI18n } from '../i18n'
import SubDeckMark from '../components/SubDeckLogo'

export default function Login({ isDark, toggleTheme }) {
  const { t, toggleLang } = useI18n()
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
      setErrorMsg(err.message || t('login.errorFallback'))
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="relative min-h-[100dvh] flex items-center justify-center p-4 bg-[#F6F7F9] dark:bg-[#090B0F] text-zinc-900 dark:text-zinc-100 font-sans ambient-grid transition-colors duration-150">
      {/* Top-Right Lang & Theme Controls */}
      <div className="fixed top-4 right-4 sm:top-5 sm:right-5 flex items-center gap-2">
        <button
          type="button"
          onClick={toggleLang}
          title={t('common.switchLang')}
          className="inline-flex items-center gap-1.5 px-3 py-2 rounded-xl bg-white/90 dark:bg-[#11141B]/90 border border-zinc-200/80 dark:border-white/10 shadow-xs text-xs font-mono font-semibold text-zinc-600 dark:text-zinc-300 hover:text-zinc-900 dark:hover:text-white transition-all active:scale-95"
        >
          <Languages className="w-3.5 h-3.5 text-zinc-400" />
          <span>{t('common.langLabel')}</span>
        </button>

        {toggleTheme && (
          <button
            type="button"
            onClick={toggleTheme}
            title={isDark ? t('common.lightTheme') : t('common.darkTheme')}
            className="p-2.5 rounded-xl bg-white/90 dark:bg-[#11141B]/90 border border-zinc-200/80 dark:border-white/10 shadow-xs text-zinc-500 hover:text-zinc-900 dark:hover:text-white transition-all active:scale-95"
          >
            {isDark ? <Sun className="w-4 h-4 text-amber-400" /> : <Moon className="w-4 h-4 text-[#2563EB]" />}
          </button>
        )}
      </div>

      <motion.div
        initial={{ opacity: 0, y: 12 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.22, ease: [0.16, 1, 0.3, 1] }}
        className="w-full max-w-sm bg-white dark:bg-[#11141B] rounded-2xl shadow-xl shadow-slate-900/[0.06] dark:shadow-black/50 border border-zinc-200/80 dark:border-white/10 p-6 sm:p-8 space-y-6"
      >
        {/* Brand */}
        <div className="flex items-center gap-3">
          <SubDeckMark className="w-10 h-10 rounded-xl shadow-sm shrink-0" />
          <h1 className="text-xl font-extrabold tracking-tight text-zinc-900 dark:text-white">
            SubDeck
          </h1>
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
              {t('login.username')}
            </label>
            <input
              id="login-username"
              type="text"
              required
              autoFocus
              value={form.username}
              onChange={(e) => setForm({ ...form, username: e.target.value })}
              placeholder={t('login.usernamePlaceholder')}
              className="hero-input px-3.5 py-2.5 text-sm sm:text-xs text-zinc-900 dark:text-white placeholder:text-zinc-400"
            />
          </div>

          <div className="space-y-1.5">
            <div className="flex items-center justify-between">
              <label
                htmlFor="login-password"
                className="block text-[11px] font-semibold text-zinc-600 dark:text-zinc-400 uppercase tracking-wider"
              >
                {t('login.password')}
              </label>
              <button
                type="button"
                onClick={() => setShowPassword(!showPassword)}
                className="text-[11px] text-zinc-400 hover:text-[#2563EB] dark:hover:text-[#60A5FA] flex items-center gap-1 transition-colors"
              >
                {showPassword ? <EyeOff className="w-3 h-3" /> : <Eye className="w-3 h-3" />}
                <span>{showPassword ? t('login.hide') : t('login.show')}</span>
              </button>
            </div>
            <input
              id="login-password"
              type={showPassword ? 'text' : 'password'}
              required
              value={form.password}
              onChange={(e) => setForm({ ...form, password: e.target.value })}
              placeholder={t('login.passwordPlaceholder')}
              className="hero-input px-3.5 py-2.5 text-sm sm:text-xs text-zinc-900 dark:text-white placeholder:text-zinc-400"
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
                <span>{t('login.submitting')}</span>
              </>
            ) : (
              <span>{t('login.submit')}</span>
            )}
          </button>
        </form>
      </motion.div>
    </div>
  )
}
