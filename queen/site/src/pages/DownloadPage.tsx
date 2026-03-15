import { Terminal, Apple, Monitor, Container, ArrowRight, Globe } from 'lucide-react'
import { Link } from 'react-router-dom'
import { Layout } from '../components/Layout'
import { CopyBlock } from '../components/CopyBlock'
import { useI18n } from '../i18n'

const QUICK_CMD = `curl -fsSL https://starclaw.me/install.sh | bash`

const NYDUS_CMD = `curl -fsSL https://starclaw.me/install-cn.sh | bash`

const DOCKER_CMD = `git clone https://github.com/yinhe/starclaw.git
cd starclaw
cp .env.example .env
docker compose up -d`

const SOURCE_CMD = `git clone https://github.com/yinhe/starclaw.git
cd starclaw
# API
cd api && go run ./cmd/server
# Web (new terminal)
cd web && npm install && npm run dev`

const REQUIREMENTS = [
  { label: 'Docker', value: '24.0+' },
  { label: 'Docker Compose', value: 'v2.20+' },
  { label: 'RAM', value: '2 GB minimum' },
  { label: 'Disk', value: '5 GB free' },
  { label: 'OS', value: 'Linux / macOS / Windows' },
]

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

          {/* Quick Start */}
          <div className="rounded-xl border border-claw-500/30 bg-claw-500/[0.05] p-8 mb-12">
            <h2 className="text-xl font-semibold text-white mb-4 flex items-center gap-2">
              <Container size={20} className="text-claw-400" />
              {t('dl.quick')}
            </h2>
            <CopyBlock text={QUICK_CMD} />
            <p className="mt-3 text-sm text-gray-500">{t('dl.docker.note')}</p>
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

          {/* Nydus Mirror */}
          <div className="rounded-xl border border-orange-500/30 bg-orange-500/[0.03] p-8 mb-8">
            <div className="flex items-center gap-3 mb-4">
              <div className="inline-flex items-center justify-center w-10 h-10 rounded-lg bg-orange-500/10 text-orange-400">
                <Globe size={20} />
              </div>
              <div>
                <h3 className="text-lg font-semibold text-white">{t('dl.nydus')}</h3>
                <p className="text-sm text-gray-400">{t('dl.nydus.desc')}</p>
              </div>
            </div>
            <CopyBlock text={NYDUS_CMD} />
            <p className="mt-3 text-sm text-gray-500">
              {t('dl.nydus.note')}{' '}
              <a
                href="https://nydus.starclaw.net"
                target="_blank"
                rel="noopener noreferrer"
                className="text-orange-400 hover:underline"
              >
                nydus.starclaw.net
              </a>
            </p>
          </div>

          {/* From Source */}
          <div className="rounded-xl border border-white/10 bg-white/[0.02] p-8 mb-16">
            <div className="flex items-center gap-3 mb-4">
              <div className="inline-flex items-center justify-center w-10 h-10 rounded-lg bg-claw-500/10 text-claw-400">
                <Terminal size={20} />
              </div>
              <div>
                <h3 className="text-lg font-semibold text-white">{t('dl.source')}</h3>
                <p className="text-sm text-gray-400">{t('dl.source.desc')}</p>
              </div>
            </div>
            <CopyBlock text={SOURCE_CMD} />
            <p className="mt-3 text-sm text-gray-500">{t('dl.source.note')}</p>
          </div>

          {/* Requirements */}
          <div className="rounded-xl border border-white/10 bg-white/[0.02] p-8 mb-16">
            <h2 className="text-xl font-semibold text-white mb-6">{t('dl.reqs')}</h2>
            <div className="grid grid-cols-2 md:grid-cols-3 gap-4">
              {REQUIREMENTS.map((r) => (
                <div key={r.label} className="text-sm">
                  <div className="text-gray-500 mb-1">{r.label}</div>
                  <div className="text-white font-medium">{r.value}</div>
                </div>
              ))}
            </div>
          </div>

          {/* Platform Icons */}
          <div className="text-center">
            <div className="flex items-center justify-center gap-8 text-gray-500 mb-6">
              <div className="flex flex-col items-center gap-2">
                <Terminal size={28} />
                <span className="text-xs">Linux</span>
              </div>
              <div className="flex flex-col items-center gap-2">
                <Apple size={28} />
                <span className="text-xs">macOS</span>
              </div>
              <div className="flex flex-col items-center gap-2">
                <Monitor size={28} />
                <span className="text-xs">Windows</span>
              </div>
            </div>
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
