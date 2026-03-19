import { useEffect, useState } from 'react';
import { Video, Image, Music, Loader2, CheckCircle, XCircle, Clock, RefreshCw } from 'lucide-react';
import { dash } from '../lib/api';
import type { Generation } from '../lib/api';

const statusColors: Record<string, string> = {
  pending: 'text-gray-400',
  running: 'text-blue-400',
  succeeded: 'text-green-400',
  failed: 'text-red-400',
};

const statusIcons: Record<string, React.ReactNode> = {
  pending: <Clock className="w-4 h-4" />,
  running: <Loader2 className="w-4 h-4 animate-spin" />,
  succeeded: <CheckCircle className="w-4 h-4" />,
  failed: <XCircle className="w-4 h-4" />,
};

const typeIcons: Record<string, React.ReactNode> = {
  video: <Video className="w-4 h-4" />,
  image: <Image className="w-4 h-4" />,
  audio: <Music className="w-4 h-4" />,
};

export default function GenerationsPage() {
  const [gens, setGens] = useState<Generation[]>([]);
  const [total, setTotal] = useState(0);
  const [stats, setStats] = useState<{ total: number; running: number; succeeded: number; failed: number; total_cost: number; by_model: { model: string; count: number }[] } | null>(null);
  const [filter, setFilter] = useState<{ status: string; model: string }>({ status: '', model: '' });
  const [loading, setLoading] = useState(true);
  const [offset, setOffset] = useState(0);
  const limit = 20;

  const load = async () => {
    setLoading(true);
    try {
      const [genRes, statsRes] = await Promise.all([
        dash.generations({ status: filter.status || undefined, model: filter.model || undefined, limit, offset }),
        dash.generationStats(),
      ]);
      setGens(genRes.generations || []);
      setTotal(genRes.total);
      setStats(statsRes);
    } catch (e) {
      console.error(e);
    }
    setLoading(false);
  };

  useEffect(() => { load(); }, [filter, offset]);

  // Auto-refresh if there are running tasks
  useEffect(() => {
    const hasRunning = gens.some(g => g.status === 'running' || g.status === 'pending');
    if (!hasRunning) return;
    const timer = setInterval(load, 10000);
    return () => clearInterval(timer);
  }, [gens]);

  const fmtCost = (cents: number) => cents > 0 ? `¥${(cents / 100).toFixed(4)}` : '-';
  const fmtTime = (ts: string | null) => {
    if (!ts) return '-';
    const d = new Date(ts);
    return d.toLocaleString('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' });
  };

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold text-white">Generations</h1>
        <button onClick={load} className="flex items-center gap-2 px-3 py-1.5 bg-gray-800 hover:bg-gray-700 text-gray-300 rounded-lg text-sm transition">
          <RefreshCw className={`w-4 h-4 ${loading ? 'animate-spin' : ''}`} />
          刷新
        </button>
      </div>

      {/* Stats */}
      {stats && (
        <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
          <StatMini label="总任务" value={String(stats.total)} color="text-white" />
          <StatMini label="进行中" value={String(stats.running)} color="text-blue-400" />
          <StatMini label="已完成" value={String(stats.succeeded)} color="text-green-400" />
          <StatMini label="总消费" value={fmtCost(stats.total_cost)} color="text-amber-400" />
        </div>
      )}

      {/* Model breakdown */}
      {stats && stats.by_model && stats.by_model.length > 0 && (
        <div className="flex flex-wrap gap-2">
          <button
            onClick={() => setFilter(f => ({ ...f, model: '' }))}
            className={`px-3 py-1 rounded-full text-xs transition ${!filter.model ? 'bg-indigo-600 text-white' : 'bg-gray-800 text-gray-400 hover:bg-gray-700'}`}
          >
            全部
          </button>
          {stats.by_model.map(m => (
            <button
              key={m.model}
              onClick={() => setFilter(f => ({ ...f, model: f.model === m.model ? '' : m.model }))}
              className={`px-3 py-1 rounded-full text-xs transition ${filter.model === m.model ? 'bg-indigo-600 text-white' : 'bg-gray-800 text-gray-400 hover:bg-gray-700'}`}
            >
              {m.model} ({m.count})
            </button>
          ))}
        </div>
      )}

      {/* Status filter */}
      <div className="flex gap-2">
        {['', 'running', 'succeeded', 'failed'].map(s => (
          <button
            key={s}
            onClick={() => { setFilter(f => ({ ...f, status: s })); setOffset(0); }}
            className={`px-3 py-1 rounded-lg text-xs transition ${filter.status === s ? 'bg-gray-700 text-white' : 'bg-gray-900 text-gray-500 hover:bg-gray-800'}`}
          >
            {s === '' ? '全部' : s === 'running' ? '进行中' : s === 'succeeded' ? '已完成' : '失败'}
          </button>
        ))}
      </div>

      {/* Generations grid */}
      {loading && gens.length === 0 ? (
        <div className="text-center py-12 text-gray-500">
          <Loader2 className="w-6 h-6 animate-spin mx-auto mb-2" />
          加载中...
        </div>
      ) : gens.length === 0 ? (
        <div className="text-center py-12 text-gray-500">
          暂无生成记录
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {gens.map(gen => (
            <div key={gen.id} className="bg-gray-900 border border-gray-800 rounded-xl overflow-hidden">
              {/* Thumbnail / Preview */}
              <div className="aspect-video bg-gray-800 flex items-center justify-center relative">
                {gen.thumbnail_url ? (
                  <img src={gen.thumbnail_url} alt="" className="w-full h-full object-cover" />
                ) : gen.result_url && gen.type === 'video' ? (
                  <video src={gen.result_url} className="w-full h-full object-cover" muted />
                ) : (
                  <div className="text-gray-600">
                    {typeIcons[gen.type] || <Video className="w-8 h-8" />}
                  </div>
                )}
                {/* Status badge */}
                <div className={`absolute top-2 right-2 flex items-center gap-1 px-2 py-0.5 rounded-full text-xs bg-black/60 ${statusColors[gen.status]}`}>
                  {statusIcons[gen.status]}
                  {gen.status}
                </div>
                {/* Model badge */}
                <div className="absolute bottom-2 left-2 px-2 py-0.5 rounded-full text-xs bg-black/60 text-gray-300">
                  {gen.model}
                </div>
                {gen.duration > 0 && (
                  <div className="absolute bottom-2 right-2 px-2 py-0.5 rounded-full text-xs bg-black/60 text-gray-300">
                    {gen.duration}s
                  </div>
                )}
              </div>
              {/* Info */}
              <div className="p-3 space-y-1">
                <p className="text-sm text-gray-300 line-clamp-2">{gen.prompt || '(no prompt)'}</p>
                <div className="flex items-center justify-between text-xs text-gray-500">
                  <span>{fmtTime(gen.created_at)}</span>
                  <span>{fmtCost(gen.cost_cents)}</span>
                </div>
              </div>
            </div>
          ))}
        </div>
      )}

      {/* Pagination */}
      {total > limit && (
        <div className="flex items-center justify-center gap-4">
          <button
            onClick={() => setOffset(Math.max(0, offset - limit))}
            disabled={offset === 0}
            className="px-4 py-1.5 bg-gray-800 text-gray-300 rounded-lg text-sm disabled:opacity-30"
          >
            上一页
          </button>
          <span className="text-sm text-gray-500">
            {offset + 1}-{Math.min(offset + limit, total)} / {total}
          </span>
          <button
            onClick={() => setOffset(offset + limit)}
            disabled={offset + limit >= total}
            className="px-4 py-1.5 bg-gray-800 text-gray-300 rounded-lg text-sm disabled:opacity-30"
          >
            下一页
          </button>
        </div>
      )}
    </div>
  );
}

function StatMini({ label, value, color }: { label: string; value: string; color: string }) {
  return (
    <div className="bg-gray-900 border border-gray-800 rounded-lg p-3">
      <div className="text-xs text-gray-500">{label}</div>
      <div className={`text-lg font-bold ${color}`}>{value}</div>
    </div>
  );
}
