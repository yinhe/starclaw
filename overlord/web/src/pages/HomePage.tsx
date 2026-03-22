import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { Bot, MessageSquare, Loader2, HeartPulse, Code, Megaphone, Headphones, BarChart3, TrendingUp, ShoppingCart, Film, Crosshair, Shield, Wrench } from 'lucide-react'
import { api, isEmployee } from '../api/client'

interface TeamInstance {
  id: string
  name: string
  template_name: string
  status: string
  goal: string
  energy_budget: number
  energy_used: number
  mission_count: number
  created_at: string
}

const templateIcons: Record<string, string> = {
  MedClaw: '\u{1FA7A}',
  DevClaw: '\u{1F4BB}',
  MarketClaw: '\u{1F4E2}',
  SupportClaw: '\u{1F3A7}',
  DataClaw: '\u{1F4CA}',
  QuantClaw: '\u{1F4C8}',
  EcomClaw: '\u{1F6D2}',
  DramaClaw: '\u{1F3AC}',
  SalesClaw: '\u{1F91D}',
  OpsClaw: '\u2699\uFE0F',
}

const statusDot: Record<string, string> = {
  forming: 'bg-yellow-400',
  ready: 'bg-blue-400',
  running: 'bg-green-400',
  paused: 'bg-orange-400',
  maintenance: 'bg-purple-400',
  completed: 'bg-gray-400',
  disbanded: 'bg-red-400',
}

const statusLabel: Record<string, string> = {
  forming: '\u7EC4\u5EFA\u4E2D',
  ready: '\u5C31\u7EEA',
  running: '\u8FD0\u884C\u4E2D',
  paused: '\u5DF2\u6682\u505C',
  completed: '\u5DF2\u5B8C\u6210',
  disbanded: '\u5DF2\u89E3\u6563',
}

function getIcon(templateName: string): string {
  for (const [key, icon] of Object.entries(templateIcons)) {
    if (templateName.includes(key)) return icon
  }
  return '\u{1F916}'
}

export default function HomePage() {
  const navigate = useNavigate()
  const [instances, setInstances] = useState<TeamInstance[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    loadInstances()
  }, [])

  async function loadInstances() {
    setLoading(true)
    try {
      const res = await api.teamInstances()
      setInstances((res.instances || []).filter((i: TeamInstance) => i.status !== 'disbanded'))
    } catch {}
    setLoading(false)
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center h-full text-gray-500 text-sm">
        <Loader2 className="w-4 h-4 animate-spin mr-2" />
        {'\u52A0\u8F7D\u4E2D...'}
      </div>
    )
  }

  return (
    <div className="h-full flex flex-col bg-gray-950">
      {/* Header */}
      <div className="px-4 py-4 border-b border-gray-800 bg-gray-900/60 shrink-0">
        <div className="flex items-center gap-2">
          <span className="text-xl">{'\u{1F99E}'}</span>
          <div>
            <div className="text-base font-bold text-white">{'AI \u52A9\u624B'}</div>
            <div className="text-[11px] text-gray-500">{'\u9009\u62E9 AI \u56E2\u961F\u5F00\u59CB\u5BF9\u8BDD'}</div>
          </div>
        </div>
      </div>

      <div className="flex-1 overflow-auto">
        {/* Team instances */}
        {instances.length > 0 && (
          <div className="px-4 pt-4">
            <div className="text-[11px] text-gray-500 uppercase tracking-wider mb-2 font-medium">
              {'AI \u56E2\u961F'}
            </div>
            <div className="space-y-2.5">
              {instances.map(inst => {
                const isActive = ['forming', 'ready', 'running'].includes(inst.status)
                return (
                  <button
                    key={inst.id}
                    onClick={() => navigate(`/chat/${inst.id}`)}
                    className={`w-full text-left rounded-xl p-3.5 border transition-all active:scale-[0.98] ${
                      isActive
                        ? 'bg-gray-800/50 border-gray-700/50 hover:border-brand-500/40'
                        : 'bg-gray-800/30 border-gray-700/30 hover:border-gray-600/50'
                    }`}
                  >
                    <div className="flex items-center gap-3">
                      <div className="w-10 h-10 rounded-xl bg-gray-700/40 flex items-center justify-center shrink-0 text-lg">
                        {getIcon(inst.template_name)}
                      </div>
                      <div className="flex-1 min-w-0">
                        <div className="flex items-center gap-2">
                          <div className="text-sm font-medium text-white truncate">{inst.name}</div>
                          <div className={`w-2 h-2 rounded-full shrink-0 ${statusDot[inst.status] || 'bg-gray-500'} ${inst.status === 'running' ? 'animate-pulse' : ''}`} />
                        </div>
                        <div className="text-[11px] text-gray-500 mt-0.5 truncate">
                          {inst.template_name}{' \u00B7 '}{statusLabel[inst.status] || inst.status}
                        </div>
                      </div>
                      <div className="text-right shrink-0">
                        <div className="text-xs text-gray-400 tabular-nums">{inst.mission_count} {'\u4EFB\u52A1'}</div>
                        {inst.energy_used > 0 && (
                          <div className="text-[10px] text-gray-600 tabular-nums">{inst.energy_used.toLocaleString()}{'\u26A1'}</div>
                        )}
                      </div>
                    </div>
                  </button>
                )
              })}
            </div>
          </div>
        )}

        {/* Direct chat entry */}
        <div className="px-4 pt-4 pb-4">
          <div className="text-[11px] text-gray-500 uppercase tracking-wider mb-2 font-medium">
            {'\u901A\u7528\u5BF9\u8BDD'}
          </div>
          <button
            onClick={() => navigate('/chat/direct')}
            className="w-full text-left rounded-xl p-3.5 border border-gray-700/30 bg-gray-800/30 hover:border-brand-500/40 transition-all active:scale-[0.98]"
          >
            <div className="flex items-center gap-3">
              <div className="w-10 h-10 rounded-xl bg-brand-600/10 flex items-center justify-center shrink-0">
                <MessageSquare className="w-5 h-5 text-brand-400" />
              </div>
              <div className="flex-1 min-w-0">
                <div className="text-sm font-medium text-white">{'\u901A\u7528 AI \u52A9\u624B'}</div>
                <div className="text-[11px] text-gray-500 mt-0.5">
                  {'StarClaw AI \u00B7 \u968F\u65F6\u5F00\u59CB\u5BF9\u8BDD'}
                </div>
              </div>
            </div>
          </button>
        </div>

        {/* Empty state when no instances */}
        {instances.length === 0 && (
          <div className="text-center py-6 px-4">
            <div className="text-4xl mb-3">{'\u{1F916}'}</div>
            <div className="text-sm text-gray-300 font-medium">{'\u6682\u65E0 AI \u56E2\u961F'}</div>
            <div className="text-xs text-gray-500 mt-1">
              {'\u7BA1\u7406\u5458\u53EF\u5728\u63A7\u5236\u53F0\u521B\u5EFA AI \u56E2\u961F\uFF0C\u4F60\u4E5F\u53EF\u4EE5\u76F4\u63A5\u4F7F\u7528\u4E0A\u65B9\u901A\u7528\u5BF9\u8BDD'}
            </div>
          </div>
        )}
      </div>
    </div>
  )
}
