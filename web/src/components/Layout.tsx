import { useState, useEffect } from 'react'
import { Outlet, NavLink, useNavigate } from 'react-router-dom'
import { MessageSquare, Bot, Cpu, BookOpen, Plug, GitBranch, LayoutDashboard, Settings, LogOut, Store, Moon, Sun, Menu, X, Bell, ListTodo, CheckCircle2, XCircle, Info, AlertTriangle, Radar, Zap, Film, FolderOpen, CreditCard, FileText, Brain, Activity, Webhook, Code2, Shield, Target, FlaskConical, MessageCircle, Swords } from 'lucide-react'
import { notificationAPI, versionAPI, systemAPI, authRequestAPI } from '../lib/api'
import { starclawWS } from '../lib/websocket'

const CrawfishIcon = ({ className }: { className?: string }) => (
  <svg className={className} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
    <path d="M12 19c-2 0-4-1-5-3s-1-4 0-6c1-1.5 3-3 5-3s4 1.5 5 3c1 2 1 4 0 6s-3 3-5 3z" />
    <path d="M9 10c-2-2-4-3-6-2" />
    <path d="M15 10c2-2 4-3 6-2" />
    <path d="M8 7c-1-2-1-4 0-5" />
    <path d="M16 7c1-2 1-4 0-5" />
    <circle cx="10" cy="11" r="0.8" fill="currentColor" />
    <circle cx="14" cy="11" r="0.8" fill="currentColor" />
    <path d="M10 16c0.5 0.5 1.5 1 2 1s1.5-0.5 2-1" />
    <path d="M9 19l-1 3" />
    <path d="M15 19l1 3" />
    <path d="M11 19.5l-0.5 2.5" />
    <path d="M13 19.5l0.5 2.5" />
  </svg>
)
import HPBar from './HPBar'
import { useAuthStore } from '../stores/authStore'
import { useThemeStore } from '../stores/themeStore'
import { useConfigStore } from '../stores/configStore'
import { useI18n } from '../lib/i18n'

interface NavItem { to: string; icon: React.ComponentType<{ className?: string }>; label: string }
interface NavGroup { group: string; items: NavItem[] }

function getNavGroups(isHosted: boolean, t: (key: string) => string): NavGroup[] {
  const systemItems: NavItem[] = [
    { to: '/wallet', icon: Zap, label: t('nav.wallet') },
    { to: '/mining', icon: Cpu, label: t('nav.mining') },
    { to: '/settings', icon: Settings, label: t('nav.settings') },
  ]
  if (isHosted) {
    systemItems.unshift({ to: '/billing', icon: CreditCard, label: t('nav.billing') })
  }
  return [
    {
      group: '',
      items: [
        { to: '/dashboard', icon: LayoutDashboard, label: t('nav.dashboard') },
        { to: '/chat', icon: MessageSquare, label: t('nav.chat') },
      ],
    },
    {
      group: t('nav.group.swarm'),
      items: [
        { to: '/agents', icon: Bot, label: t('nav.agents') },
        { to: '/marketplace', icon: Store, label: t('nav.marketplace') },
        { to: '/squads', icon: Swords, label: t('nav.squads') },
        { to: '/hivemind', icon: Radar, label: t('nav.hivemind') },
      ],
    },
    {
      group: t('nav.group.capability'),
      items: [
        { to: '/models', icon: Cpu, label: t('nav.models') },
        { to: '/knowledge', icon: BookOpen, label: t('nav.knowledge') },
        { to: '/skills', icon: Plug, label: t('nav.skills') },
        { to: '/memories', icon: Brain, label: t('nav.memories') },
      ],
    },
    {
      group: t('nav.group.automation'),
      items: [
        { to: '/workflows', icon: GitBranch, label: t('nav.workflows') },
        { to: '/tasks', icon: ListTodo, label: t('nav.tasks') },
        { to: '/activities', icon: Brain, label: t('nav.activities') },
        { to: '/integrations', icon: MessageCircle, label: t('nav.integrations') },
      ],
    },
    {
      group: t('nav.group.creation'),
      items: [
        { to: '/videos', icon: Film, label: t('nav.videos') },
        { to: '/resources', icon: FolderOpen, label: t('nav.resources') },
      ],
    },
    {
      group: t('nav.group.ops'),
      items: [
        { to: '/observe', icon: Activity, label: t('nav.observe') },
        { to: '/webhooks', icon: Webhook, label: t('nav.webhooks') },
        { to: '/developer', icon: Code2, label: t('nav.developer') },
        { to: '/security', icon: Shield, label: t('nav.security') },
      ],
    },
    {
      group: t('nav.group.intelligence'),
      items: [
        { to: '/goals', icon: Target, label: t('nav.goals') },
        { to: '/finetune', icon: FlaskConical, label: t('nav.finetune') },
      ],
    },
    {
      group: t('nav.group.system'),
      items: systemItems,
    },
  ]
}

export default function Layout() {
  const { user, logout } = useAuthStore()
  const { dark, toggle: toggleTheme } = useThemeStore()
  const { deployMode, loaded: configLoaded, fetchConfig } = useConfigStore()
  const { locale, setLocale, t } = useI18n()
  const navigate = useNavigate()
  const [mobileOpen, setMobileOpen] = useState(false)
  const [unreadCount, setUnreadCount] = useState(0)
  const [showNotif, setShowNotif] = useState(false)
  const [notifications, setNotifications] = useState<any[]>([])
  const [updateInfo, setUpdateInfo] = useState<{ latest: string; latest_url: string } | null>(null)
  const [updateDismissed, setUpdateDismissed] = useState(false)

  useEffect(() => { if (!configLoaded) fetchConfig() }, [configLoaded, fetchConfig])

  // Molt: check for version updates
  useEffect(() => {
    versionAPI.check().then(res => {
      if (res.data.update_available) {
        setUpdateInfo({ latest: res.data.latest, latest_url: res.data.latest_url })
      }
    }).catch(() => {})
  }, [])

  // WebSocket: connect for real-time push
  useEffect(() => {
    starclawWS.connect()
    const unsubNotif = starclawWS.on('notification', () => {
      setUnreadCount(prev => prev + 1)
    })
    return () => {
      unsubNotif()
      starclawWS.disconnect()
    }
  }, [])

  const navGroups = getNavGroups(deployMode === 'hosted', t)

  useEffect(() => {
    const poll = async () => {
      try {
        const res = await notificationAPI.unreadCount()
        setUnreadCount(res.data.unread_count || 0)
      } catch {}
    }
    poll()
    const interval = setInterval(poll, 15000)
    return () => clearInterval(interval)
  }, [])

  const openNotifications = async () => {
    setShowNotif(!showNotif)
    if (!showNotif) {
      try {
        const res = await notificationAPI.list()
        setNotifications(res.data.notifications || [])
      } catch {}
    }
  }

  const markAllRead = async () => {
    try {
      await notificationAPI.markRead()
      setUnreadCount(0)
      setNotifications(prev => prev.map(n => ({ ...n, is_read: true })))
    } catch {}
  }

  const notifIcon = (type: string) => {
    switch (type) {
      case 'task_complete': return <CheckCircle2 className="w-4 h-4 text-green-500" />
      case 'task_failed': return <XCircle className="w-4 h-4 text-red-500" />
      case 'warning': return <AlertTriangle className="w-4 h-4 text-yellow-500" />
      default: return <Info className="w-4 h-4 text-blue-500" />
    }
  }

  // Auth Request approval (MetaMask-style)
  const [authRequests, setAuthRequests] = useState<any[]>([])
  const [authApproving, setAuthApproving] = useState(false)

  useEffect(() => {
    const pollAuth = async () => {
      try {
        const res = await authRequestAPI.list()
        setAuthRequests(res.data.requests || [])
      } catch {}
    }
    pollAuth()
    const interval = setInterval(pollAuth, 3000)
    return () => clearInterval(interval)
  }, [])

  const handleAuthApprove = async (id: string) => {
    setAuthApproving(true)
    try {
      await authRequestAPI.approve(id)
      setAuthRequests(prev => prev.filter(r => r.id !== id))
    } catch {}
    setAuthApproving(false)
  }

  const handleAuthReject = async (id: string) => {
    try {
      await authRequestAPI.reject(id)
      setAuthRequests(prev => prev.filter(r => r.id !== id))
    } catch {}
  }

  const handleLogout = () => {
    logout()
    navigate('/login')
  }

  return (
    <div className="flex h-screen">
      {/* Auth Request Approval Modal */}
      {authRequests.length > 0 && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 backdrop-blur-sm">
          <div className="bg-white dark:bg-gray-800 rounded-2xl shadow-2xl p-6 w-full max-w-sm mx-4 animate-in">
            <div className="text-center mb-4">
              <div className="w-14 h-14 rounded-full bg-violet-100 dark:bg-violet-900/30 flex items-center justify-center mx-auto mb-3">
                <Radar className="w-7 h-7 text-violet-600 dark:text-violet-400" />
              </div>
              <h3 className="text-lg font-bold text-gray-900 dark:text-white">{t('auth.request_title')}</h3>
              <p className="text-sm text-gray-500 dark:text-gray-400 mt-1">{t('auth.request_desc')}</p>
            </div>
            {authRequests.slice(0, 1).map(req => (
              <div key={req.id} className="space-y-3">
                <div className="bg-gray-50 dark:bg-gray-700/50 rounded-xl p-3 space-y-1.5">
                  <div className="flex justify-between text-xs">
                    <span className="text-gray-400">{t('auth.source')}</span>
                    <span className="text-gray-700 dark:text-gray-300 font-mono">{req.origin || '未知'}</span>
                  </div>
                  <div className="flex justify-between text-xs">
                    <span className="text-gray-400">{t('auth.challenge')}</span>
                    <span className="text-gray-700 dark:text-gray-300 font-mono truncate max-w-[180px]">{req.challenge?.slice(0, 16)}...</span>
                  </div>
                  <div className="flex justify-between text-xs">
                    <span className="text-gray-400">{t('common.time')}</span>
                    <span className="text-gray-700 dark:text-gray-300">{new Date(req.created_at * 1000).toLocaleTimeString()}</span>
                  </div>
                </div>
                <div className="flex gap-3">
                  <button
                    onClick={() => handleAuthReject(req.id)}
                    className="flex-1 py-2.5 rounded-xl border border-gray-200 dark:border-gray-600 text-sm font-medium text-gray-600 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-700 transition"
                  >
                    {t('auth.reject')}
                  </button>
                  <button
                    onClick={() => handleAuthApprove(req.id)}
                    disabled={authApproving}
                    className="flex-1 py-2.5 rounded-xl bg-violet-600 text-white text-sm font-semibold hover:bg-violet-700 transition disabled:opacity-50"
                  >
                    {authApproving ? t('auth.signing') : t('auth.approve')}
                  </button>
                </div>
                <p className="text-[11px] text-gray-400 text-center">{t('auth.approve_hint')}</p>
              </div>
            ))}
          </div>
        </div>
      )}
      {/* Mobile header */}
      <div className="md:hidden fixed top-0 left-0 right-0 z-30 h-12 bg-gray-900 flex items-center px-4 gap-3">
        <button onClick={() => setMobileOpen(!mobileOpen)} className="text-white">
          {mobileOpen ? <X className="w-5 h-5" /> : <Menu className="w-5 h-5" />}
        </button>
        <CrawfishIcon className="w-6 h-6 text-red-400" />
        <span className="text-white font-bold" translate="no">StarClaw</span>
      </div>

      {/* Overlay */}
      {mobileOpen && (
        <div className="md:hidden fixed inset-0 z-20 bg-black/50" onClick={() => setMobileOpen(false)} />
      )}

      {/* Sidebar */}
      <aside className={`fixed md:static z-20 top-0 md:top-auto left-0 h-full w-60 bg-gray-900 text-white flex flex-col transition-transform md:translate-x-0 ${mobileOpen ? 'translate-x-0' : '-translate-x-full md:translate-x-0'}`}>
        <div className="p-4 border-b border-gray-700">
          <div className="flex items-center gap-2">
            <CrawfishIcon className="w-7 h-7 text-red-400" />
            <span className="text-xl font-bold" translate="no">StarClaw</span>
          </div>
          <p className="text-xs text-gray-400 mt-1">{t('app.subtitle')}</p>
        </div>

        <nav className="flex-1 p-3 space-y-0.5 overflow-y-auto">
          {navGroups.map(({ group, items }, gi) => (
            <div key={group || `g${gi}`}>
              {gi > 0 && <div className="my-2 mx-3 border-t border-gray-700/60" />}
              {group && (
                <div className="px-3 pt-2 pb-1 text-[10px] font-semibold uppercase tracking-wider text-gray-500">
                  {group}
                </div>
              )}
              {items.map(({ to, icon: Icon, label }) => (
                <NavLink
                  key={to}
                  to={to}
                  onClick={() => setMobileOpen(false)}
                  className={({ isActive }) =>
                    `flex items-center gap-3 px-3 py-2 rounded-lg text-sm transition-colors ${
                      isActive
                        ? 'bg-primary-600 text-white'
                        : 'text-gray-300 hover:bg-gray-800 hover:text-white'
                    }`
                  }
                >
                  <Icon className="w-4 h-4" />
                  {label}
                </NavLink>
              ))}
            </div>
          ))}
        </nav>

        <div className="border-t border-gray-700">
          <HPBar />
        </div>
        <div className="p-3 border-t border-gray-700 space-y-2">
          <a
            href="https://starclaw.net/docs"
            target="_blank"
            rel="noopener noreferrer"
            className="flex items-center gap-3 px-3 py-2 rounded-lg text-sm text-gray-400 hover:bg-gray-800 hover:text-white transition-colors"
          >
            <FileText className="w-4 h-4" />
            {t('nav.docs')}
          </a>
          <div className="px-3 py-1">
            <kbd className="text-xs text-gray-500 bg-gray-800 px-1.5 py-0.5 rounded border border-gray-700">Ctrl+K</kbd>
            <span className="text-xs text-gray-500 ml-1.5">{t('common.quick_search')}</span>
          </div>
          <div className="flex items-center justify-between px-3 py-1">
            <span className="text-xs text-gray-400">{t('common.theme')}</span>
            <div className="flex items-center gap-1">
              <button
                onClick={() => setLocale(locale === 'zh' ? 'en' : 'zh')}
                className="px-1.5 py-0.5 rounded text-xs text-gray-400 hover:text-white hover:bg-gray-800 transition-colors font-mono"
                title={locale === 'zh' ? 'Switch to English' : '切换到中文'}
              >
                {locale === 'zh' ? 'EN' : '中'}
              </button>
              <button
                onClick={toggleTheme}
                className="p-1.5 rounded-lg text-gray-400 hover:text-white hover:bg-gray-800 transition-colors"
                title={dark ? '浅色模式' : '深色模式'}
              >
                {dark ? <Sun className="w-4 h-4" /> : <Moon className="w-4 h-4" />}
              </button>
            </div>
          </div>
          <div className="flex items-center justify-between px-3 py-2">
            <div className="flex items-center gap-2">
              <div className="w-7 h-7 rounded-full bg-primary-600 flex items-center justify-center text-xs font-medium">
                {user?.username?.charAt(0).toUpperCase()}
              </div>
              <span className="text-sm text-gray-300 truncate max-w-[120px]">
                {user?.username}
              </span>
            </div>
            <button
              onClick={handleLogout}
              className="text-gray-400 hover:text-white transition-colors"
              title={t('common.logout')}
            >
              <LogOut className="w-4 h-4" />
            </button>
          </div>
        </div>
      </aside>

      {/* Main content */}
      <main className="flex-1 flex flex-col overflow-hidden bg-gray-50 dark:bg-gray-900 pt-12 md:pt-0">
        {/* Notification bar - desktop only */}
        <div className="hidden md:flex items-center justify-end h-10 px-4 flex-shrink-0 relative">
          <button
            onClick={openNotifications}
            className="relative p-2 rounded-full hover:bg-gray-200 dark:hover:bg-gray-700 transition-colors"
          >
            <Bell className="w-5 h-5 text-gray-600 dark:text-gray-300" />
            {unreadCount > 0 && (
              <span className="absolute -top-0.5 -right-0.5 min-w-[18px] h-[18px] flex items-center justify-center text-[10px] font-bold text-white bg-red-500 rounded-full px-1">
                {unreadCount > 99 ? '99+' : unreadCount}
              </span>
            )}
          </button>

          {/* Notification dropdown */}
          {showNotif && (
            <>
              <div className="fixed inset-0 z-10" onClick={() => setShowNotif(false)} />
              <div className="absolute right-4 top-10 w-80 max-h-96 bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-xl shadow-xl z-20 flex flex-col">
                <div className="flex items-center justify-between px-4 py-3 border-b border-gray-100 dark:border-gray-700">
                  <span className="font-semibold text-sm text-gray-900 dark:text-white">{t('notification.title')}</span>
                  {unreadCount > 0 && (
                    <button onClick={markAllRead} className="text-xs text-violet-600 hover:underline">{t('notification.mark_all_read')}</button>
                  )}
                </div>
                <div className="flex-1 overflow-y-auto">
                  {notifications.length === 0 ? (
                    <div className="py-8 text-center text-gray-400 text-sm">{t('notification.none')}</div>
                  ) : (
                    notifications.slice(0, 20).map(n => (
                      <div
                        key={n.id}
                        className={`px-4 py-3 border-b border-gray-50 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-750 ${!n.is_read ? 'bg-violet-50/50 dark:bg-violet-900/10' : ''}`}
                        onClick={() => { if (n.task_id) { navigate(`/tasks`); setShowNotif(false) } }}
                      >
                        <div className="flex items-start gap-2">
                          {notifIcon(n.type)}
                          <div className="flex-1 min-w-0">
                            <p className="text-sm font-medium text-gray-900 dark:text-white truncate">{n.title}</p>
                            {n.content && <p className="text-xs text-gray-500 dark:text-gray-400 mt-0.5 line-clamp-2">{n.content}</p>}
                            <p className="text-[10px] text-gray-400 mt-1">{new Date(n.created_at).toLocaleString('zh-CN')}</p>
                          </div>
                          {!n.is_read && <div className="w-2 h-2 rounded-full bg-violet-500 mt-1.5 flex-none" />}
                        </div>
                      </div>
                    ))
                  )}
                </div>
              </div>
            </>
          )}
        </div>
        <div className="flex-1 overflow-hidden relative">
          {updateInfo && !updateDismissed && (
            <div className="bg-orange-50 border-b border-orange-200 px-4 py-2 flex items-center justify-between text-sm">
              <span className="text-orange-700">
                {t('update.available')} <strong>v{updateInfo.latest}</strong>
              </span>
              <div className="flex items-center gap-3">
                <a href={updateInfo.latest_url} target="_blank" rel="noopener noreferrer" className="text-xs text-orange-600 hover:underline">
                  {t('update.view_details')}
                </a>
                <button
                  onClick={async () => {
                    try {
                      await systemAPI.triggerUpdate()
                      setUpdateDismissed(true)
                    } catch {}
                  }}
                  className="px-2.5 py-1 text-xs bg-orange-500 text-white rounded-md hover:bg-orange-600 font-medium"
                >
                  {t('update.one_click')}
                </button>
                <button onClick={() => setUpdateDismissed(true)} className="text-orange-400 hover:text-orange-600">
                  <X className="w-4 h-4" />
                </button>
              </div>
            </div>
          )}
          <Outlet />
        </div>
      </main>
    </div>
  )
}
