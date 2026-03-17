import { Link, useLocation } from 'react-router-dom'
import { Github, Globe, Menu, X } from 'lucide-react'
import { useState, useRef, useEffect } from 'react'
import { useI18n, LOCALES } from '../i18n'

export function Layout({ children }: { children: React.ReactNode }) {
  const { pathname } = useLocation()
  const { t, locale, setLocale } = useI18n()
  const [open, setOpen] = useState(false)
  const [langOpen, setLangOpen] = useState(false)
  const langRef = useRef<HTMLDivElement>(null)

  const NAV = [
    { label: t('nav.home'), to: '/' },
    { label: t('nav.enterprise'), to: '/enterprise' },
    { label: t('nav.star_ai'), to: '/star-ai' },
    { label: t('nav.pricing'), to: '/pricing' },
    { label: t('nav.download'), to: '/download' },
    { label: t('nav.docs'), to: '/docs' },
  ]

  useEffect(() => {
    function onClick(e: MouseEvent) {
      if (langRef.current && !langRef.current.contains(e.target as Node)) setLangOpen(false)
    }
    document.addEventListener('mousedown', onClick)
    return () => document.removeEventListener('mousedown', onClick)
  }, [])

  const currentFlag = LOCALES.find((l) => l.code === locale)?.flag ?? '🌐'

  return (
    <div className="min-h-screen flex flex-col">
      {/* Navbar */}
      <header className="sticky top-0 z-50 border-b border-white/10 bg-gray-950/80 backdrop-blur-lg">
        <nav className="mx-auto flex max-w-6xl items-center justify-between px-6 py-4">
          <Link to="/" className="text-xl font-bold tracking-tight">
            <span className="text-claw-500">Star</span>Claw
          </Link>

          {/* Desktop nav */}
          <div className="hidden md:flex items-center gap-8">
            {NAV.map((n) => (
              <Link
                key={n.to}
                to={n.to}
                className={`text-sm font-medium transition-colors hover:text-white ${
                  pathname === n.to ? 'text-white' : 'text-gray-400'
                }`}
              >
                {n.label}
              </Link>
            ))}
            <a
              href="https://app.starclaw.me"
              className="text-sm font-medium text-gray-400 hover:text-white transition-colors"
            >
              {t('nav.demo')}
            </a>
            <a
              href="https://github.com/yinhe/starclaw"
              target="_blank"
              rel="noopener noreferrer"
              className="text-gray-400 hover:text-white transition-colors"
            >
              <Github size={20} />
            </a>

            {/* Language Switcher */}
            <div className="relative" ref={langRef}>
              <button
                onClick={() => setLangOpen(!langOpen)}
                className="flex items-center gap-1.5 text-sm text-gray-400 hover:text-white transition-colors"
              >
                <Globe size={16} />
                <span>{currentFlag}</span>
              </button>
              {langOpen && (
                <div className="absolute right-0 top-full mt-2 w-44 rounded-lg border border-white/10 bg-gray-900 py-1 shadow-xl">
                  {LOCALES.map((l) => (
                    <button
                      key={l.code}
                      onClick={() => { setLocale(l.code); setLangOpen(false) }}
                      className={`w-full flex items-center gap-2.5 px-3 py-2 text-sm transition-colors text-left ${
                        locale === l.code ? 'text-claw-400 bg-claw-500/10' : 'text-gray-300 hover:bg-white/5'
                      }`}
                    >
                      <span>{l.flag}</span>
                      {l.label}
                    </button>
                  ))}
                </div>
              )}
            </div>
          </div>

          {/* Mobile toggle */}
          <button
            className="md:hidden text-gray-400"
            onClick={() => setOpen(!open)}
          >
            {open ? <X size={24} /> : <Menu size={24} />}
          </button>
        </nav>

        {/* Mobile menu */}
        {open && (
          <div className="md:hidden border-t border-white/10 px-6 py-4 space-y-3">
            {NAV.map((n) => (
              <Link
                key={n.to}
                to={n.to}
                onClick={() => setOpen(false)}
                className="block text-sm font-medium text-gray-300 hover:text-white"
              >
                {n.label}
              </Link>
            ))}
            <a
              href="https://app.starclaw.me"
              className="block text-sm font-medium text-gray-300 hover:text-white"
            >
              {t('nav.demo')}
            </a>
            {/* Mobile language */}
            <div className="pt-2 border-t border-white/10 flex flex-wrap gap-2">
              {LOCALES.map((l) => (
                <button
                  key={l.code}
                  onClick={() => { setLocale(l.code); setOpen(false) }}
                  className={`px-2 py-1 rounded text-xs ${locale === l.code ? 'bg-claw-500/20 text-claw-400' : 'text-gray-400 hover:text-white'}`}
                >
                  {l.flag} {l.label}
                </button>
              ))}
            </div>
          </div>
        )}
      </header>

      {/* Content */}
      <main className="flex-1">{children}</main>

      {/* Footer */}
      <footer className="border-t border-white/10 py-12">
        <div className="mx-auto max-w-6xl px-6">
          <div className="grid grid-cols-2 md:grid-cols-4 gap-8">
            <div>
              <h3 className="text-sm font-semibold text-white mb-3">{t('footer.product')}</h3>
              <ul className="space-y-2 text-sm text-gray-400">
                <li><Link to="/enterprise" className="hover:text-white transition-colors">{t('nav.enterprise')}</Link></li>
                <li><Link to="/star-ai" className="hover:text-white transition-colors">{t('nav.star_ai')}</Link></li>
                <li><Link to="/pricing" className="hover:text-white transition-colors">{t('nav.pricing')}</Link></li>
                <li><Link to="/download" className="hover:text-white transition-colors">{t('nav.download')}</Link></li>
              </ul>
            </div>
            <div>
              <h3 className="text-sm font-semibold text-white mb-3">{t('footer.community')}</h3>
              <ul className="space-y-2 text-sm text-gray-400">
                <li><a href="https://github.com/yinhe/starclaw" target="_blank" rel="noopener noreferrer" className="hover:text-white transition-colors">GitHub</a></li>
                <li><Link to="/docs" className="hover:text-white transition-colors">{t('nav.docs')}</Link></li>
                <li><Link to="/partners" className="hover:text-white transition-colors">{t('nav.partners')}</Link></li>
              </ul>
            </div>
            <div>
              <h3 className="text-sm font-semibold text-white mb-3">{t('footer.ecosystem')}</h3>
              <ul className="space-y-2 text-sm text-gray-400">
                <li><a href="https://starclaw.net" className="hover:text-white transition-colors">{t('footer.portal')}</a></li>
                <li><a href="https://star-ai.net" className="hover:text-white transition-colors">Star AI</a></li>
                <li><a href="https://app.starclaw.me" className="hover:text-white transition-colors">{t('nav.demo')}</a></li>
              </ul>
            </div>
            <div>
              <h3 className="text-sm font-semibold text-white mb-3">{t('footer.legal')}</h3>
              <ul className="space-y-2 text-sm text-gray-400">
                <li><span className="cursor-default">MIT License</span></li>
                <li><Link to="/about" className="hover:text-white transition-colors">{t('nav.about')}</Link></li>
              </ul>
            </div>
          </div>
          <div className="mt-10 pt-6 border-t border-white/10 text-center text-sm text-gray-500">
            &copy; {new Date().getFullYear()} StarClaw. {t('footer.copyright')}
          </div>
        </div>
      </footer>
    </div>
  )
}
