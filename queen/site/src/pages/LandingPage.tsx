import { Link } from 'react-router-dom'
import { useState } from 'react'
import {
  Bot,
  Brain,
  Code2,
  Download,
  GitFork,
  Globe,
  MessageSquare,
  Network,
  Puzzle,
  Shield,
  Workflow,
  Zap,
  Video,
  ArrowRight,
  Users,
  Sparkles,
  Terminal,
  Monitor,
  Container,
} from 'lucide-react'
import { Layout } from '../components/Layout'
import { CopyBlock } from '../components/CopyBlock'
import { useI18n } from '../i18n'

const INSTALL_TABS = [
  { key: 'linux', label: 'Linux / macOS', icon: Terminal, commands: [
    { label: 'GitHub', cmd: 'curl -fsSL https://starclaw.me/install.sh | bash' },
    { label: '国内镜像', cmd: 'curl -fsSL https://nydus.starclaw.net/install.sh | bash' },
  ]},
  { key: 'windows', label: 'Windows', icon: Monitor, commands: [
    { label: 'PowerShell (GitHub)', cmd: 'irm https://starclaw.me/install.ps1 | iex' },
    { label: 'PowerShell (国内镜像)', cmd: 'irm https://nydus.starclaw.net/install.ps1 | iex' },
  ]},
  { key: 'docker', label: 'Docker', icon: Container, commands: [
    { label: 'Docker Compose', cmd: 'git clone https://github.com/yinhe/starclaw.git && cd starclaw && docker compose up -d' },
    { label: 'Docker (国内镜像)', cmd: 'git clone https://nydus.starclaw.net/git/starclaw.git && cd starclaw && docker compose up -d' },
  ]},
]

function InstallBlock() {
  const [tab, setTab] = useState(0)
  const current = INSTALL_TABS[tab]
  return (
    <div className="mt-12 max-w-2xl mx-auto">
      <div className="flex items-center justify-center gap-1 mb-3">
        {INSTALL_TABS.map((t, i) => (
          <button
            key={t.key}
            onClick={() => setTab(i)}
            className={`inline-flex items-center gap-1.5 px-4 py-2 rounded-lg text-xs font-medium transition ${
              tab === i
                ? 'bg-claw-500/15 text-claw-400 border border-claw-500/30'
                : 'text-gray-500 hover:text-gray-300 border border-transparent'
            }`}
          >
            <t.icon size={14} />
            {t.label}
          </button>
        ))}
      </div>
      <div className="space-y-2">
        {current.commands.map((c) => (
          <div key={c.label}>
            <div className="text-[11px] text-gray-500 mb-1 pl-1">{c.label}</div>
            <CopyBlock text={c.cmd} />
          </div>
        ))}
      </div>
    </div>
  )
}

const FEATURE_KEYS = [
  { icon: MessageSquare, key: 'chat' },
  { icon: Bot, key: 'agents' },
  { icon: Brain, key: 'kb' },
  { icon: Workflow, key: 'workflow' },
  { icon: Code2, key: 'coding' },
  { icon: Puzzle, key: 'tools' },
  { icon: Video, key: 'video' },
  { icon: Globe, key: 'multi' },
  { icon: Shield, key: 'selfhost' },
]

export function LandingPage() {
  const { t } = useI18n()

  return (
    <Layout>
      {/* Hero */}
      <section className="relative overflow-hidden">
        <div className="absolute inset-0 bg-gradient-to-b from-claw-950/30 via-transparent to-transparent" />
        <div className="mx-auto max-w-6xl px-4 sm:px-6 pt-16 sm:pt-24 pb-16 sm:pb-20 text-center relative">
          <div className="inline-flex items-center gap-2 rounded-full border border-claw-500/30 bg-claw-500/10 px-4 py-1.5 text-sm text-claw-400 mb-8">
            <Zap size={14} />
            {t('hero.badge')}
          </div>

          <h1 className="text-3xl sm:text-5xl md:text-7xl font-bold tracking-tight leading-tight">
            {t('hero.title1')}
            <br />
            <span className="text-transparent bg-clip-text bg-gradient-to-r from-claw-400 to-orange-400">
              {t('hero.title2')}
            </span>
          </h1>

          <p className="mt-6 text-lg md:text-xl text-gray-400 max-w-2xl mx-auto leading-relaxed">
            {t('hero.desc')}
          </p>

          <div className="mt-10 flex flex-col sm:flex-row items-center justify-center gap-4">
            <Link
              to="/download"
              className="inline-flex items-center gap-2 rounded-lg bg-claw-600 px-6 py-3 text-sm font-semibold text-white hover:bg-claw-500 transition-colors"
            >
              <Download size={18} />
              {t('hero.cta')}
            </Link>
            <a
              href="/create"
              className="inline-flex items-center gap-2 rounded-lg border border-claw-500/30 bg-claw-500/10 px-6 py-3 text-sm font-semibold text-claw-400 hover:bg-claw-500/20 transition-colors"
            >
              <Zap size={16} />
              {t('hero.cloud')}
            </a>
            <a
              href="https://app.starclaw.me"
              className="inline-flex items-center gap-2 rounded-lg border border-white/15 px-6 py-3 text-sm font-semibold text-gray-300 hover:bg-white/5 transition-colors"
            >
              {t('hero.demo')}
              <ArrowRight size={16} />
            </a>
            <a
              href="https://github.com/yinhe/starclaw"
              target="_blank"
              rel="noopener noreferrer"
              className="inline-flex items-center gap-2 rounded-lg border border-white/15 px-6 py-3 text-sm font-semibold text-gray-300 hover:bg-white/5 transition-colors"
            >
              <GitFork size={16} />
              GitHub
            </a>
            <a
              href="https://starclaw.net"
              target="_blank"
              rel="noopener noreferrer"
              className="inline-flex items-center gap-2 rounded-lg border border-claw-500/30 bg-claw-500/10 px-6 py-3 text-sm font-semibold text-claw-400 hover:bg-claw-500/20 transition-colors"
            >
              <Network size={16} />
              {t('hero.swarm')}
            </a>
          </div>

          {/* Quick install — multi-platform */}
          <InstallBlock />
        </div>
      </section>

      {/* Features Grid */}
      <section className="py-16 md:py-24 border-t border-white/5">
        <div className="mx-auto max-w-6xl px-4 sm:px-6">
          <div className="text-center mb-12 md:mb-16">
            <h2 className="text-2xl sm:text-3xl md:text-4xl font-bold tracking-tight">
              {t('features.title')}
            </h2>
            <p className="mt-4 text-gray-400 text-lg">
              {t('features.desc')}
            </p>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
            {FEATURE_KEYS.map((f) => (
              <div
                key={f.key}
                className="group rounded-xl border border-white/10 bg-white/[0.02] p-6 hover:border-claw-500/30 hover:bg-claw-500/[0.03] transition-all"
              >
                <div className="inline-flex items-center justify-center w-10 h-10 rounded-lg bg-claw-500/10 text-claw-400 mb-4">
                  <f.icon size={20} />
                </div>
                <h3 className="text-lg font-semibold text-white mb-2">{t(`feat.${f.key}`)}</h3>
                <p className="text-sm text-gray-400 leading-relaxed">{t(`feat.${f.key}.desc`)}</p>
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* Team Agent */}
      <section className="py-16 md:py-24 border-t border-white/5 bg-gradient-to-b from-transparent via-claw-950/20 to-transparent">
        <div className="mx-auto max-w-6xl px-4 sm:px-6">
          <div className="text-center mb-12 md:mb-16">
            <div className="inline-flex items-center gap-2 rounded-full border border-orange-500/30 bg-orange-500/10 px-4 py-1.5 text-sm text-orange-400 mb-6">
              <Users size={14} />
              {t('team.badge')}
            </div>
            <h2 className="text-2xl sm:text-3xl md:text-4xl font-bold tracking-tight">
              {t('team.title')}
            </h2>
            <p className="mt-4 text-gray-400 text-lg max-w-2xl mx-auto">
              {t('team.desc')}
            </p>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-5 mb-8">
            {[
              { key: 'dev', color: 'text-blue-400 bg-blue-500/10 border-blue-500/20' },
              { key: 'market', color: 'text-pink-400 bg-pink-500/10 border-pink-500/20' },
              { key: 'support', color: 'text-green-400 bg-green-500/10 border-green-500/20' },
              { key: 'data', color: 'text-cyan-400 bg-cyan-500/10 border-cyan-500/20' },
              { key: 'quant', color: 'text-yellow-400 bg-yellow-500/10 border-yellow-500/20' },
            ].map((tmpl) => (
              <div
                key={tmpl.key}
                className={`rounded-xl border p-5 ${tmpl.color} transition-all hover:scale-[1.02]`}
              >
                <div className="flex items-center gap-2 mb-2">
                  <Sparkles size={16} />
                  <h3 className="font-semibold">{t(`team.${tmpl.key}`)}</h3>
                </div>
                <p className="text-sm text-gray-400 leading-relaxed">{t(`team.${tmpl.key}.desc`)}</p>
              </div>
            ))}
            <div className="rounded-xl border border-white/10 bg-white/[0.02] p-5 flex items-center justify-center">
              <p className="text-sm text-gray-500 text-center">{t('team.more')}</p>
            </div>
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

      {/* Architecture */}
      <section className="py-16 md:py-24 border-t border-white/5">
        <div className="mx-auto max-w-6xl px-4 sm:px-6">
          <div className="text-center mb-12 md:mb-16">
            <h2 className="text-2xl sm:text-3xl md:text-4xl font-bold tracking-tight">
              {t('scale.title')}
            </h2>
            <p className="mt-4 text-gray-400 text-lg">
              {t('scale.desc')}
            </p>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-3 gap-8">
            <div className="rounded-xl border border-white/10 bg-white/[0.02] p-8 text-center">
              <div className="text-4xl font-bold text-claw-400 mb-2">1</div>
              <h3 className="text-lg font-semibold text-white mb-3">{t('scale.one')}</h3>
              <p className="text-sm text-gray-400">{t('scale.one.desc')}</p>
            </div>
            <div className="rounded-xl border border-white/10 bg-white/[0.02] p-8 text-center">
              <div className="text-4xl font-bold text-claw-400 mb-2">N</div>
              <h3 className="text-lg font-semibold text-white mb-3">{t('scale.n')}</h3>
              <p className="text-sm text-gray-400">{t('scale.n.desc')}</p>
            </div>
            <div className="rounded-xl border border-white/10 bg-white/[0.02] p-8 text-center">
              <div className="text-4xl font-bold text-claw-400 mb-2">&infin;</div>
              <h3 className="text-lg font-semibold text-white mb-3">{t('scale.inf')}</h3>
              <p className="text-sm text-gray-400">{t('scale.inf.desc')}</p>
            </div>
          </div>
        </div>
      </section>

      {/* CTA */}
      <section className="py-16 md:py-24 border-t border-white/5">
        <div className="mx-auto max-w-3xl px-4 sm:px-6 text-center">
          <h2 className="text-2xl sm:text-3xl md:text-4xl font-bold tracking-tight">
            {t('cta.title')}
          </h2>
          <p className="mt-4 text-gray-400 text-lg">
            {t('cta.desc')}
          </p>
          <div className="mt-8 flex flex-col sm:flex-row items-center justify-center gap-4">
            <Link
              to="/download"
              className="inline-flex items-center gap-2 rounded-lg bg-claw-600 px-8 py-3.5 text-sm font-semibold text-white hover:bg-claw-500 transition-colors"
            >
              <Download size={18} />
              {t('hero.install')}
            </Link>
            <Link
              to="/docs"
              className="inline-flex items-center gap-2 rounded-lg border border-white/15 px-8 py-3.5 text-sm font-semibold text-gray-300 hover:bg-white/5 transition-colors"
            >
              {t('hero.read_docs')}
              <ArrowRight size={16} />
            </Link>
          </div>
        </div>
      </section>
    </Layout>
  )
}
