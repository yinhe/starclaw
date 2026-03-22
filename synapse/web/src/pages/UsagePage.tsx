import { useEffect, useState } from 'react';
import { BarChart3 } from 'lucide-react';
import { dash } from '../lib/api';

interface UsageRecord {
  id: string;
  model: string;
  prompt_tokens: number;
  completion_tokens: number;
  total_tokens: number;
  cost_cents: number;
  created_at: string;
}

interface ToolRecord {
  id: string;
  remark: string;
  amount: number; // energy units (1⚡ = 10000)
  created_at: string;
}

export default function UsagePage() {
  const [records, setRecords] = useState<UsageRecord[]>([]);
  const [toolRecords, setToolRecords] = useState<ToolRecord[]>([]);
  const [summary, setSummary] = useState({ total_tokens: 0, total_cost: 0, total_requests: 0 });
  const [days, setDays] = useState(7);
  const [tab, setTab] = useState<'llm' | 'tool'>('llm');

  useEffect(() => {
    dash.usage(days).then(r => {
      setRecords(r.records || []);
      setSummary({ total_tokens: r.total_tokens, total_cost: r.total_cost, total_requests: r.total_requests });
    }).catch(console.error);
    dash.toolUsage(days).then(r => {
      setToolRecords(r.records || []);
    }).catch(console.error);
  }, [days]);

  const parseToolRemark = (remark: string) => {
    // e.g. "image_generation(flux-2-pro) upstream=¥0.040"
    const match = remark.match(/^(\w+)\(([^)]+)\)/);
    if (match) return { tool: match[1], model: match[2] };
    return { tool: remark, model: '' };
  };

  const fmt = (cents: number) => `¥${(cents / 100).toFixed(4)}`;

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-white">用量明细</h1>
          <p className="text-gray-400 text-sm mt-1">API 调用记录和费用</p>
        </div>
        <select
          value={days}
          onChange={e => setDays(Number(e.target.value))}
          className="bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-white text-sm focus:outline-none focus:border-amber-500"
        >
          <option value={1}>今天</option>
          <option value={7}>近 7 天</option>
          <option value={30}>近 30 天</option>
        </select>
      </div>

      {/* Summary */}
      <div className="grid grid-cols-3 gap-4">
        <div className="bg-gray-900 border border-gray-800 rounded-xl p-4 text-center">
          <div className="text-2xl font-bold text-white">{summary.total_requests}</div>
          <div className="text-xs text-gray-400 mt-1">请求次数</div>
        </div>
        <div className="bg-gray-900 border border-gray-800 rounded-xl p-4 text-center">
          <div className="text-2xl font-bold text-white">{(summary.total_tokens / 1000).toFixed(1)}K</div>
          <div className="text-xs text-gray-400 mt-1">总 Tokens</div>
        </div>
        <div className="bg-gray-900 border border-gray-800 rounded-xl p-4 text-center">
          <div className="text-2xl font-bold text-white">{fmt(summary.total_cost)}</div>
          <div className="text-xs text-gray-400 mt-1">总费用</div>
        </div>
      </div>

      {/* Daily usage bar chart */}
      {records.length > 0 && (
        <div className="bg-gray-900 border border-gray-800 rounded-xl p-5">
          <div className="flex items-center gap-2 mb-4">
            <BarChart3 className="w-4 h-4 text-amber-400" />
            <span className="text-sm font-medium text-gray-300">近 {days} 天用量</span>
          </div>
          {(() => {
            // Aggregate tokens by day
            const byDay: Record<string, number> = {};
            const now = new Date();
            for (let i = days - 1; i >= 0; i--) {
              const d = new Date(now);
              d.setDate(d.getDate() - i);
              byDay[d.toISOString().slice(0, 10)] = 0;
            }
            records.forEach(r => {
              const day = r.created_at.slice(0, 10);
              if (day in byDay) byDay[day] += r.total_tokens;
            });
            const entries = Object.entries(byDay);
            const maxVal = Math.max(...entries.map(([, v]) => v), 1);
            return (
              <div className="flex items-end gap-1.5" style={{ height: 120 }}>
                {entries.map(([day, val]) => (
                  <div key={day} className="flex-1 flex flex-col items-center gap-1 h-full justify-end">
                    <span className="text-[10px] text-gray-500">{val > 0 ? (val > 999999 ? `${(val / 1000000).toFixed(1)}M` : val > 999 ? `${(val / 1000).toFixed(0)}K` : val) : ''}</span>
                    <div
                      className="w-full rounded-t bg-gradient-to-t from-amber-600 to-amber-400 min-w-[8px] transition-all duration-300"
                      style={{ height: `${Math.max((val / maxVal) * 90, val > 0 ? 4 : 0)}px` }}
                    />
                    <span className="text-[10px] text-gray-500">{day.slice(5)}</span>
                  </div>
                ))}
              </div>
            );
          })()}
        </div>
      )}

      {/* Tab switcher */}
      <div className="flex gap-2">
        <button
          onClick={() => setTab('llm')}
          className={`px-4 py-2 rounded-lg text-sm font-medium transition-colors ${
            tab === 'llm' ? 'bg-amber-500 text-black' : 'bg-gray-800 text-gray-400 hover:text-white'
          }`}
        >
          大模型对话 ({records.length})
        </button>
        <button
          onClick={() => setTab('tool')}
          className={`px-4 py-2 rounded-lg text-sm font-medium transition-colors ${
            tab === 'tool' ? 'bg-amber-500 text-black' : 'bg-gray-800 text-gray-400 hover:text-white'
          }`}
        >
          工具调用 ({toolRecords.length})
        </button>
      </div>

      {/* Records table */}
      <div className="bg-gray-900 border border-gray-800 rounded-xl overflow-hidden">
        {tab === 'llm' ? (
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-gray-800 text-gray-400 text-left">
              <th className="px-5 py-3 font-medium">时间</th>
              <th className="px-5 py-3 font-medium">模型</th>
              <th className="px-5 py-3 font-medium text-right">输入</th>
              <th className="px-5 py-3 font-medium text-right">输出</th>
              <th className="px-5 py-3 font-medium text-right">费用</th>
            </tr>
          </thead>
          <tbody>
            {records.length === 0 ? (
              <tr>
                <td colSpan={5} className="px-5 py-12 text-center text-gray-500">
                  <BarChart3 className="w-8 h-8 mx-auto mb-2 opacity-30" />
                  暂无调用记录
                </td>
              </tr>
            ) : (
              records.map(r => (
                <tr key={r.id} className="border-b border-gray-800/50 hover:bg-gray-800/30">
                  <td className="px-5 py-3 text-gray-400">{new Date(r.created_at).toLocaleString()}</td>
                  <td className="px-5 py-3">
                    <code className="text-amber-400 text-xs bg-amber-500/10 px-2 py-0.5 rounded">{r.model}</code>
                  </td>
                  <td className="px-5 py-3 text-gray-400 text-right">{r.prompt_tokens}</td>
                  <td className="px-5 py-3 text-gray-400 text-right">{r.completion_tokens}</td>
                  <td className="px-5 py-3 text-white text-right">{fmt(r.cost_cents)}</td>
                </tr>
              ))
            )}
          </tbody>
        </table>
        ) : (
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-gray-800 text-gray-400 text-left">
              <th className="px-5 py-3 font-medium">时间</th>
              <th className="px-5 py-3 font-medium">工具</th>
              <th className="px-5 py-3 font-medium">模型/参数</th>
              <th className="px-5 py-3 font-medium text-right">消耗星能</th>
            </tr>
          </thead>
          <tbody>
            {toolRecords.length === 0 ? (
              <tr>
                <td colSpan={4} className="px-5 py-12 text-center text-gray-500">
                  <BarChart3 className="w-8 h-8 mx-auto mb-2 opacity-30" />
                  暂无工具调用记录
                </td>
              </tr>
            ) : (
              toolRecords.map(r => {
                const info = parseToolRemark(r.remark);
                return (
                  <tr key={r.id} className="border-b border-gray-800/50 hover:bg-gray-800/30">
                    <td className="px-5 py-3 text-gray-400">{new Date(r.created_at).toLocaleString()}</td>
                    <td className="px-5 py-3">
                      <code className="text-purple-400 text-xs bg-purple-500/10 px-2 py-0.5 rounded">{info.tool}</code>
                    </td>
                    <td className="px-5 py-3 text-gray-400">{info.model}</td>
                    <td className="px-5 py-3 text-white text-right">{(r.amount / 10000).toFixed(1)}⚡</td>
                  </tr>
                );
              })
            )}
          </tbody>
        </table>
        )}
      </div>
    </div>
  );
}
