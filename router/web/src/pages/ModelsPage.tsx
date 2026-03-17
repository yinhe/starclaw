import { useEffect, useState } from 'react';
import { Cpu, Search, Zap, Globe, MessageSquare, Image, Video, Music, Brain } from 'lucide-react';
import { models, type ModelInfo } from '../lib/api';

const typeIcons: Record<string, React.ReactNode> = {
  chat: <MessageSquare className="w-4 h-4" />,
  reasoning: <Brain className="w-4 h-4" />,
  embedding: <Zap className="w-4 h-4" />,
  image: <Image className="w-4 h-4" />,
  video: <Video className="w-4 h-4" />,
  tts: <Music className="w-4 h-4" />,
  stt: <Music className="w-4 h-4" />,
};

const typeLabels: Record<string, string> = {
  chat: '对话',
  reasoning: '推理',
  embedding: '向量',
  image: '图片',
  video: '视频',
  tts: '语音合成',
  stt: '语音识别',
  image_edit: '图片编辑',
};

const providerColors: Record<string, string> = {
  openai: 'bg-green-500/10 text-green-400 border-green-500/20',
  anthropic: 'bg-orange-500/10 text-orange-400 border-orange-500/20',
  qwen: 'bg-blue-500/10 text-blue-400 border-blue-500/20',
  deepseek: 'bg-cyan-500/10 text-cyan-400 border-cyan-500/20',
  google: 'bg-red-500/10 text-red-400 border-red-500/20',
  grok: 'bg-purple-500/10 text-purple-400 border-purple-500/20',
  fal: 'bg-pink-500/10 text-pink-400 border-pink-500/20',
};

export default function ModelsPage() {
  const [allModels, setAllModels] = useState<ModelInfo[]>([]);
  const [search, setSearch] = useState('');
  const [typeFilter, setTypeFilter] = useState('all');
  const [providerFilter, setProviderFilter] = useState('all');
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    models.list()
      .then(r => setAllModels(r.data || []))
      .catch(console.error)
      .finally(() => setLoading(false));
  }, []);

  const providers = [...new Set(allModels.map(m => m.owned_by))].sort();
  const types = [...new Set(allModels.map(m => m.type).filter(Boolean))].sort();

  const filtered = allModels.filter(m => {
    if (search && !m.id.toLowerCase().includes(search.toLowerCase()) && !m.owned_by.toLowerCase().includes(search.toLowerCase())) return false;
    if (typeFilter !== 'all' && m.type !== typeFilter) return false;
    if (providerFilter !== 'all' && m.owned_by !== providerFilter) return false;
    return true;
  });

  const fmtPrice = (price?: number) => {
    if (!price) return '-';
    if (price < 0.01) return `$${price.toFixed(4)}`;
    return `$${price.toFixed(2)}`;
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="animate-spin w-8 h-8 border-2 border-amber-500 border-t-transparent rounded-full" />
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold text-white">模型目录</h1>
        <p className="text-gray-400 text-sm mt-1">
          {allModels.length} 个模型 · {providers.length} 家提供商 · OpenAI 兼容 API
        </p>
      </div>

      {/* Filters */}
      <div className="flex items-center gap-3">
        <div className="flex-1 relative">
          <Search className="w-4 h-4 text-gray-500 absolute left-3 top-1/2 -translate-y-1/2" />
          <input
            value={search}
            onChange={e => setSearch(e.target.value)}
            placeholder="搜索模型名称或提供商..."
            className="w-full bg-gray-900 border border-gray-800 rounded-lg pl-10 pr-4 py-2.5 text-white text-sm focus:outline-none focus:border-amber-500 placeholder-gray-600"
          />
        </div>
        <select
          value={providerFilter}
          onChange={e => setProviderFilter(e.target.value)}
          className="bg-gray-900 border border-gray-800 rounded-lg px-3 py-2.5 text-white text-sm focus:outline-none focus:border-amber-500"
        >
          <option value="all">全部提供商</option>
          {providers.map(p => <option key={p} value={p}>{p}</option>)}
        </select>
        <select
          value={typeFilter}
          onChange={e => setTypeFilter(e.target.value)}
          className="bg-gray-900 border border-gray-800 rounded-lg px-3 py-2.5 text-white text-sm focus:outline-none focus:border-amber-500"
        >
          <option value="all">全部类型</option>
          {types.map(t => <option key={t} value={t}>{typeLabels[t!] || t}</option>)}
        </select>
      </div>

      {/* Provider summary */}
      <div className="flex flex-wrap gap-2">
        {providers.map(p => {
          const count = allModels.filter(m => m.owned_by === p).length;
          const colors = providerColors[p] || 'bg-gray-500/10 text-gray-400 border-gray-500/20';
          return (
            <button
              key={p}
              onClick={() => setProviderFilter(providerFilter === p ? 'all' : p)}
              className={`inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs border transition ${
                providerFilter === p ? colors + ' ring-1 ring-current' : colors + ' opacity-70 hover:opacity-100'
              }`}
            >
              <Globe className="w-3 h-3" />
              {p}
              <span className="opacity-60">({count})</span>
            </button>
          );
        })}
      </div>

      {/* Models grid */}
      {filtered.length === 0 ? (
        <div className="bg-gray-900 border border-gray-800 border-dashed rounded-xl p-12 text-center">
          <Cpu className="w-10 h-10 text-gray-600 mx-auto mb-3" />
          <p className="text-gray-500">没有匹配的模型</p>
        </div>
      ) : (
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
          {filtered.map(m => {
            const colors = providerColors[m.owned_by] || 'bg-gray-500/10 text-gray-400 border-gray-500/20';
            return (
              <div key={m.id} className="bg-gray-900 border border-gray-800 rounded-xl p-4 hover:border-gray-700 transition group">
                <div className="flex items-start justify-between mb-2">
                  <div className="flex items-center gap-2 min-w-0">
                    <div className={`w-7 h-7 rounded-lg flex items-center justify-center shrink-0 ${colors.split(' ').slice(0, 1).join(' ')}`}>
                      {typeIcons[m.type || 'chat'] || <Cpu className="w-4 h-4" />}
                    </div>
                    <div className="min-w-0">
                      <div className="text-sm font-medium text-white truncate">{m.id}</div>
                    </div>
                  </div>
                </div>
                <div className="flex items-center gap-2 mb-3">
                  <span className={`text-[10px] px-1.5 py-0.5 rounded border ${colors}`}>{m.owned_by}</span>
                  {m.type && (
                    <span className="text-[10px] px-1.5 py-0.5 rounded bg-gray-800 text-gray-400">
                      {typeLabels[m.type] || m.type}
                    </span>
                  )}
                  {m.context_length && m.context_length > 0 && (
                    <span className="text-[10px] text-gray-500">{(m.context_length / 1000).toFixed(0)}K ctx</span>
                  )}
                </div>
                {(m.input_price || m.output_price) ? (
                  <div className="flex items-center gap-4 text-xs text-gray-500">
                    <span>输入: <span className="text-gray-300">{fmtPrice(m.input_price)}/M</span></span>
                    <span>输出: <span className="text-gray-300">{fmtPrice(m.output_price)}/M</span></span>
                  </div>
                ) : (
                  <div className="text-xs text-gray-600">按次计费</div>
                )}
              </div>
            );
          })}
        </div>
      )}

      {/* Usage hint */}
      <div className="bg-gray-900 border border-gray-800 rounded-xl p-5">
        <h3 className="text-sm font-medium text-white mb-3">如何使用</h3>
        <div className="bg-gray-800 rounded-lg p-3 font-mono text-xs">
          <div className="text-gray-500"># 使用模型 ID 作为 model 参数</div>
          <div className="text-amber-400 mt-1">
            curl https://api.star-ai.net/v1/chat/completions \
          </div>
          <div className="text-amber-400 pl-4">
            -H "Authorization: Bearer sk-star-xxx" \
          </div>
          <div className="text-amber-400 pl-4">
            -d '{`{"model":"qwen/qwen-turbo","messages":[{"role":"user","content":"你好"}]}`}'
          </div>
        </div>
      </div>
    </div>
  );
}
