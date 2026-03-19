import { ArrowRight, Globe, Zap, CreditCard, Code2 } from 'lucide-react'
import { Layout } from '../components/Layout'
import { useI18n } from '../i18n'
import { CopyBlock } from '../components/CopyBlock'

const MODELS = [
  { name: 'GPT-4o', provider: 'OpenAI', input: '$2.50/M', output: '$10.00/M', tag: 'Flagship' },
  { name: 'GPT-4o-mini', provider: 'OpenAI', input: '$0.15/M', output: '$0.60/M', tag: 'Value' },
  { name: 'Claude 3.5 Sonnet', provider: 'Anthropic', input: '$3.00/M', output: '$15.00/M', tag: 'Flagship' },
  { name: 'DeepSeek-V3', provider: 'DeepSeek', input: '$0.14/M', output: '$0.28/M', tag: 'Domestic' },
  { name: 'Qwen-Max', provider: 'Qwen', input: '$0.28/M', output: '$0.84/M', tag: 'Domestic' },
  { name: 'Qwen-Turbo', provider: 'Qwen', input: '$0.04/M', output: '$0.08/M', tag: 'Fast' },
  { name: 'Gemini 2.0 Flash', provider: 'Google', input: '$0.07/M', output: '$0.28/M', tag: 'Multimodal' },
  { name: 'Grok-2', provider: 'xAI', input: '$2.00/M', output: '$10.00/M', tag: 'Reasoning' },
]

const FEATURES = [
  { key: 'unified', icon: Globe },
  { key: 'domestic', icon: Zap },
  { key: 'billing', icon: CreditCard },
]

const STEPS = ['1', '2', '3', '4']

export function StarAIPage() {
  const { t } = useI18n()

  return (
    <Layout>
      {/* Hero */}
      <section className="relative overflow-hidden">
        <div className="absolute inset-0 bg-gradient-to-b from-amber-950/20 via-transparent to-transparent" />
        <div className="mx-auto max-w-5xl px-6 pt-24 pb-20 text-center relative">
          <div className="inline-flex items-center gap-2 rounded-full border border-amber-500/30 bg-amber-500/10 px-4 py-1.5 text-sm text-amber-400 mb-8">
            <Zap size={14} />
            star-ai.net
          </div>
          <h1 className="text-4xl md:text-6xl font-bold tracking-tight leading-tight">
            {t('ai.title')}
            <br />
            <span className="text-transparent bg-clip-text bg-gradient-to-r from-amber-400 to-orange-400">
              {t('ai.subtitle')}
            </span>
          </h1>
          <p className="mt-6 text-lg text-gray-400 max-w-2xl mx-auto">
            {t('ai.desc')}
          </p>
          <div className="mt-10 flex flex-col sm:flex-row items-center justify-center gap-4">
            <a
              href="https://star-ai.net"
              target="_blank"
              rel="noopener noreferrer"
              className="inline-flex items-center gap-2 rounded-lg bg-amber-600 px-8 py-3.5 text-sm font-semibold text-white hover:bg-amber-500 transition-colors"
            >
              {t('ai.cta')}
              <ArrowRight size={16} />
            </a>
          </div>
          <div className="mt-12 max-w-2xl mx-auto">
            <CopyBlock text='curl https://api.star-ai.net/v1/chat/completions -H "Authorization: Bearer sk-star-xxx" -H "Content-Type: application/json" -d &apos;{"model":"qwen/qwen-turbo","messages":[{"role":"user","content":"Hello"}]}&apos;' />
          </div>
        </div>
      </section>

      {/* Features */}
      <section className="py-16 md:py-24 border-t border-white/5">
        <div className="mx-auto max-w-5xl px-4 sm:px-6">
          <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
            {FEATURES.map(f => (
              <div key={f.key} className="rounded-xl border border-white/10 bg-white/[0.02] p-6">
                <div className="inline-flex items-center justify-center w-10 h-10 rounded-lg bg-amber-500/10 text-amber-400 mb-4">
                  <f.icon size={20} />
                </div>
                <h3 className="text-lg font-semibold text-white mb-2">{t(`ai.feature.${f.key}`)}</h3>
                <p className="text-sm text-gray-400 leading-relaxed">{t(`ai.feature.${f.key}.desc`)}</p>
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* How it works */}
      <section className="py-16 md:py-24 border-t border-white/5">
        <div className="mx-auto max-w-4xl px-4 sm:px-6">
          <h2 className="text-2xl sm:text-3xl font-bold tracking-tight text-center mb-12">{t('ai.how')}</h2>
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-6">
            {STEPS.map((s, i) => (
              <div key={s} className="text-center">
                <div className="w-10 h-10 rounded-full bg-amber-500/10 text-amber-400 font-bold text-lg flex items-center justify-center mx-auto mb-3">
                  {i + 1}
                </div>
                <p className="text-sm text-gray-300">{t(`ai.how.${s}`)}</p>
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* Models */}
      <section className="py-16 md:py-24 border-t border-white/5">
        <div className="mx-auto max-w-5xl px-4 sm:px-6">
          <div className="text-center mb-12">
            <h2 className="text-2xl sm:text-3xl font-bold tracking-tight">{t('ai.models')}</h2>
            <p className="mt-4 text-gray-400">{t('ai.models.desc')}</p>
          </div>
          <div className="rounded-xl border border-white/10 overflow-x-auto -mx-6 px-6 md:mx-0 md:px-0">
            <table className="w-full text-sm min-w-[540px]">
              <thead>
                <tr className="border-b border-white/10 text-gray-400 text-left">
                  <th className="px-5 py-3 font-medium">Model</th>
                  <th className="px-5 py-3 font-medium">Provider</th>
                  <th className="px-5 py-3 font-medium text-right">Input</th>
                  <th className="px-5 py-3 font-medium text-right">Output</th>
                  <th className="px-5 py-3 font-medium"></th>
                </tr>
              </thead>
              <tbody>
                {MODELS.map(m => (
                  <tr key={m.name} className="border-b border-white/5 hover:bg-white/[0.02]">
                    <td className="px-5 py-3 font-medium text-white">{m.name}</td>
                    <td className="px-5 py-3 text-gray-400">{m.provider}</td>
                    <td className="px-5 py-3 text-gray-300 text-right">{m.input}</td>
                    <td className="px-5 py-3 text-gray-300 text-right">{m.output}</td>
                    <td className="px-5 py-3">
                      <span className="text-[10px] px-1.5 py-0.5 rounded bg-amber-500/10 text-amber-400 border border-amber-500/20">
                        {m.tag}
                      </span>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          <p className="text-center text-xs text-gray-500 mt-4">
            100+ models available. Prices shown are upstream USD per million tokens. StarAI markup: 30%.
          </p>
        </div>
      </section>

      {/* API example */}
      <section className="py-16 md:py-24 border-t border-white/5">
        <div className="mx-auto max-w-3xl px-4 sm:px-6">
          <h2 className="text-2xl sm:text-3xl font-bold tracking-tight text-center mb-8">
            <Code2 className="inline w-8 h-8 text-amber-400 mr-2" />
            OpenAI-Compatible API
          </h2>
          <div className="rounded-xl border border-white/10 bg-gray-900 p-4 sm:p-6 font-mono text-xs sm:text-sm overflow-x-auto">
            <pre className="text-gray-300">
{`import openai

client = openai.OpenAI(
    base_url="https://api.star-ai.net/v1",
    api_key="sk-star-xxxx"
)

resp = client.chat.completions.create(
    model="qwen/qwen-turbo",
    messages=[{"role": "user", "content": "Hello!"}]
)
print(resp.choices[0].message.content)`}
            </pre>
          </div>
          <p className="text-center text-sm text-gray-500 mt-4">
            Works with any OpenAI SDK — Python, Node.js, Go, Rust, etc.
          </p>
        </div>
      </section>
    </Layout>
  )
}
