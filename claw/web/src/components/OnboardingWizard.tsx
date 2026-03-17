import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Cpu, ArrowRight, Check, Sparkles, Key, MessageSquare } from 'lucide-react'
import { modelAPI } from '../lib/api'

const PROVIDERS = [
  { value: 'qwen', label: '通义千问 (Qwen)', icon: '🤖', placeholder: 'sk-...', url: 'https://dashscope.console.aliyun.com/', base_url: 'https://dashscope.aliyuncs.com/compatible-mode/v1' },
  { value: 'openai', label: 'OpenAI', icon: '🟢', placeholder: 'sk-...', url: 'https://platform.openai.com/api-keys', base_url: 'https://api.openai.com/v1' },
  { value: 'google', label: 'Google Gemini', icon: '🔵', placeholder: 'AIza...', url: 'https://aistudio.google.com/apikey', base_url: 'https://generativelanguage.googleapis.com/v1beta/openai' },
  { value: 'deepseek', label: 'DeepSeek', icon: '🐋', placeholder: 'sk-...', url: 'https://platform.deepseek.com/api_keys', base_url: 'https://api.deepseek.com/v1' },
  { value: 'anthropic', label: 'Anthropic', icon: '🟠', placeholder: 'sk-ant-...', url: 'https://console.anthropic.com/', base_url: 'https://api.anthropic.com' },
  { value: 'grok', label: 'Grok (xAI)', icon: '𝕏', placeholder: 'xai-...', url: 'https://console.x.ai/', base_url: 'https://api.x.ai/v1' },
  { value: 'minimax', label: 'MiniMax', icon: '🐚', placeholder: 'eyJ...', url: 'https://platform.minimaxi.com/user-center/basic-information/interface-key', base_url: 'https://api.minimaxi.com/v1' },
]

interface Props {
  onComplete: () => void
}

export default function OnboardingWizard({ onComplete }: Props) {
  const [step, setStep] = useState(0)
  const [selectedProvider, setSelectedProvider] = useState(PROVIDERS[0])
  const [apiKey, setApiKey] = useState('')
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')
  const navigate = useNavigate()

  const handleSaveKey = async () => {
    if (!apiKey.trim()) return
    setSaving(true)
    setError('')
    try {
      await modelAPI.create({
        provider: selectedProvider.value,
        api_key: apiKey.trim(),
        base_url: selectedProvider.base_url,
      })
      setStep(2)
    } catch {
      setError('Failed to save API key. Please check and try again.')
    } finally {
      setSaving(false)
    }
  }

  const finish = () => {
    localStorage.setItem('starclaw_onboarded', '1')
    onComplete()
  }

  return (
    <div className="fixed inset-0 bg-black/60 backdrop-blur-sm flex items-center justify-center z-50 p-4">
      <div className="bg-white rounded-2xl shadow-2xl w-full max-w-lg overflow-hidden">
        {/* Progress */}
        <div className="flex bg-gray-50 border-b">
          {['Welcome', 'Add API Key', 'Ready!'].map((label, i) => (
            <div key={label} className={`flex-1 py-3 text-center text-xs font-medium transition-colors ${i <= step ? 'text-primary-600' : 'text-gray-400'}`}>
              <div className="flex items-center justify-center gap-1.5">
                {i < step ? <Check className="w-3.5 h-3.5" /> : <span className="w-5 h-5 rounded-full border-2 inline-flex items-center justify-center text-[10px]" style={{ borderColor: i <= step ? 'var(--color-primary-500)' : '#d1d5db' }}>{i + 1}</span>}
                {label}
              </div>
            </div>
          ))}
        </div>

        <div className="p-6">
          {/* Step 0: Welcome */}
          {step === 0 && (
            <div className="text-center">
              <div className="text-5xl mb-4">🦞</div>
              <h2 className="text-xl font-bold text-gray-900 mb-2">Welcome to StarClaw!</h2>
              <p className="text-gray-500 text-sm mb-6 leading-relaxed">
                AI Agent orchestration platform with multi-model support, visual workflow, RAG knowledge base, and more.
                <br />Let's get you set up in 30 seconds.
              </p>
              <button
                onClick={() => setStep(1)}
                className="inline-flex items-center gap-2 px-6 py-2.5 bg-primary-600 text-white rounded-lg font-medium hover:bg-primary-700 transition-colors"
              >
                Get Started <ArrowRight className="w-4 h-4" />
              </button>
            </div>
          )}

          {/* Step 1: Add API Key */}
          {step === 1 && (
            <div>
              <div className="flex items-center gap-2 mb-4">
                <Key className="w-5 h-5 text-primary-500" />
                <h2 className="text-lg font-semibold text-gray-900">Add Your First Model</h2>
              </div>
              <p className="text-gray-500 text-sm mb-4">Choose a provider and enter your API key. You can add more later in Settings → Models.</p>

              <div className="grid grid-cols-5 gap-2 mb-4">
                {PROVIDERS.map((p) => (
                  <button
                    key={p.value}
                    onClick={() => setSelectedProvider(p)}
                    className={`py-2 px-1 rounded-lg border text-center text-xs transition-all ${
                      selectedProvider.value === p.value
                        ? 'border-primary-500 bg-primary-50 ring-1 ring-primary-500'
                        : 'border-gray-200 hover:border-gray-300'
                    }`}
                  >
                    <div className="text-lg mb-0.5">{p.icon}</div>
                    <div className="truncate">{p.label.split(' ')[0]}</div>
                  </button>
                ))}
              </div>

              <div className="mb-3">
                <label className="block text-sm font-medium text-gray-700 mb-1">
                  {selectedProvider.label} API Key
                </label>
                <input
                  type="password"
                  value={apiKey}
                  onChange={(e) => setApiKey(e.target.value)}
                  onKeyDown={(e) => e.key === 'Enter' && handleSaveKey()}
                  className="w-full px-3 py-2.5 border rounded-lg text-sm outline-none focus:ring-2 focus:ring-primary-500"
                  placeholder={selectedProvider.placeholder}
                  autoFocus
                />
                <a href={selectedProvider.url} target="_blank" rel="noopener noreferrer" className="text-xs text-primary-500 hover:underline mt-1 inline-block">
                  Get your API key →
                </a>
              </div>

              {error && <p className="text-red-500 text-xs mb-3">{error}</p>}

              <div className="flex gap-3">
                <button
                  onClick={handleSaveKey}
                  disabled={!apiKey.trim() || saving}
                  className="flex-1 py-2.5 bg-primary-600 text-white rounded-lg font-medium hover:bg-primary-700 disabled:opacity-50 transition-colors text-sm"
                >
                  {saving ? 'Saving...' : 'Save & Continue'}
                </button>
                <button
                  onClick={() => { setStep(2) }}
                  className="px-4 py-2.5 text-gray-500 hover:bg-gray-100 rounded-lg text-sm transition-colors"
                >
                  Skip
                </button>
              </div>
            </div>
          )}

          {/* Step 2: Done */}
          {step === 2 && (
            <div className="text-center">
              <div className="w-16 h-16 rounded-full bg-green-100 flex items-center justify-center mx-auto mb-4">
                <Sparkles className="w-8 h-8 text-green-600" />
              </div>
              <h2 className="text-xl font-bold text-gray-900 mb-2">You're All Set!</h2>
              <p className="text-gray-500 text-sm mb-6">Start chatting with your AI agents, build workflows, or explore the platform.</p>

              <div className="flex gap-3">
                <button
                  onClick={() => { finish(); navigate('/chat') }}
                  className="flex-1 inline-flex items-center justify-center gap-2 py-2.5 bg-primary-600 text-white rounded-lg font-medium hover:bg-primary-700 transition-colors text-sm"
                >
                  <MessageSquare className="w-4 h-4" /> Start Chatting
                </button>
                <button
                  onClick={() => { finish(); navigate('/models') }}
                  className="flex-1 inline-flex items-center justify-center gap-2 py-2.5 border border-gray-300 text-gray-700 rounded-lg font-medium hover:bg-gray-50 transition-colors text-sm"
                >
                  <Cpu className="w-4 h-4" /> Add More Models
                </button>
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
