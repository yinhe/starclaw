import { Terminal, Apple, Monitor, Container, ArrowRight, Download, Zap, Copy, Check } from 'lucide-react'
import { useState, useEffect } from 'react'
import { Link } from 'react-router-dom'
import { Layout } from '../components/Layout'
import { CopyBlock } from '../components/CopyBlock'
import { useI18n } from '../i18n'

const NYDUS_BASE = 'https://nydus.starclaw.net/spore/releases'
const STARAI_BASE = 'https://star-ai.net/downloads'
const V = 'v2026.0320.1609'

const PACKAGES = [
  {
    id: 'win',
    icon: Monitor,
    setupUrl: `${NYDUS_BASE}/StarClaw-Setup-${V}.exe`,
    mirrorUrl: `${STARAI_BASE}/StarClaw-Setup-${V}.exe`,
    setupSize: '26 MB',
    setupLabel: 'StarClaw-Setup.exe',
    scriptCmd: 'irm https://nydus.starclaw.net/spore/install.ps1 | iex',
  },
  {
    id: 'linux',
    icon: Terminal,
    setupUrl: `${NYDUS_BASE}/StarClaw-Setup-${V}-linux-amd64.tar.gz`,
    mirrorUrl: `${STARAI_BASE}/StarClaw-Setup-${V}-linux-amd64.tar.gz`,
    setupSize: '19 MB',
    setupLabel: 'StarClaw-Setup.tar.gz',
    scriptCmd: 'curl -fsSL https://nydus.starclaw.net/spore/install.sh | sh',
  },
  {
    id: 'mac.arm',
    icon: Apple,
    setupUrl: `${NYDUS_BASE}/StarClaw-Setup-${V}-darwin-arm64.dmg`,
    mirrorUrl: `${STARAI_BASE}/StarClaw-Setup-${V}-darwin-arm64.dmg`,
    setupSize: '26 MB',
    setupLabel: 'StarClaw-Setup.dmg',
    scriptCmd: 'curl -fsSL https://nydus.starclaw.net/spore/install.sh | sh',
  },
  {
    id: 'mac.intel',
    icon: Apple,
    setupUrl: `${NYDUS_BASE}/StarClaw-Setup-${V}-darwin-amd64.dmg`,
    mirrorUrl: `${STARAI_BASE}/StarClaw-Setup-${V}-darwin-amd64.dmg`,
    setupSize: '27 MB',
    setupLabel: 'StarClaw-Setup.dmg',
    scriptCmd: 'curl -fsSL https://nydus.starclaw.net/spore/install.sh | sh',
  },
]

const DOCKER_CMD = `git clone https://github.com/yinhe/starclaw.git
cd starclaw && cp .env.example .env
docker compose up -d`

const DOCKER_CMD_MIRROR = `git clone https://nydus.starclaw.net/git/starclaw.git
cd starclaw && cp .env.example .env
docker compose up -d`

function CopyButton({ text }: { text: string }) {
  const [copied, setCopied] = useState(false)
  return (
    <button
      onClick={() => { navigator.clipboard.writeText(text); setCopied(true); setTimeout(() => setCopied(false), 2000) }}
      className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-white/5 hover:bg-white/10 text-xs text-gray-400 hover:text-white transition-colors"
    >
      {copied ? <Check size={12} className="text-green-400" /> : <Copy size={12} />}
      {copied ? 'Copied' : 'Copy'}
    </button>
  )
}

export function DownloadPage() {
  const { t } = useI18n()
  const [version, setVersion] = useState('')

  useEffect(() => {
    fetch('https://nydus.starclaw.net/releases/latest')
      .then(r => r.json())
      .then(d => {
        const v = d.tag_name || ''
        setVersion(v.startsWith('v') ? v : 'v' + v)
      })
      .catch(() => {})
  }, [])

  return (
    <Layout>
      <section className="py-12 md:py-20">
        <div className="mx-auto max-w-4xl px-4 sm:px-6">
          <div className="text-center mb-16">
            <h1 className="text-4xl md:text-5xl font-bold tracking-tight">
              {t('dl.title')}
            </h1>
            <p className="mt-4 text-gray-400 text-lg">
              {t('dl.desc')}
            </p>
          </div>

          {/* Spore One-Click — Hero */}
          <div className="rounded-2xl border-2 border-claw-500/40 bg-gradient-to-br from-claw-500/[0.08] to-transparent p-4 sm:p-8 md:p-10 mb-12">
            <div className="flex items-center gap-3 mb-6">
              <div className="inline-flex items-center justify-center w-12 h-12 rounded-xl bg-claw-500/20 text-claw-400">
                <Zap size={24} />
              </div>
              <div>
                <h2 className="text-2xl font-bold text-white">
                  Spore — {t('dl.quick')}
                  {version && <span className="ml-2 text-base font-medium text-claw-400">{version}</span>}
                </h2>
                <p className="text-sm text-gray-400">{t('dl.spore.tagline')}</p>
              </div>
            </div>

            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              {PACKAGES.map((pkg) => {
                const Icon = pkg.icon
                return (
                  <div
                    key={pkg.id}
                    className="rounded-xl border p-4 sm:p-6 flex flex-col border-white/10 bg-white/[0.03] hover:border-claw-500/30 transition-colors overflow-hidden"
                  >
                    <div className="min-h-[52px] mb-4">
                      <div className="flex items-center gap-2">
                        <Icon size={20} className="text-gray-400 shrink-0" />
                        <span className="font-semibold text-white text-lg leading-tight">{t(`dl.pkg.${pkg.id}`)}</span>
                      </div>
                      <div className="text-xs text-gray-500 ml-7 mt-1">{t(`dl.pkg.${pkg.id}.arch`)}</div>
                    </div>

                    {pkg.setupUrl && (
                      <>
                        <a
                          href={pkg.mirrorUrl}
                          download
                          className="flex items-center justify-center gap-2 w-full min-h-[48px] px-3 sm:px-4 rounded-lg bg-claw-500 hover:bg-claw-400 text-white font-medium text-xs sm:text-sm transition-colors mb-1.5"
                        >
                          <Download size={16} className="shrink-0" />
                          <span className="truncate">{pkg.setupLabel}</span>
                          <span className="text-claw-200 text-xs shrink-0">({pkg.setupSize})</span>
                        </a>
                        <div className="flex items-center justify-center gap-2 mb-3">
                          <span className="text-[10px] text-green-400">⚡ {t('dl.mirror.cn')}</span>
                          <span className="text-[10px] text-gray-600">|</span>
                          <a href={pkg.setupUrl} download className="text-[10px] text-gray-500 hover:text-gray-300 transition-colors">{t('dl.mirror.overseas')}</a>
                        </div>
                        <p className="text-xs text-gray-500 text-center mb-4">{t(`dl.pkg.${pkg.id}.note`)}</p>
                      </>
                    )}

                    <div className="mt-auto">
                      <div className="text-xs text-gray-500 mb-1.5">{t(`dl.pkg.${pkg.id}.script`)}</div>
                      <div className="rounded-lg bg-gray-900/80 px-2 sm:px-3 py-2">
                        <code className="block text-[11px] sm:text-xs text-gray-400 break-all whitespace-pre-wrap mb-2">
                          {pkg.scriptCmd}
                        </code>
                        <CopyButton text={pkg.scriptCmd} />
                      </div>
                    </div>
                  </div>
                )
              })}
            </div>

            <p className="mt-4 text-xs text-gray-500 text-center">
              {t('dl.spore.footer')}
            </p>
          </div>

          {/* Docker Method */}
          <div className="rounded-xl border border-white/10 bg-white/[0.02] p-4 sm:p-8 mb-8">
            <div className="flex items-center gap-3 mb-4">
              <div className="inline-flex items-center justify-center w-10 h-10 rounded-lg bg-claw-500/10 text-claw-400">
                <Container size={20} />
              </div>
              <div>
                <h3 className="text-lg font-semibold text-white">{t('dl.docker')}</h3>
                <p className="text-sm text-gray-400">{t('dl.docker.desc')}</p>
              </div>
            </div>
            <div className="space-y-3">
              <div>
                <div className="text-xs text-green-400 mb-1">⚡ {t('dl.mirror.docker.cn')}</div>
                <CopyBlock text={DOCKER_CMD_MIRROR} />
              </div>
              <div>
                <div className="text-xs text-gray-500 mb-1">GitHub</div>
                <CopyBlock text={DOCKER_CMD} />
              </div>
            </div>
            <p className="mt-3 text-sm text-gray-500">{t('dl.docker.note')}</p>
          </div>

          {/* Requirements */}
          <div className="rounded-xl border border-white/10 bg-white/[0.02] p-4 sm:p-8 mb-16">
            <h2 className="text-xl font-semibold text-white mb-6">{t('dl.reqs')}</h2>
            <div className="grid grid-cols-2 md:grid-cols-4 gap-4 text-sm">
              <div><div className="text-gray-500 mb-1">RAM</div><div className="text-white font-medium">2 GB+</div></div>
              <div><div className="text-gray-500 mb-1">Disk</div><div className="text-white font-medium">500 MB</div></div>
              <div><div className="text-gray-500 mb-1">OS</div><div className="text-white font-medium">Linux / macOS / Win</div></div>
              <div><div className="text-gray-500 mb-1">Docker (optional)</div><div className="text-white font-medium">24.0+</div></div>
            </div>
          </div>

          {/* CTA */}
          <div className="text-center">
            <Link
              to="/docs"
              className="inline-flex items-center gap-2 text-sm text-claw-400 hover:text-claw-300 transition-colors"
            >
              {t('dl.help')}
              <ArrowRight size={14} />
            </Link>
          </div>
        </div>
      </section>
    </Layout>
  )
}
