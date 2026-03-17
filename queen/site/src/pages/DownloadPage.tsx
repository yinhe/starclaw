import { Terminal, Apple, Monitor, Container, ArrowRight, Download, Zap, Copy, Check } from 'lucide-react'
import { useState } from 'react'
import { Link } from 'react-router-dom'
import { Layout } from '../components/Layout'
import { CopyBlock } from '../components/CopyBlock'
import { useI18n } from '../i18n'

const NYDUS_BASE = 'https://nydus.starclaw.net/spore/releases'

const PACKAGES = [
  {
    platform: 'Windows',
    icon: Monitor,
    sporeUrl: `${NYDUS_BASE}/claw-v1.0.0-windows-amd64.spore`,
    runtimeUrl: `${NYDUS_BASE}/spore-windows-amd64.exe`,
    size: '11.5 MB',
    runtimeSize: '6.0 MB',
    arch: 'x86_64',
    scriptCmd: 'irm https://nydus.starclaw.net/spore/install.ps1 | iex',
    scriptLabel: 'PowerShell',
  },
  {
    platform: 'Linux',
    icon: Terminal,
    sporeUrl: `${NYDUS_BASE}/claw-v1.0.0-linux-amd64.spore`,
    runtimeUrl: `${NYDUS_BASE}/spore-linux-amd64`,
    size: '12.2 MB',
    runtimeSize: '5.8 MB',
    arch: 'x86_64',
    scriptCmd: 'curl -fsSL https://nydus.starclaw.net/spore/install.sh | sh',
    scriptLabel: 'Bash',
  },
  {
    platform: 'macOS',
    icon: Apple,
    sporeUrl: '',
    runtimeUrl: '',
    size: '',
    runtimeSize: '',
    arch: 'Apple Silicon / Intel',
    scriptCmd: 'curl -fsSL https://nydus.starclaw.net/spore/install.sh | sh',
    scriptLabel: 'Terminal',
    comingSoon: true,
  },
]

const DOCKER_CMD = `git clone https://github.com/yinhe/starclaw.git
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

  return (
    <Layout>
      <section className="py-20">
        <div className="mx-auto max-w-4xl px-6">
          <div className="text-center mb-16">
            <h1 className="text-4xl md:text-5xl font-bold tracking-tight">
              {t('dl.title')}
            </h1>
            <p className="mt-4 text-gray-400 text-lg">
              {t('dl.desc')}
            </p>
          </div>

          {/* Spore One-Click — Hero */}
          <div className="rounded-2xl border-2 border-claw-500/40 bg-gradient-to-br from-claw-500/[0.08] to-transparent p-8 md:p-10 mb-12">
            <div className="flex items-center gap-3 mb-6">
              <div className="inline-flex items-center justify-center w-12 h-12 rounded-xl bg-claw-500/20 text-claw-400">
                <Zap size={24} />
              </div>
              <div>
                <h2 className="text-2xl font-bold text-white">Spore — {t('dl.quick')}</h2>
                <p className="text-sm text-gray-400">{t('dl.spore.tagline')}</p>
              </div>
            </div>

            <div className="grid md:grid-cols-3 gap-4">
              {PACKAGES.map((pkg) => {
                const Icon = pkg.icon
                return (
                  <div
                    key={pkg.platform}
                    className={`rounded-xl border p-6 flex flex-col ${
                      pkg.comingSoon
                        ? 'border-white/5 bg-white/[0.02] opacity-50'
                        : 'border-white/10 bg-white/[0.03] hover:border-claw-500/30 transition-colors'
                    }`}
                  >
                    <div className="flex items-center gap-2 mb-4">
                      <Icon size={20} className="text-gray-400" />
                      <span className="font-semibold text-white">{pkg.platform}</span>
                      <span className="text-xs text-gray-500">{pkg.arch}</span>
                    </div>

                    {pkg.comingSoon ? (
                      <div className="text-sm text-gray-500 mb-4 flex-1">Coming Soon</div>
                    ) : (
                      <>
                        <a
                          href={pkg.sporeUrl}
                          download
                          className="inline-flex items-center justify-center gap-2 w-full px-4 py-3 rounded-lg bg-claw-500 hover:bg-claw-400 text-white font-medium text-sm transition-colors mb-3"
                        >
                          <Download size={16} />
                          Claw v1.0.0
                          <span className="text-claw-200 text-xs">({pkg.size})</span>
                        </a>
                        <a
                          href={pkg.runtimeUrl}
                          download
                          className="inline-flex items-center justify-center gap-2 w-full px-3 py-2 rounded-lg border border-white/10 hover:border-claw-500/30 text-gray-300 hover:text-white text-xs transition-colors mb-4"
                        >
                          <Download size={12} />
                          Spore Runtime ({pkg.runtimeSize})
                        </a>
                      </>
                    )}

                    <div className="mt-auto">
                      <div className="text-xs text-gray-500 mb-1.5">{pkg.scriptLabel}</div>
                      <div className="flex items-center gap-2 rounded-lg bg-gray-900/80 px-3 py-2">
                        <code className="text-xs text-gray-400 flex-1 overflow-hidden text-ellipsis whitespace-nowrap">
                          {pkg.scriptCmd}
                        </code>
                        {!pkg.comingSoon && <CopyButton text={pkg.scriptCmd} />}
                      </div>
                    </div>
                  </div>
                )
              })}
            </div>

            <p className="mt-4 text-xs text-gray-500 text-center">
              Spore: ~18 MB total · No Docker required · Delta updates ~2 MB
            </p>
          </div>

          {/* Docker Method */}
          <div className="rounded-xl border border-white/10 bg-white/[0.02] p-8 mb-8">
            <div className="flex items-center gap-3 mb-4">
              <div className="inline-flex items-center justify-center w-10 h-10 rounded-lg bg-claw-500/10 text-claw-400">
                <Container size={20} />
              </div>
              <div>
                <h3 className="text-lg font-semibold text-white">{t('dl.docker')}</h3>
                <p className="text-sm text-gray-400">{t('dl.docker.desc')}</p>
              </div>
            </div>
            <CopyBlock text={DOCKER_CMD} />
            <p className="mt-3 text-sm text-gray-500">{t('dl.docker.note')}</p>
          </div>

          {/* Requirements */}
          <div className="rounded-xl border border-white/10 bg-white/[0.02] p-8 mb-16">
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
