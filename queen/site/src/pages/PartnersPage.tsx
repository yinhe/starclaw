import { MapPin, TrendingUp, Headphones, Megaphone, Send, CheckCircle } from 'lucide-react'
import { Layout } from '../components/Layout'
import { useI18n } from '../i18n'
import { useState } from 'react'

const BENEFITS = [
  { key: 'w1', icon: MapPin },
  { key: 'w2', icon: TrendingUp },
  { key: 'w3', icon: Headphones },
  { key: 'w4', icon: Megaphone },
]

const FIELDS = ['name', 'company', 'city', 'phone', 'email'] as const

export function PartnersPage() {
  const { t } = useI18n()
  const [submitted, setSubmitted] = useState(false)
  const [form, setForm] = useState<Record<string, string>>({})

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    // In production, POST to an API endpoint
    console.log('Partner application:', form)
    setSubmitted(true)
  }

  return (
    <Layout>
      {/* Hero */}
      <section className="py-24">
        <div className="mx-auto max-w-5xl px-6 text-center">
          <h1 className="text-4xl md:text-5xl font-bold tracking-tight">{t('partner.title')}</h1>
          <p className="mt-4 text-lg text-gray-400 max-w-2xl mx-auto">{t('partner.desc')}</p>
        </div>
      </section>

      {/* Benefits */}
      <section className="py-24 border-t border-white/5">
        <div className="mx-auto max-w-5xl px-6">
          <h2 className="text-3xl font-bold tracking-tight text-center mb-12">{t('partner.why')}</h2>
          <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
            {BENEFITS.map(b => (
              <div key={b.key} className="rounded-xl border border-white/10 bg-white/[0.02] p-6 flex gap-4">
                <div className="w-10 h-10 rounded-lg bg-claw-500/10 text-claw-400 flex items-center justify-center shrink-0">
                  <b.icon size={20} />
                </div>
                <div>
                  <h3 className="font-semibold text-white mb-1">{t(`partner.${b.key}`)}</h3>
                  <p className="text-sm text-gray-400 leading-relaxed">{t(`partner.${b.key}.desc`)}</p>
                </div>
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* Application Form */}
      <section className="py-24 border-t border-white/5">
        <div className="mx-auto max-w-xl px-6">
          <h2 className="text-3xl font-bold tracking-tight text-center mb-8">{t('partner.apply')}</h2>

          {submitted ? (
            <div className="rounded-xl border border-green-500/20 bg-green-500/5 p-8 text-center">
              <CheckCircle size={48} className="text-green-400 mx-auto mb-4" />
              <p className="text-green-400 font-medium">{t('partner.form.success')}</p>
            </div>
          ) : (
            <form onSubmit={handleSubmit} className="space-y-4">
              {FIELDS.map(f => (
                <div key={f}>
                  <label className="block text-sm text-gray-400 mb-1.5">{t(`partner.form.${f}`)}</label>
                  <input
                    required
                    type={f === 'email' ? 'email' : f === 'phone' ? 'tel' : 'text'}
                    value={form[f] || ''}
                    onChange={e => setForm({ ...form, [f]: e.target.value })}
                    className="w-full rounded-lg border border-white/10 bg-white/[0.02] px-4 py-2.5 text-white text-sm focus:outline-none focus:border-claw-500 placeholder-gray-600"
                  />
                </div>
              ))}
              <div>
                <label className="block text-sm text-gray-400 mb-1.5">{t('partner.form.exp')}</label>
                <textarea
                  rows={4}
                  value={form.exp || ''}
                  onChange={e => setForm({ ...form, exp: e.target.value })}
                  className="w-full rounded-lg border border-white/10 bg-white/[0.02] px-4 py-2.5 text-white text-sm focus:outline-none focus:border-claw-500 placeholder-gray-600 resize-none"
                />
              </div>
              <button
                type="submit"
                className="w-full rounded-lg bg-claw-600 py-3 text-sm font-semibold text-white hover:bg-claw-500 transition-colors flex items-center justify-center gap-2"
              >
                <Send size={16} />
                {t('partner.form.submit')}
              </button>
            </form>
          )}
        </div>
      </section>
    </Layout>
  )
}
