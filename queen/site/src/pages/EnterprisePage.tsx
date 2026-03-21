import { Link } from 'react-router-dom'
import { Shield, Users, CreditCard, ScrollText, Bell, Palette, ArrowRight, Server, Code2, Database, Sparkles } from 'lucide-react'
import { Layout } from '../components/Layout'
import { useI18n } from '../i18n'

const FEATURES = [
  { key: 'fleet', icon: Users },
  { key: 'rbac', icon: Shield },
  { key: 'billing', icon: CreditCard },
  { key: 'audit', icon: ScrollText },
  { key: 'webhook', icon: Bell },
  { key: 'whitelabel', icon: Palette },
]

const TEAM_TEMPLATES = [
  { name: 'DevClaw', roles: '5 Agents', color: 'border-blue-500/30 text-blue-400' },
  { name: 'MarketClaw', roles: '4 Agents', color: 'border-pink-500/30 text-pink-400' },
  { name: 'SupportClaw', roles: '4 Agents', color: 'border-green-500/30 text-green-400' },
  { name: 'DataClaw', roles: '3 Agents', color: 'border-cyan-500/30 text-cyan-400' },
  { name: 'QuantClaw', roles: '4 Agents', color: 'border-yellow-500/30 text-yellow-400' },
  { name: 'EcomClaw', roles: '4 Agents', color: 'border-purple-500/30 text-purple-400' },
  { name: 'DramaClaw', roles: '5 Agents', color: 'border-red-500/30 text-red-400' },
  { name: 'SalesClaw', roles: '4 Agents', color: 'border-orange-500/30 text-orange-400' },
  { name: 'OpsClaw', roles: '4 Agents', color: 'border-emerald-500/30 text-emerald-400' },
]

const STACK = [
  { label: 'Go + Gin', desc: 'High-performance API backend', icon: Server },
  { label: 'React + Vite', desc: 'Modern admin console', icon: Code2 },
  { label: 'MySQL + GORM', desc: 'Reliable data persistence', icon: Database },
]

export function EnterprisePage() {
  const { t } = useI18n()

  return (
    <Layout>
      {/* Hero */}
      <section className="relative overflow-hidden">
        <div className="absolute inset-0 bg-gradient-to-b from-claw-950/30 via-transparent to-transparent" />
        <div className="mx-auto max-w-5xl px-6 pt-24 pb-20 text-center relative">
          <div className="inline-flex items-center gap-2 rounded-full border border-claw-500/30 bg-claw-500/10 px-4 py-1.5 text-sm text-claw-400 mb-8">
            <Shield size={14} />
            Overlord
          </div>
          <h1 className="text-4xl md:text-6xl font-bold tracking-tight leading-tight">
            {t('ent.title')}
          </h1>
          <p className="mt-6 text-lg text-gray-400 max-w-2xl mx-auto">
            {t('ent.desc')}
          </p>
          <div className="mt-10 flex flex-col sm:flex-row items-center justify-center gap-4">
            <a
              href="mailto:hello@starclaw.me"
              className="inline-flex items-center gap-2 rounded-lg bg-claw-600 px-8 py-3.5 text-sm font-semibold text-white hover:bg-claw-500 transition-colors"
            >
              {t('ent.hero_cta')}
              <ArrowRight size={16} />
            </a>
            <Link
              to="/pricing"
              className="inline-flex items-center gap-2 rounded-lg border border-white/15 px-8 py-3.5 text-sm font-semibold text-gray-300 hover:bg-white/5 transition-colors"
            >
              {t('nav.pricing')}
            </Link>
          </div>
        </div>
      </section>

      {/* Features */}
      <section className="py-16 md:py-24 border-t border-white/5">
        <div className="mx-auto max-w-6xl px-4 sm:px-6">
          <h2 className="text-2xl sm:text-3xl font-bold tracking-tight text-center mb-12 md:mb-16">{t('ent.features')}</h2>
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
            {FEATURES.map(f => (
              <div key={f.key} className="rounded-xl border border-white/10 bg-white/[0.02] p-6 hover:border-claw-500/30 transition-colors">
                <div className="inline-flex items-center justify-center w-10 h-10 rounded-lg bg-claw-500/10 text-claw-400 mb-4">
                  <f.icon size={20} />
                </div>
                <h3 className="text-lg font-semibold text-white mb-2">{t(`ent.f.${f.key}`)}</h3>
                <p className="text-sm text-gray-400 leading-relaxed">{t(`ent.f.${f.key}.desc`)}</p>
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* Team Agent Templates */}
      <section className="py-16 md:py-24 border-t border-white/5 bg-gradient-to-b from-transparent via-claw-950/20 to-transparent">
        <div className="mx-auto max-w-6xl px-4 sm:px-6">
          <div className="text-center mb-12">
            <div className="inline-flex items-center gap-2 rounded-full border border-orange-500/30 bg-orange-500/10 px-4 py-1.5 text-sm text-orange-400 mb-6">
              <Sparkles size={14} />
              {t('team.badge')}
            </div>
            <h2 className="text-2xl sm:text-3xl font-bold tracking-tight">{t('team.title')}</h2>
            <p className="mt-4 text-gray-400 text-lg max-w-2xl mx-auto">{t('team.desc')}</p>
          </div>
          <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-5 gap-3 mb-8">
            {TEAM_TEMPLATES.map(tmpl => (
              <div key={tmpl.name} className={`rounded-lg border ${tmpl.color} bg-white/[0.02] p-4 text-center hover:scale-105 transition-transform`}>
                <div className="font-semibold text-sm mb-1">{tmpl.name}</div>
                <div className="text-xs text-gray-500">{tmpl.roles}</div>
              </div>
            ))}
          </div>
          <div className="rounded-xl border border-white/10 bg-white/[0.02] p-4 text-center text-sm text-gray-400 mb-8">
            {t('team.how')}
          </div>
          <div className="text-center">
            <a
              href="https://overlord.starclaw.net"
              target="_blank"
              rel="noopener noreferrer"
              className="inline-flex items-center gap-2 rounded-lg bg-orange-600 px-6 py-3 text-sm font-semibold text-white hover:bg-orange-500 transition-colors"
            >
              <Users size={18} />
              {t('team.cta')}
              <ArrowRight size={16} />
            </a>
          </div>
        </div>
      </section>

      {/* Tech Stack */}
      <section className="py-16 md:py-24 border-t border-white/5">
        <div className="mx-auto max-w-4xl px-4 sm:px-6">
          <h2 className="text-2xl sm:text-3xl font-bold tracking-tight text-center mb-12">{t('ent.stack')}</h2>
          <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
            {STACK.map(s => (
              <div key={s.label} className="rounded-xl border border-white/10 bg-white/[0.02] p-6 text-center">
                <div className="inline-flex items-center justify-center w-12 h-12 rounded-lg bg-claw-500/10 text-claw-400 mb-4">
                  <s.icon size={24} />
                </div>
                <h3 className="font-semibold text-white mb-1">{s.label}</h3>
                <p className="text-sm text-gray-400">{s.desc}</p>
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* CTA */}
      <section className="py-16 md:py-24 border-t border-white/5">
        <div className="mx-auto max-w-3xl px-4 sm:px-6 text-center">
          <h2 className="text-2xl sm:text-3xl font-bold tracking-tight">{t('ent.cta')}</h2>
          <p className="mt-4 text-gray-400 text-lg">{t('ent.cta.desc')}</p>
          <div className="mt-8">
            <a
              href="mailto:hello@starclaw.me"
              className="inline-flex items-center gap-2 rounded-lg bg-claw-600 px-8 py-3.5 text-sm font-semibold text-white hover:bg-claw-500 transition-colors"
            >
              {t('ent.hero_cta')}
              <ArrowRight size={16} />
            </a>
          </div>
        </div>
      </section>
    </Layout>
  )
}
