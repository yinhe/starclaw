import { Heart, Eye, Code2, Shield, Scale, Mail, Github, ArrowRight } from 'lucide-react'
import { Layout } from '../components/Layout'
import { useI18n } from '../i18n'

const VALUES = [
  { key: 'open', icon: Code2 },
  { key: 'privacy', icon: Shield },
  { key: 'fair', icon: Scale },
]

export function AboutPage() {
  const { t } = useI18n()

  return (
    <Layout>
      {/* Hero */}
      <section className="py-24">
        <div className="mx-auto max-w-5xl px-6 text-center">
          <h1 className="text-4xl md:text-5xl font-bold tracking-tight">{t('about.title')}</h1>
        </div>
      </section>

      {/* Mission & Vision */}
      <section className="py-24 border-t border-white/5">
        <div className="mx-auto max-w-4xl px-6">
          <div className="grid grid-cols-1 md:grid-cols-2 gap-8">
            <div className="rounded-xl border border-white/10 bg-white/[0.02] p-8">
              <div className="inline-flex items-center justify-center w-12 h-12 rounded-lg bg-claw-500/10 text-claw-400 mb-4">
                <Heart size={24} />
              </div>
              <h2 className="text-2xl font-bold text-white mb-3">{t('about.mission')}</h2>
              <p className="text-gray-400 leading-relaxed">{t('about.mission.desc')}</p>
            </div>
            <div className="rounded-xl border border-white/10 bg-white/[0.02] p-8">
              <div className="inline-flex items-center justify-center w-12 h-12 rounded-lg bg-claw-500/10 text-claw-400 mb-4">
                <Eye size={24} />
              </div>
              <h2 className="text-2xl font-bold text-white mb-3">{t('about.vision')}</h2>
              <p className="text-gray-400 leading-relaxed">{t('about.vision.desc')}</p>
            </div>
          </div>
        </div>
      </section>

      {/* Values */}
      <section className="py-24 border-t border-white/5">
        <div className="mx-auto max-w-4xl px-6">
          <h2 className="text-3xl font-bold tracking-tight text-center mb-12">{t('about.values')}</h2>
          <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
            {VALUES.map(v => (
              <div key={v.key} className="rounded-xl border border-white/10 bg-white/[0.02] p-6 text-center">
                <div className="inline-flex items-center justify-center w-10 h-10 rounded-lg bg-claw-500/10 text-claw-400 mb-4">
                  <v.icon size={20} />
                </div>
                <h3 className="font-semibold text-white mb-2">{t(`about.v.${v.key}`)}</h3>
                <p className="text-sm text-gray-400 leading-relaxed">{t(`about.v.${v.key}.desc`)}</p>
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* Contact */}
      <section className="py-24 border-t border-white/5">
        <div className="mx-auto max-w-xl px-6 text-center">
          <h2 className="text-3xl font-bold tracking-tight mb-8">{t('about.contact')}</h2>
          <div className="space-y-4">
            <a
              href={`mailto:${t('about.contact.email')}`}
              className="flex items-center justify-center gap-3 rounded-xl border border-white/10 bg-white/[0.02] p-4 text-gray-300 hover:text-white hover:border-claw-500/30 transition-colors"
            >
              <Mail size={20} className="text-claw-400" />
              {t('about.contact.email')}
              <ArrowRight size={14} className="text-gray-500" />
            </a>
            <a
              href={`https://${t('about.contact.github')}`}
              target="_blank"
              rel="noopener noreferrer"
              className="flex items-center justify-center gap-3 rounded-xl border border-white/10 bg-white/[0.02] p-4 text-gray-300 hover:text-white hover:border-claw-500/30 transition-colors"
            >
              <Github size={20} className="text-claw-400" />
              {t('about.contact.github')}
              <ArrowRight size={14} className="text-gray-500" />
            </a>
          </div>
        </div>
      </section>
    </Layout>
  )
}
