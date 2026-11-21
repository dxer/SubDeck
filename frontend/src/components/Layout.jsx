import React, { useState, useMemo } from 'react'
import { Outlet, NavLink, useLocation, useNavigate } from 'react-router-dom'
import { motion, AnimatePresence } from 'framer-motion'
import {
  LayoutDashboard,
  Layers,
  Rss,
  Settings as SettingsIcon,
  Sun,
  Moon,
  LogOut,
  ChevronRight,
  PanelLeftClose,
  PanelLeftOpen,
  Languages,
  Menu,
  X
} from 'lucide-react'
import { setToken, setUser, getUser } from '../api'
import { useI18n } from '../i18n'
import SubDeckMark from './SubDeckLogo'

export default function Layout({ isDark, toggleTheme }) {
  const { t, toggleLang } = useI18n()
  const [isCollapsed, setIsCollapsed] = useState(() => {
    return localStorage.getItem('sublite_sidebar_collapsed') === 'true'
  })
  const [mobileDrawerOpen, setMobileDrawerOpen] = useState(false)
  const location = useLocation()
  const navigate = useNavigate()
  const username = getUser() || 'admin'

  const navGroups = useMemo(
    () => [
      {
        title: t('nav.groupOverview'),
        items: [{ path: '/', label: t('nav.dashboard'), icon: LayoutDashboard }]
      },
      {
        title: t('nav.groupCore'),
        items: [
          { path: '/profiles', label: t('nav.profiles'), icon: Layers },
          { path: '/subscriptions', label: t('nav.subscriptions'), icon: Rss }
        ]
      },
      {
        title: t('nav.groupSystem'),
        items: [{ path: '/settings', label: t('nav.settings'), icon: SettingsIcon }]
      }
    ],
    [t]
  )

  const flatNavItems = useMemo(
    () => [
      { path: '/', label: t('nav.dashboard'), icon: LayoutDashboard },
      { path: '/profiles', label: t('nav.profiles'), icon: Layers },
      { path: '/subscriptions', label: t('nav.subscriptions'), icon: Rss },
      { path: '/settings', label: t('nav.settings'), icon: SettingsIcon }
    ],
    [t]
  )

  const toggleSidebar = () => {
    setIsCollapsed((prev) => {
      const next = !prev
      localStorage.setItem('sublite_sidebar_collapsed', next ? 'true' : 'false')
      return next
    })
  }

  const handleLogout = () => {
    setToken('')
    setUser('')
    navigate('/login')
  }

  const currentRouteName = useMemo(() => {
    const match = flatNavItems.find((item) => item.path === location.pathname)
    return match ? match.label : t('nav.console')
  }, [flatNavItems, location.pathname, t])

  return (
    <div className="flex h-[100dvh] overflow-hidden bg-[#F8F9FB] dark:bg-[#090B0F] text-zinc-900 dark:text-zinc-100 font-sans transition-colors duration-150">
      {/* Desktop Animated Collapsible Sidebar */}
      <motion.aside
        animate={{ width: isCollapsed ? 76 : 240 }}
        transition={{ duration: 0.2, ease: [0.16, 1, 0.3, 1] }}
        className="hidden md:flex flex-col bg-white dark:bg-[#11141B] border-r border-zinc-200/80 dark:border-white/[0.08] shrink-0 relative z-30"
      >
        {/* Brand Header */}
        <div
          className={`flex items-center h-14 border-b border-zinc-100 dark:border-white/[0.06] relative ${
            isCollapsed ? 'justify-center px-2' : 'justify-between px-4'
          }`}
        >
          <div
            onClick={isCollapsed ? toggleSidebar : undefined}
            className={`flex items-center gap-2.5 ${isCollapsed ? 'cursor-pointer' : 'overflow-hidden'}`}
            title={isCollapsed ? t('nav.expandSidebar') : ''}
          >
            <SubDeckMark className="w-8 h-8 shrink-0 transition-transform hover:scale-105 shadow-xs rounded-xl" />
            {!isCollapsed && (
              <motion.span
                initial={{ opacity: 0, x: -6 }}
                animate={{ opacity: 1, x: 0 }}
                exit={{ opacity: 0 }}
                className="font-extrabold text-[15px] tracking-tight text-zinc-900 dark:text-white truncate"
              >
                SubDeck
              </motion.span>
            )}
          </div>

          {isCollapsed ? (
            <button
              onClick={toggleSidebar}
              title={t('nav.expandSidebar')}
              className="absolute -right-3 top-4 w-6 h-6 rounded-full bg-white dark:bg-[#181C26] border border-zinc-200 dark:border-white/15 shadow-sm text-zinc-400 hover:text-[#2563EB] dark:hover:text-[#60A5FA] flex items-center justify-center transition-all z-40 active:scale-90 hover:scale-110"
            >
              <PanelLeftOpen className="w-3.5 h-3.5" />
            </button>
          ) : (
            <button
              onClick={toggleSidebar}
              title={t('nav.collapseSidebar')}
              className="p-1.5 rounded-lg text-zinc-400 hover:text-zinc-700 dark:hover:text-white hover:bg-zinc-100 dark:hover:bg-white/5 transition-all active:scale-95 shrink-0"
            >
              <PanelLeftClose className="w-4 h-4" />
            </button>
          )}
        </div>

        {/* Navigation Groups */}
        <nav className="flex-1 px-3 py-4 space-y-5 overflow-y-auto">
          {navGroups.map((group) => (
            <div key={group.title} className="space-y-1">
              {!isCollapsed ? (
                <div className="px-3 text-[10.5px] font-bold text-zinc-400 dark:text-zinc-500 uppercase tracking-wider mb-1.5">
                  {group.title}
                </div>
              ) : (
                <div className="h-1.5" />
              )}

              {group.items.map((item) => {
                const isActive = location.pathname === item.path
                return (
                  <NavLink
                    key={item.path}
                    to={item.path}
                    title={isCollapsed ? item.label : ''}
                    className={`relative flex items-center py-2.5 rounded-xl text-xs transition-colors duration-150 group active:scale-[0.98] ${
                      isCollapsed ? 'justify-center px-2' : 'gap-3 px-3'
                    } ${
                      isActive
                        ? 'text-[#2563EB] dark:text-white font-semibold'
                        : 'text-zinc-600 dark:text-zinc-400 hover:text-zinc-900 dark:hover:text-white hover:bg-zinc-100/80 dark:hover:bg-white/[0.04] font-medium'
                    }`}
                  >
                    {isActive && (
                      <motion.span
                        layoutId="sidebar-active-pill"
                        transition={{ type: 'spring', stiffness: 380, damping: 32 }}
                        className="absolute inset-0 rounded-xl bg-[#2563EB]/[0.08] dark:bg-[#2563EB]/20 border border-[#2563EB]/15 dark:border-[#3B82F6]/30"
                      />
                    )}
                    <item.icon
                      className={`relative z-10 w-4 h-4 shrink-0 transition-transform group-hover:scale-105 ${
                        isActive ? 'text-[#2563EB] dark:text-[#60A5FA]' : 'text-zinc-400 dark:text-zinc-500'
                      }`}
                    />
                    {!isCollapsed && <span className="relative z-10 truncate">{item.label}</span>}
                  </NavLink>
                )
              })}
            </div>
          ))}
        </nav>

        {/* Footer: User & Sign Out */}
        <div className="p-3 border-t border-zinc-100 dark:border-white/[0.06]">
          <div
            className={`flex items-center p-2 rounded-xl bg-zinc-50 dark:bg-white/[0.03] border border-zinc-200/60 dark:border-white/[0.06] ${
              isCollapsed ? 'justify-center' : 'justify-between'
            }`}
          >
            <div className="flex items-center gap-2.5 min-w-0">
              <div
                title={isCollapsed ? `${username} · ${t('nav.logout')}` : ''}
                onClick={isCollapsed ? handleLogout : undefined}
                className={`w-8 h-8 rounded-lg bg-[#0F172A] dark:bg-[#2563EB] text-white flex items-center justify-center font-bold text-xs shadow-xs shrink-0 ${
                  isCollapsed ? 'cursor-pointer hover:opacity-85 active:scale-95' : ''
                }`}
              >
                {username.slice(0, 1).toUpperCase()}
              </div>
              {!isCollapsed && (
                <span className="text-xs font-bold text-zinc-800 dark:text-zinc-200 truncate">{username}</span>
              )}
            </div>
            {!isCollapsed && (
              <button
                onClick={handleLogout}
                title={t('nav.logout')}
                className="text-zinc-400 hover:text-rose-600 hover:bg-rose-500/10 p-1.5 rounded-lg transition-all active:scale-95"
              >
                <LogOut className="w-4 h-4" />
              </button>
            )}
          </div>
        </div>
      </motion.aside>

      {/* Main Content Area */}
      <div className="flex-1 flex flex-col min-w-0 overflow-hidden relative">
        {/* Desktop Header */}
        <header className="hidden md:flex items-center justify-between px-6 lg:px-8 h-14 border-b border-zinc-200/70 dark:border-white/[0.07] bg-white/80 dark:bg-[#11141B]/80 backdrop-blur-md sticky top-0 z-20 shrink-0">
          <div className="flex items-center gap-2 text-xs font-medium">
            <span className="text-zinc-400">SubDeck</span>
            <ChevronRight className="w-3.5 h-3.5 text-zinc-300 dark:text-zinc-600" />
            <span className="text-zinc-900 dark:text-white font-semibold">{currentRouteName}</span>
          </div>

          <div className="flex items-center gap-2">
            <button
              onClick={toggleLang}
              title={t('common.switchLang')}
              className="inline-flex items-center gap-1.5 px-2.5 py-1.5 rounded-lg text-xs font-semibold text-zinc-600 dark:text-zinc-300 hover:bg-zinc-100 dark:hover:bg-white/5 border border-zinc-200/70 dark:border-white/10 transition-all active:scale-95"
            >
              <Languages className="w-3.5 h-3.5 text-zinc-400" />
              <span className="font-mono">{t('common.langLabel')}</span>
            </button>

            <button
              onClick={toggleTheme}
              title={isDark ? t('common.lightTheme') : t('common.darkTheme')}
              className="p-2 rounded-lg text-zinc-500 dark:text-zinc-400 hover:bg-zinc-100 dark:hover:bg-white/5 border border-zinc-200/70 dark:border-white/10 transition-all active:scale-95"
            >
              {isDark ? <Sun className="w-4 h-4 text-amber-400" /> : <Moon className="w-4 h-4 text-[#2563EB]" />}
            </button>
          </div>
        </header>

        {/* Mobile Header */}
        <header className="flex md:hidden items-center justify-between px-3.5 h-14 bg-white/95 dark:bg-[#11141B]/95 backdrop-blur-md border-b border-zinc-200/80 dark:border-white/[0.08] sticky top-0 z-20 shrink-0">
          <div className="flex items-center gap-2.5 min-w-0">
            <button
              onClick={() => setMobileDrawerOpen(true)}
              aria-label="Menu"
              className="p-2 -ml-1 rounded-xl text-zinc-600 dark:text-zinc-300 hover:bg-zinc-100 dark:hover:bg-white/5 active:scale-95"
            >
              <Menu className="w-5 h-5" />
            </button>
            <div className="flex items-center gap-2 min-w-0">
              <SubDeckMark className="w-7 h-7 rounded-lg shadow-xs shrink-0" />
              <span className="font-bold text-sm tracking-tight text-zinc-900 dark:text-white truncate">
                {currentRouteName}
              </span>
            </div>
          </div>

          <div className="flex items-center gap-1.5 shrink-0">
            <button
              onClick={toggleLang}
              title={t('common.switchLang')}
              className="inline-flex items-center gap-1 px-2.5 py-1.5 rounded-lg text-xs font-mono font-semibold text-zinc-600 dark:text-zinc-300 hover:bg-zinc-100 dark:hover:bg-white/5 border border-zinc-200/70 dark:border-white/10"
            >
              <Languages className="w-3.5 h-3.5 text-zinc-400" />
              <span>{t('common.langLabel')}</span>
            </button>

            <button
              onClick={toggleTheme}
              title={isDark ? t('common.lightTheme') : t('common.darkTheme')}
              className="p-2 rounded-xl text-zinc-500 hover:bg-zinc-100 dark:hover:bg-white/5"
            >
              {isDark ? <Sun className="w-4 h-4 text-amber-400" /> : <Moon className="w-4 h-4 text-[#2563EB]" />}
            </button>
          </div>
        </header>

        {/* Mobile Slide-out Menu Drawer */}
        <AnimatePresence>
          {mobileDrawerOpen && (
            <div className="fixed inset-0 z-50 flex md:hidden">
              <motion.div
                initial={{ opacity: 0 }}
                animate={{ opacity: 1 }}
                exit={{ opacity: 0 }}
                onClick={() => setMobileDrawerOpen(false)}
                className="fixed inset-0 bg-black/55 backdrop-blur-sm"
              />
              <motion.div
                initial={{ x: -280 }}
                animate={{ x: 0 }}
                exit={{ x: -280 }}
                transition={{ type: 'spring', damping: 28, stiffness: 320 }}
                className="relative w-72 bg-white dark:bg-[#11141B] h-full shadow-2xl flex flex-col z-10 border-r border-zinc-200/80 dark:border-white/10"
              >
                <div className="flex items-center justify-between px-4 h-14 border-b border-zinc-100 dark:border-white/[0.06]">
                  <div className="flex items-center gap-2.5">
                    <SubDeckMark className="w-7 h-7 rounded-lg shadow-xs" />
                    <span className="font-bold text-sm text-zinc-900 dark:text-white">SubDeck</span>
                  </div>
                  <button
                    onClick={() => setMobileDrawerOpen(false)}
                    className="p-1.5 text-zinc-400 hover:text-zinc-600 dark:hover:text-white"
                  >
                    <X className="w-5 h-5" />
                  </button>
                </div>

                <nav className="flex-1 p-3 space-y-4 overflow-y-auto">
                  {navGroups.map((group) => (
                    <div key={group.title} className="space-y-1">
                      <div className="px-3 text-[11px] font-bold text-zinc-400 uppercase tracking-wider mb-1">
                        {group.title}
                      </div>
                      {group.items.map((item) => {
                        const isActive = location.pathname === item.path
                        return (
                          <NavLink
                            key={item.path}
                            to={item.path}
                            onClick={() => setMobileDrawerOpen(false)}
                            className={`flex items-center gap-3 px-3 py-2.5 rounded-xl text-xs font-semibold ${
                              isActive
                                ? 'bg-[#2563EB]/10 text-[#2563EB] dark:bg-[#2563EB]/20 dark:text-white'
                                : 'text-zinc-600 dark:text-zinc-400 hover:bg-zinc-100 dark:hover:bg-white/5'
                            }`}
                          >
                            <item.icon className="w-4 h-4" />
                            <span>{item.label}</span>
                          </NavLink>
                        )
                      })}
                    </div>
                  ))}
                </nav>

                <div className="p-3 pb-[calc(0.75rem+env(safe-area-inset-bottom))] border-t border-zinc-100 dark:border-white/[0.06] space-y-2">
                  <div className="flex items-center gap-2.5 px-2 py-1 text-xs text-zinc-500">
                    <div className="w-7 h-7 rounded-lg bg-[#0F172A] dark:bg-[#2563EB] text-white flex items-center justify-center font-bold text-[11px]">
                      {username.slice(0, 1).toUpperCase()}
                    </div>
                    <span className="font-semibold text-zinc-800 dark:text-zinc-200 truncate">{username}</span>
                  </div>
                  <button
                    onClick={handleLogout}
                    className="w-full flex items-center justify-center gap-2 p-2.5 rounded-xl text-xs font-semibold text-rose-600 bg-rose-500/10 hover:bg-rose-500/15"
                  >
                    <LogOut className="w-4 h-4" />
                    <span>{t('nav.logout')}</span>
                  </button>
                </div>
              </motion.div>
            </div>
          )}
        </AnimatePresence>

        {/* Scrollable Main Content */}
        <main className="flex-1 overflow-y-auto overflow-x-hidden p-3.5 sm:p-6 lg:p-8 pb-20 md:pb-8">
          <Outlet />
        </main>

        {/* Mobile Bottom Tab Bar */}
        <nav className="md:hidden fixed bottom-0 inset-x-0 z-20 bg-white/95 dark:bg-[#11141B]/95 backdrop-blur-md border-t border-zinc-200/80 dark:border-white/[0.08] px-2 pt-1.5 pb-[calc(0.375rem+env(safe-area-inset-bottom))] grid grid-cols-4 gap-1">
          {flatNavItems.map((item) => {
            const isActive = location.pathname === item.path
            return (
              <NavLink
                key={item.path}
                to={item.path}
                className={`flex flex-col items-center justify-center gap-0.5 py-1.5 rounded-xl text-[10.5px] font-semibold transition-colors ${
                  isActive
                    ? 'text-[#2563EB] dark:text-[#60A5FA] bg-[#2563EB]/[0.07] dark:bg-[#2563EB]/15'
                    : 'text-zinc-500 dark:text-zinc-400'
                }`}
              >
                <item.icon className="w-4 h-4" />
                <span className="truncate max-w-full px-1">{item.label}</span>
              </NavLink>
            )
          })}
        </nav>
      </div>
    </div>
  )
}
