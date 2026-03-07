import { useState } from 'react'
import { Check, Copy, Play, Loader2 } from 'lucide-react'

interface CodeBlockProps {
  children: string
  language?: string
  onRun?: () => Promise<{ stdout: string; stderr: string; exit_code: number; duration: string }>
}

export default function CodeBlock({ children, language, onRun }: CodeBlockProps) {
  const [copied, setCopied] = useState(false)
  const [running, setRunning] = useState(false)
  const [result, setResult] = useState<{ stdout: string; stderr: string; exit_code: number; duration: string } | null>(null)

  const handleCopy = () => {
    navigator.clipboard.writeText(children)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  const handleRun = async () => {
    if (!onRun || running) return
    setRunning(true)
    setResult(null)
    try {
      const res = await onRun()
      setResult(res)
    } catch (e: any) {
      setResult({ stdout: '', stderr: e.message || 'Execution failed', exit_code: -1, duration: '' })
    }
    setRunning(false)
  }

  return (
    <div className="relative group my-2">
      <div className="flex items-center justify-between bg-gray-800 dark:bg-gray-900 text-gray-400 text-xs px-4 py-1.5 rounded-t-lg">
        <span>{language || 'code'}</span>
        <div className="flex items-center gap-3">
          {onRun && (
            <button
              onClick={handleRun}
              disabled={running}
              className={`flex items-center gap-1 transition-colors ${running ? 'text-yellow-400' : 'text-emerald-400 hover:text-emerald-300'}`}
            >
              {running ? (
                <><Loader2 className="w-3 h-3 animate-spin" /> 运行中</>
              ) : (
                <><Play className="w-3 h-3" /> 运行</>
              )}
            </button>
          )}
          <button
            onClick={handleCopy}
            className="flex items-center gap-1 hover:text-white transition-colors"
          >
            {copied ? (
              <><Check className="w-3 h-3" /> 已复制</>
            ) : (
              <><Copy className="w-3 h-3" /> 复制</>
            )}
          </button>
        </div>
      </div>
      <pre className="bg-gray-900 dark:bg-gray-950 text-gray-100 p-4 rounded-b-lg overflow-x-auto text-sm leading-relaxed">
        <code>{children}</code>
      </pre>
      {result && (
        <div className={`border-t ${result.exit_code === 0 ? 'border-emerald-700' : 'border-red-700'} bg-gray-950 rounded-b-lg px-4 py-3 -mt-2`}>
          <div className="flex items-center gap-2 mb-1.5">
            <span className={`text-[10px] font-medium px-1.5 py-0.5 rounded ${result.exit_code === 0 ? 'bg-emerald-900 text-emerald-300' : 'bg-red-900 text-red-300'}`}>
              {result.exit_code === 0 ? '✓ 成功' : `✗ 退出码 ${result.exit_code}`}
            </span>
            {result.duration && <span className="text-[10px] text-gray-500">{result.duration}</span>}
          </div>
          <pre className="text-xs font-mono text-gray-200 whitespace-pre-wrap break-all max-h-48 overflow-y-auto scrollbar-thin">
            {result.stdout || ''}{result.stderr ? `\n${result.stderr}` : ''}
          </pre>
        </div>
      )}
    </div>
  )
}
