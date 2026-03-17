import { Link } from 'react-router-dom'
import { Shield, Users, CreditCard, ScrollText, Bell, Palette, ArrowRight, Server, Code2, Database } from 'lucide-react'
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
          <div className="mt-10 flex items-center justify-center gap-4">
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
      <section className="py-24 border-t border-white/5">
        <div className="mx-auto max-w-6xl px-6">
          <h2 className="text-3xl font-bold tracking-tight text-center mb-16">{t('ent.features')}</h2>
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

      {/* Tech Stack */}
      <section className="py-24 border-t border-white/5">
        <div className="mx-auto max-w-4xl px-6">
          <h2 className="text-3xl font-bold tracking-tight text-center mb-12">{t('ent.stack')}</h2>
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
      <section className="py-24 border-t border-white/5">
        <div className="mx-auto max-w-3xl px-6 text-center">
          <h2 className="text-3xl font-bold tracking-tight">{t('ent.cta')}</h2>
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
