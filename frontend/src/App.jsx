import React, { useEffect, useState, lazy, Suspense } from 'react'
import { Routes, Route, Navigate } from 'react-router-dom'
import Layout from './components/Layout'
import { getToken } from './api'
import { I18nProvider } from './i18n'

const Dashboard = lazy(() => import('./pages/Dashboard'))
const Subscriptions = lazy(() => import('./pages/Subscriptions'))
const Profiles = lazy(() => import('./pages/Profiles'))
const Settings = lazy(() => import('./pages/Settings'))
const Login = lazy(() => import('./pages/Login'))

function PageFallback() {
  return (
    <div className="max-w-7xl mx-auto space-y-5 animate-pulse pt-2">
      <div className="h-10 w-48 rounded-xl bg-zinc-200/70 dark:bg-white/10" />
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        {[...Array(3)].map((_, i) => (
          <div key={i} className="h-32 rounded-2xl bg-zinc-200/60 dark:bg-white/5" />
        ))}
      </div>
    </div>
  )
}

function ProtectedRoute({ children }) {
  const token = getToken()
  if (!token) {
    return <Navigate to="/login" replace />
  }
  return children
}

export default function App() {
  const [isDark, setIsDark] = useState(() => {
    const saved = localStorage.getItem('sublite_theme')
    if (saved) return saved === 'dark'
    return window.matchMedia('(prefers-color-scheme: dark)').matches
  })

  useEffect(() => {
    if (isDark) {
      document.documentElement.classList.add('dark')
      localStorage.setItem('sublite_theme', 'dark')
    } else {
      document.documentElement.classList.remove('dark')
      localStorage.setItem('sublite_theme', 'light')
    }
  }, [isDark])

  const toggleTheme = () => setIsDark((prev) => !prev)

  return (
    <I18nProvider>
      <Suspense fallback={<PageFallback />}>
        <Routes>
          <Route path="/login" element={<Login isDark={isDark} toggleTheme={toggleTheme} />} />
          <Route
            path="/"
            element={
              <ProtectedRoute>
                <Layout isDark={isDark} toggleTheme={toggleTheme} />
              </ProtectedRoute>
            }
          >
            <Route index element={<Dashboard />} />
            <Route path="subscriptions" element={<Subscriptions />} />
            <Route path="profiles" element={<Profiles />} />
            <Route path="settings" element={<Settings />} />
          </Route>
          <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
      </Suspense>
    </I18nProvider>
  )
}
