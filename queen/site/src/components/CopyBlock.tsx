import { useState } from 'react'
import { Copy, Check } from 'lucide-react'

export function CopyBlock({ text, className = '' }: { text: string; className?: string }) {
  const [copied, setCopied] = useState(false)

  const handleCopy = () => {
    navigator.clipboard.writeText(text)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  return (
    <div className={`group relative rounded-lg bg-gray-900 p-4 font-mono text-sm text-gray-300 overflow-x-auto ${className}`}>
      <button
        onClick={handleCopy}
        className="absolute top-3 right-3 p-1.5 rounded-md bg-white/5 text-gray-500 hover:text-white hover:bg-white/10 transition-colors opacity-0 group-hover:opacity-100"
        title="Copy"
      >
        {copied ? <Check size={14} className="text-green-400" /> : <Copy size={14} />}
      </button>
      <pre className="whitespace-pre-wrap break-all">{text}</pre>
    </div>
  )
}
