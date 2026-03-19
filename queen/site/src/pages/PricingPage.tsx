import { Link } from 'react-router-dom'
import { Check, ArrowRight, ChevronDown, ChevronUp } from 'lucide-react'
import { Layout } from '../components/Layout'
import { useI18n } from '../i18n'
import { useState } from 'react'

const PLANS = [
  { key: 'free', popular: false, cta: 'pricing.cta', link: '/download' },
  { key: 'starter', popular: false, cta: 'pricing.cta', link: '/download' },
  { key: 'pro', popular: true, cta: 'pricing.cta', link: '/download' },
  { key: 'enterprise', popular: false, cta: 'pricing.contact', link: '/about' },
  { key: 'unlimited', popular: false, cta: 'pricing.contact', link: '/about' },
]

const FAQ_KEYS = ['q1', 'q2', 'q3', 'q4']

export function PricingPage() {
  const { t } = useI18n()
  const [openFaq, setOpenFaq] = useState<string | null>(null)

  return (
    <Layout>
      <section className="py-16 md:py-24">
        <div className="mx-auto max-w-6xl px-4 sm:px-6">
          <div className="text-center mb-16">
            <h1 className="text-4xl md:text-5xl font-bold tracking-tight">{t('pricing.title')}</h1>
            <p className="mt-4 text-lg text-gray-400 max-w-2xl mx-auto">{t('pricing.desc')}</p>
          </div>

          {/* Plans grid */}
          <div className="grid grid-cols-1 md:grid-cols-3 lg:grid-cols-5 gap-4">
            {PLANS.map(plan => (
              <div
                key={plan.key}
                className={`relative rounded-xl border p-6 flex flex-col ${
                  plan.popular
                    ? 'border-claw-500/50 bg-claw-500/5 ring-1 ring-claw-500/20'
                    : 'border-white/10 bg-white/[0.02]'
                }`}
              >
                {plan.popular && (
                  <div className="absolute -top-3 left-1/2 -translate-x-1/2 rounded-full bg-claw-600 px-3 py-0.5 text-xs font-medium text-white">
                    {t('pricing.popular')}
                  </div>
                )}
                <h3 className="text-lg font-semibold text-white">{t(`pricing.${plan.key}`)}</h3>
                <div className="mt-3">
                  <span className="text-3xl font-bold text-white">{t(`pricing.${plan.key}.price`)}</span>
                  {plan.key !== 'free' && plan.key !== 'unlimited' && (
                    <span className="text-sm text-gray-400">{t('pricing.mo')}</span>
                  )}
                </div>
                <p className="mt-2 text-sm text-gray-400">{t(`pricing.${plan.key}.desc`)}</p>
                <ul className="mt-5 space-y-2.5 flex-1">
                  {[1, 2, 3, 4].map(i => (
                    <li key={i} className="flex items-start gap-2 text-sm text-gray-300">
                      <Check size={14} className="text-claw-400 mt-0.5 shrink-0" />
                      {t(`pricing.${plan.key}.f${i}`)}
                    </li>
                  ))}
                </ul>
                <Link
                  to={plan.link}
                  className={`mt-6 block text-center py-2.5 rounded-lg text-sm font-medium transition-colors ${
                    plan.popular
                      ? 'bg-claw-600 text-white hover:bg-claw-500'
                      : 'border border-white/15 text-gray-300 hover:bg-white/5'
                  }`}
                >
                  {t(plan.cta)}
                </Link>
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* FAQ */}
      <section className="py-16 md:py-24 border-t border-white/5">
        <div className="mx-auto max-w-3xl px-4 sm:px-6">
          <h2 className="text-2xl sm:text-3xl font-bold tracking-tight text-center mb-12">{t('pricing.faq')}</h2>
          <div className="space-y-3">
            {FAQ_KEYS.map(k => (
              <div key={k} className="rounded-xl border border-white/10 bg-white/[0.02]">
                <button
                  onClick={() => setOpenFaq(openFaq === k ? null : k)}
                  className="w-full flex items-center justify-between px-6 py-4 text-left"
                >
                  <span className="font-medium text-white">{t(`pricing.faq.${k}`)}</span>
                  {openFaq === k ? (
                    <ChevronUp size={18} className="text-gray-400" />
                  ) : (
                    <ChevronDown size={18} className="text-gray-400" />
                  )}
                </button>
                {openFaq === k && (
                  <div className="px-6 pb-4 text-sm text-gray-400 leading-relaxed">
                    {t(`pricing.faq.a${k.replace('q', '')}`)}
                  </div>
                )}
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* CTA */}
      <section className="py-16 md:py-24 border-t border-white/5">
        <div className="mx-auto max-w-3xl px-4 sm:px-6 text-center">
          <h2 className="text-2xl sm:text-3xl font-bold tracking-tight">{t('cta.title')}</h2>
          <p className="mt-4 text-gray-400 text-lg">{t('cta.desc')}</p>
          <div className="mt-8 flex flex-col sm:flex-row items-center justify-center gap-4">
            <Link
              to="/download"
              className="inline-flex items-center gap-2 rounded-lg bg-claw-600 px-8 py-3 text-sm font-semibold text-white hover:bg-claw-500 transition-colors"
            >
              {t('pricing.cta')}
            </Link>
            <Link
              to="/about"
              className="inline-flex items-center gap-2 rounded-lg border border-white/15 px-8 py-3 text-sm font-semibold text-gray-300 hover:bg-white/5 transition-colors"
            >
              {t('pricing.contact')}
              <ArrowRight size={16} />
            </Link>
          </div>
        </div>
      </section>
    </Layout>
  )
}
