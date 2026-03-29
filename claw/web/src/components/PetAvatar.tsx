import { useMemo } from 'react'

interface PetAvatarProps {
  path: 'abyss' | 'terrain' | 'sky' | 'larva'
  formCode: string
  level: number
  size?: 'sm' | 'md' | 'lg' | 'battle'
  animation?: 'idle' | 'attack' | 'hit' | 'crit' | 'skill' | 'death' | 'win'
  className?: string
}

const sizeMap = { sm: 64, md: 80, lg: 200, battle: 300 }

const pathColors: Record<string, { bg: string; ring: string; glow: string }> = {
  abyss:   { bg: 'from-blue-900/30 to-cyan-900/20', ring: 'ring-blue-400/50',   glow: 'shadow-blue-500/30' },
  terrain: { bg: 'from-amber-900/30 to-orange-900/20', ring: 'ring-amber-400/50', glow: 'shadow-amber-500/30' },
  sky:     { bg: 'from-purple-900/30 to-pink-900/20', ring: 'ring-purple-400/50', glow: 'shadow-purple-500/30' },
  larva:   { bg: 'from-gray-800/30 to-gray-700/20', ring: 'ring-gray-400/50',    glow: 'shadow-gray-500/30' },
}

const pathEmojis: Record<string, string> = {
  abyss: '🌊',
  terrain: '🏔️',
  sky: '🌪️',
  larva: '🥚',
}

const formEmojis: Record<string, string> = {
  claw: '🦞', octopus: '🐙', jiao: '🐉', kun: '🐋', leviathan: '🐲', abyssal: '👑',
  zergling: '🦗', hydralisk: '🦂', lurker: '🕷️', ultralisk: '🦬', titan: '⚔️', colossus: '🏛️',
  pterosaur: '🦅', argentavis: '🦤', mutalisk: '🦇', peng: '🕊️', guardian: '🛡️', skyward: '✨',
}

export default function PetAvatar({ path, formCode, level, size = 'md', animation = 'idle', className = '' }: PetAvatarProps) {
  const px = sizeMap[size]
  const colors = pathColors[path] || pathColors.larva
  const emoji = formEmojis[formCode] || '🦞'
  const pathEmoji = pathEmojis[path] || '🥚'

  const animClass = useMemo(() => {
    switch (animation) {
      case 'attack': return 'animate-bounce'
      case 'hit': return 'animate-pulse'
      case 'crit': return 'animate-ping'
      case 'win': return 'animate-spin'
      default: return ''
    }
  }, [animation])

  const levelTier = level >= 50 ? 'legendary' : level >= 30 ? 'epic' : level >= 20 ? 'rare' : level >= 10 ? 'uncommon' : 'common'

  return (
    <div
      className={`relative inline-flex items-center justify-center rounded-full bg-gradient-to-br ${colors.bg} ring-2 ${colors.ring} shadow-lg ${colors.glow} ${animClass} ${className}`}
      style={{ width: px, height: px }}
    >
      {/* Main creature emoji */}
      <span
        className="select-none"
        style={{ fontSize: px * 0.5 }}
        role="img"
        aria-label={formCode}
      >
        {emoji}
      </span>

      {/* Path badge (top-right) */}
      {size !== 'sm' && (
        <span
          className="absolute -top-1 -right-1 text-xs bg-gray-800/80 rounded-full px-1 border border-gray-600/50"
          style={{ fontSize: Math.max(10, px * 0.12) }}
        >
          {pathEmoji}
        </span>
      )}

      {/* Level badge (bottom-center) */}
      <span
        className={`absolute -bottom-1 left-1/2 -translate-x-1/2 text-[10px] font-bold px-1.5 py-0.5 rounded-full border ${
          levelTier === 'legendary' ? 'bg-yellow-500/90 text-black border-yellow-300' :
          levelTier === 'epic' ? 'bg-purple-500/90 text-white border-purple-300' :
          levelTier === 'rare' ? 'bg-blue-500/90 text-white border-blue-300' :
          levelTier === 'uncommon' ? 'bg-green-500/90 text-white border-green-300' :
          'bg-gray-600/90 text-white border-gray-400'
        }`}
        style={{ fontSize: Math.max(9, px * 0.1) }}
      >
        Lv.{level}
      </span>
    </div>
  )
}
