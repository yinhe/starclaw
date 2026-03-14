import { useState } from 'react'
import { Book, Settings, Cpu, Plug, GitBranch, Shield, ChevronRight } from 'lucide-react'
import { Layout } from '../components/Layout'
import { useI18n } from '../i18n'
import { getDocsContent, SECTION_IDS } from '../docs-content'

const SECTION_ICONS: Record<string, React.ComponentType<{ size?: number }>> = {
  quickstart: Book,
  configuration: Settings,
  models: Cpu,
  tools: Plug,
  update: GitBranch,
  security: Shield,
}

function renderMarkdown(md: string) {
  const lines = md.split('\n')
  const elements: React.ReactNode[] = []
  let inCode = false
  let codeLines: string[] = []

  lines.forEach((line, i) => {
    if (line.startsWith('```')) {
      if (inCode) {
        elements.push(
          <pre key={`code-${i}`} className="rounded-lg bg-gray-900 p-4 font-mono text-sm text-gray-300 overflow-x-auto my-4">
            {codeLines.join('\n')}
          </pre>
        )
        codeLines = []
      }
      inCode = !inCode
      return
    }
    if (inCode) {
      codeLines.push(line)
      return
    }
    if (line.startsWith('## ')) {
      elements.push(<h2 key={i} className="text-2xl font-bold text-white mt-8 mb-4">{line.slice(3)}</h2>)
    } else if (line.startsWith('### ')) {
      elements.push(<h3 key={i} className="text-lg font-semibold text-white mt-6 mb-3">{line.slice(4)}</h3>)
    } else if (line.startsWith('- **')) {
      const match = line.match(/^- \*\*(.+?)\*\*\s*[—–-]\s*(.+)$/)
      if (match) {
        elements.push(
          <div key={i} className="flex gap-2 text-sm text-gray-400 my-1.5 ml-4">
            <span className="text-gray-600">&bull;</span>
            <span><span className="text-white font-medium">{match[1]}</span> — {match[2]}</span>
          </div>
        )
      } else {
        elements.push(<div key={i} className="text-sm text-gray-400 my-1.5 ml-4">&bull; {line.slice(2)}</div>)
      }
    } else if (line.startsWith('- ')) {
      elements.push(<div key={i} className="text-sm text-gray-400 my-1.5 ml-4">&bull; {line.slice(2)}</div>)
    } else if (line.startsWith('| ') && line.includes('|')) {
      const cells = line.split('|').filter(Boolean).map(c => c.trim())
      if (cells.every(c => /^[-:]+$/.test(c))) return
      const isHeader = i > 0 && lines[i + 1]?.includes('---')
      elements.push(
        <div key={i} className={`grid gap-4 text-sm py-2 border-b border-white/5 ${isHeader ? 'text-gray-500 font-medium' : 'text-gray-400'}`}
          style={{ gridTemplateColumns: `repeat(${cells.length}, 1fr)` }}
        >
          {cells.map((c, j) => <div key={j}>{c.replace(/`/g, '')}</div>)}
        </div>
      )
    } else if (line.trim() === '') {
      elements.push(<div key={i} className="h-2" />)
    } else {
      const rendered = line.replace(/\*\*(.+?)\*\*/g, '<strong class="text-white">$1</strong>')
        .replace(/`(.+?)`/g, '<code class="text-claw-400 bg-white/5 px-1.5 py-0.5 rounded text-xs">$1</code>')
        .replace(/\[(.+?)\]\((.+?)\)/g, '<a href="$2" target="_blank" rel="noopener noreferrer" class="text-claw-400 hover:underline">$1</a>')
      elements.push(<p key={i} className="text-sm text-gray-400 leading-relaxed my-1" dangerouslySetInnerHTML={{ __html: rendered }} />)
    }
  })

  return <>{elements}</>
}

export function DocsPage() {
  const [active, setActive] = useState('quickstart')
  const { t, locale } = useI18n()
  const docs = getDocsContent(locale)
  const section = docs[active]

  return (
    <Layout>
      <section className="py-12">
        <div className="mx-auto max-w-6xl px-6">
          <div className="flex flex-col md:flex-row gap-8">
            {/* Sidebar */}
            <aside className="md:w-56 shrink-0">
              <div className="sticky top-24 space-y-1">
                <div className="text-xs font-semibold text-gray-500 uppercase tracking-wider mb-3 px-3">
                  {t('docs.title')}
                </div>
                {SECTION_IDS.map((id) => {
                  const Icon = SECTION_ICONS[id]
                  return (
                    <button
                      key={id}
                      onClick={() => setActive(id)}
                      className={`w-full flex items-center gap-2.5 rounded-lg px-3 py-2 text-sm font-medium transition-colors text-left ${
                        active === id
                          ? 'bg-claw-500/10 text-claw-400'
                          : 'text-gray-400 hover:text-white hover:bg-white/5'
                      }`}
                    >
                      <Icon size={16} />
                      {docs[id].title}
                      {active === id && <ChevronRight size={14} className="ml-auto" />}
                    </button>
                  )
                })}
              </div>
            </aside>

            {/* Content */}
            <div className="flex-1 min-w-0">
              <div className="rounded-xl border border-white/10 bg-white/[0.02] p-8 md:p-10">
                {renderMarkdown(section.content)}
              </div>
            </div>
          </div>
        </div>
      </section>
    </Layout>
  )
}
