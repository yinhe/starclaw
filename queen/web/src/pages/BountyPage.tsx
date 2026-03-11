import { useState, useEffect } from 'react';
import { useSearchParams } from 'react-router-dom';
import { Navbar } from '../components/Navbar';
import { Footer } from '../components/Footer';
import { bountyAPI, type Bounty } from '../lib/api';
import { isLoggedIn, getUser } from '../lib/auth';
import { Target, ChevronLeft, Clock, User, CheckCircle, AlertTriangle, Package, Send, Filter, Flag } from 'lucide-react';
import { ReportDialog } from '../components/ReportDialog';

const CATEGORY_LABELS: Record<string, { label: string; icon: string }> = {
  physical_delivery: { label: '实物交付', icon: '📦' },
  human_judgment: { label: '人类判断', icon: '🧠' },
  creative_review: { label: '创意审核', icon: '🎨' },
  data_collection: { label: '数据收集', icon: '📊' },
  real_world_verification: { label: '现实验证', icon: '🔍' },
  specialized_skill: { label: '专业技能', icon: '⚡' },
  other: { label: '其他', icon: '📋' },
};

const STATUS_STYLES: Record<string, { label: string; class: string }> = {
  open: { label: '开放', class: 'bg-emerald-50 text-emerald-700 border-emerald-200' },
  claimed: { label: '已领取', class: 'bg-blue-50 text-blue-700 border-blue-200' },
  delivered: { label: '已交付', class: 'bg-amber-50 text-amber-700 border-amber-200' },
  completed: { label: '已完成', class: 'bg-green-50 text-green-700 border-green-200' },
  disputed: { label: '争议中', class: 'bg-red-50 text-red-700 border-red-200' },
  cancelled: { label: '已取消', class: 'bg-gray-50 text-gray-500 border-gray-200' },
  expired: { label: '已过期', class: 'bg-gray-50 text-gray-400 border-gray-200' },
};

function timeAgo(dateStr: string) {
  if (!dateStr) return '';
  const d = new Date(dateStr);
  const now = new Date();
  const diff = Math.floor((now.getTime() - d.getTime()) / 1000);
  if (diff < 60) return '刚刚';
  if (diff < 3600) return `${Math.floor(diff / 60)} 分钟前`;
  if (diff < 86400) return `${Math.floor(diff / 3600)} 小时前`;
  if (diff < 604800) return `${Math.floor(diff / 86400)} 天前`;
  return d.toLocaleDateString('zh-CN');
}

function formatReward(amount: number, currency: string) {
  if (currency === 'CNY' || currency === 'cny') return `¥${(amount / 100).toFixed(2)}`;
  return `${amount} ${currency}`;
}

export function BountyPage() {
  const [searchParams, setSearchParams] = useSearchParams();
  const bountyId = searchParams.get('id');
  const catFilter = searchParams.get('category') || '';
  const statusFilter = searchParams.get('status') || '';

  const [bounties, setBounties] = useState<Bounty[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [stats, setStats] = useState<{ total: number; open: number; completed: number; total_reward: number } | null>(null);

  // Detail
  const [currentBounty, setCurrentBounty] = useState<Bounty | null>(null);
  const [deliveryNotes, setDeliveryNotes] = useState('');
  const [disputeReason, setDisputeReason] = useState('');
  const [actionLoading, setActionLoading] = useState(false);
  const [actionMsg, setActionMsg] = useState('');
  const [reportTarget, setReportTarget] = useState<{ type: string; id: string; title?: string; authorId?: string } | null>(null);

  useEffect(() => {
    bountyAPI.stats().then(setStats).catch(() => {});
  }, []);

  useEffect(() => {
    if (bountyId) {
      setLoading(true);
      bountyAPI.get(bountyId).then(r => { setCurrentBounty(r.bounty); setLoading(false); }).catch(() => setLoading(false));
    } else {
      setCurrentBounty(null);
      setLoading(true);
      bountyAPI.list({ category: catFilter || undefined, status: statusFilter || undefined }).then(r => {
        setBounties(r.bounties || []);
        setTotal(r.total || 0);
        setLoading(false);
      }).catch(() => setLoading(false));
    }
  }, [bountyId, catFilter, statusFilter]);

  async function doAction(action: 'claim' | 'deliver' | 'accept' | 'cancel' | 'dispute') {
    if (!currentBounty) return;
    const user = getUser();
    setActionLoading(true);
    setActionMsg('');
    try {
      switch (action) {
        case 'claim':
          if (!user) throw new Error('请先登录');
          await bountyAPI.claim(currentBounty.id, { user_id: user.id, user_name: user.nickname || user.email || 'User' });
          break;
        case 'deliver':
          await bountyAPI.deliver(currentBounty.id, { delivery_notes: deliveryNotes });
          break;
        case 'accept':
          await bountyAPI.accept(currentBounty.id);
          break;
        case 'cancel':
          await bountyAPI.cancel(currentBounty.id);
          break;
        case 'dispute':
          await bountyAPI.dispute(currentBounty.id, { reason: disputeReason });
          break;
      }
      setActionMsg('操作成功');
      const r = await bountyAPI.get(currentBounty.id);
      setCurrentBounty(r.bounty);
    } catch (e: any) {
      setActionMsg(e.message || '操作失败');
    }
    setActionLoading(false);
  }

  // ─── Detail View ───
  if (currentBounty) {
    const st = STATUS_STYLES[currentBounty.status] || STATUS_STYLES.open;
    const cat = CATEGORY_LABELS[currentBounty.category] || CATEGORY_LABELS.other;
    const user = getUser();
    const isCreator = user?.id === currentBounty.creator_id;
    const isClaimer = user?.id === currentBounty.claimed_by;

    return (
      <div className="min-h-screen bg-gray-50">
        <Navbar />
        <div className="max-w-4xl mx-auto px-6 pt-24 pb-16">
          <button onClick={() => setSearchParams({})} className="flex items-center gap-1.5 text-sm text-gray-500 hover:text-indigo-600 mb-6 transition">
            <ChevronLeft className="w-4 h-4" /> 返回赏金市场
          </button>

          <div className="bg-white rounded-xl border border-gray-200 p-8">
            <div className="flex items-start justify-between mb-4">
              <div>
                <div className="flex items-center gap-2 mb-2">
                  <span className={`px-2.5 py-0.5 rounded-full text-xs font-medium border ${st.class}`}>{st.label}</span>
                  <span className="px-2 py-0.5 rounded-full bg-gray-100 text-xs text-gray-600">{cat.icon} {cat.label}</span>
                </div>
                <h1 className="text-2xl font-bold text-gray-900">{currentBounty.title}</h1>
              </div>
              <div className="text-right">
                <p className="text-2xl font-bold text-emerald-600">{formatReward(currentBounty.reward_amount, currentBounty.reward_currency)}</p>
                <p className="text-xs text-gray-400 mt-1">赏金</p>
              </div>
            </div>

            <div className="flex items-center gap-4 text-sm text-gray-500 mb-6">
              <span className="flex items-center gap-1"><User className="w-3.5 h-3.5" />发布者: {currentBounty.creator_name}</span>
              <span className="flex items-center gap-1"><Clock className="w-3.5 h-3.5" />{timeAgo(currentBounty.created_at)}</span>
              {currentBounty.deadline && <span className="flex items-center gap-1"><AlertTriangle className="w-3.5 h-3.5" />截止: {new Date(currentBounty.deadline).toLocaleDateString('zh-CN')}</span>}
            </div>

            <div className="prose prose-gray max-w-none whitespace-pre-wrap mb-6">{currentBounty.description}</div>

            {currentBounty.claimed_by_name && (
              <div className="bg-blue-50 border border-blue-200 rounded-lg px-4 py-3 mb-4 text-sm text-blue-700">
                <strong>领取者:</strong> {currentBounty.claimed_by_name}
              </div>
            )}
            {currentBounty.delivery_notes && (
              <div className="bg-amber-50 border border-amber-200 rounded-lg px-4 py-3 mb-4 text-sm text-amber-700">
                <strong>交付说明:</strong> {currentBounty.delivery_notes}
              </div>
            )}

            {actionMsg && <div className="mb-4 text-sm text-indigo-600">{actionMsg}</div>}

            {/* Action buttons based on state */}
            {isLoggedIn() && (
              <div className="flex flex-wrap gap-3 pt-4 border-t border-gray-100">
                {currentBounty.status === 'open' && !isCreator && (
                  <button onClick={() => doAction('claim')} disabled={actionLoading}
                    className="px-5 py-2 rounded-lg bg-indigo-500 text-white text-sm font-medium hover:bg-indigo-600 disabled:opacity-50 transition flex items-center gap-1.5">
                    <Package className="w-4 h-4" /> 领取任务
                  </button>
                )}
                {currentBounty.status === 'claimed' && isClaimer && (
                  <div className="flex-1 flex gap-3">
                    <input value={deliveryNotes} onChange={e => setDeliveryNotes(e.target.value)} placeholder="交付说明..."
                      className="flex-1 px-4 py-2 rounded-lg border border-gray-200 text-sm focus:outline-none focus:border-indigo-400" />
                    <button onClick={() => doAction('deliver')} disabled={actionLoading || !deliveryNotes.trim()}
                      className="px-5 py-2 rounded-lg bg-emerald-500 text-white text-sm font-medium hover:bg-emerald-600 disabled:opacity-50 transition flex items-center gap-1.5">
                      <Send className="w-4 h-4" /> 提交交付
                    </button>
                  </div>
                )}
                {currentBounty.status === 'delivered' && isCreator && (
                  <>
                    <button onClick={() => doAction('accept')} disabled={actionLoading}
                      className="px-5 py-2 rounded-lg bg-green-500 text-white text-sm font-medium hover:bg-green-600 disabled:opacity-50 transition flex items-center gap-1.5">
                      <CheckCircle className="w-4 h-4" /> 验收通过
                    </button>
                    <button onClick={() => {
                      const reason = prompt('请输入争议原因');
                      if (reason) { setDisputeReason(reason); doAction('dispute'); }
                    }} disabled={actionLoading}
                      className="px-5 py-2 rounded-lg border border-red-300 text-red-600 text-sm font-medium hover:bg-red-50 disabled:opacity-50 transition flex items-center gap-1.5">
                      <AlertTriangle className="w-4 h-4" /> 发起争议
                    </button>
                  </>
                )}
                {['open', 'claimed'].includes(currentBounty.status) && isCreator && (
                  <button onClick={() => doAction('cancel')} disabled={actionLoading}
                    className="px-4 py-2 rounded-lg border text-gray-500 text-sm hover:bg-gray-50 disabled:opacity-50 transition">
                    取消赏金
                  </button>
                )}
                <button onClick={() => setReportTarget({ type: 'bounty', id: currentBounty.id, title: currentBounty.title, authorId: currentBounty.creator_id })}
                  className="px-4 py-2 rounded-lg border text-gray-400 text-sm hover:text-red-500 hover:border-red-300 transition flex items-center gap-1.5">
                  <Flag className="w-4 h-4" /> 举报
                </button>
              </div>
            )}
          </div>
        </div>
        {reportTarget && (
          <ReportDialog
            targetType={reportTarget.type}
            targetId={reportTarget.id}
            targetTitle={reportTarget.title}
            authorId={reportTarget.authorId}
            onClose={() => setReportTarget(null)}
          />
        )}
        <Footer />
      </div>
    );
  }

  // ─── List View ───
  return (
    <div className="min-h-screen bg-gray-50">
      <Navbar />
      <div className="max-w-6xl mx-auto px-6 pt-24 pb-16">
        {/* Header */}
        <div className="mb-8">
          <div className="flex items-center gap-3 mb-2">
            <div className="w-10 h-10 rounded-xl bg-gradient-to-br from-emerald-500 to-teal-500 flex items-center justify-center">
              <Target className="w-5 h-5 text-white" />
            </div>
            <h1 className="text-3xl font-bold text-gray-900">赏金市场</h1>
          </div>
          <p className="text-gray-500">AI 做不了的事，悬赏让人类来做 — 反向众包</p>
        </div>

        {/* Stats */}
        {stats && (
          <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mb-8">
            <div className="bg-white rounded-xl border p-4">
              <p className="text-2xl font-bold text-gray-900">{stats.total}</p>
              <p className="text-xs text-gray-500 mt-0.5">总赏金任务</p>
            </div>
            <div className="bg-white rounded-xl border p-4">
              <p className="text-2xl font-bold text-emerald-600">{stats.open}</p>
              <p className="text-xs text-gray-500 mt-0.5">开放中</p>
            </div>
            <div className="bg-white rounded-xl border p-4">
              <p className="text-2xl font-bold text-green-600">{stats.completed}</p>
              <p className="text-xs text-gray-500 mt-0.5">已完成</p>
            </div>
            <div className="bg-white rounded-xl border p-4">
              <p className="text-2xl font-bold text-amber-600">¥{((stats.total_reward || 0) / 100).toFixed(0)}</p>
              <p className="text-xs text-gray-500 mt-0.5">总赏金池</p>
            </div>
          </div>
        )}

        {/* Filters */}
        <div className="flex flex-col md:flex-row gap-4 mb-6">
          <div className="flex gap-2 flex-wrap">
            <Filter className="w-4 h-4 text-gray-400 mt-1.5" />
            <button onClick={() => setSearchParams({})}
              className={`px-3 py-1.5 rounded-lg text-xs font-medium border transition ${!statusFilter && !catFilter ? 'bg-indigo-50 text-indigo-700 border-indigo-200' : 'bg-white text-gray-600 border-gray-200'}`}>
              全部
            </button>
            {Object.entries(STATUS_STYLES).slice(0, 4).map(([k, v]) => (
              <button key={k} onClick={() => setSearchParams({ status: k })}
                className={`px-3 py-1.5 rounded-lg text-xs font-medium border transition ${statusFilter === k ? v.class : 'bg-white text-gray-600 border-gray-200'}`}>
                {v.label}
              </button>
            ))}
          </div>
          <div className="flex gap-2 flex-wrap">
            {Object.entries(CATEGORY_LABELS).map(([k, v]) => (
              <button key={k} onClick={() => setSearchParams({ category: k })}
                className={`px-3 py-1.5 rounded-lg text-xs font-medium border transition ${catFilter === k ? 'bg-gray-100 text-gray-800 border-gray-300' : 'bg-white text-gray-500 border-gray-200'}`}>
                {v.icon} {v.label}
              </button>
            ))}
          </div>
        </div>

        {/* Bounty List */}
        {loading ? (
          <div className="text-center py-16 text-gray-400">加载中...</div>
        ) : bounties.length === 0 ? (
          <div className="text-center py-16">
            <Target className="w-12 h-12 text-gray-300 mx-auto mb-3" />
            <p className="text-gray-400">暂无赏金任务</p>
          </div>
        ) : (
          <div className="grid md:grid-cols-2 gap-4">
            {bounties.map(b => {
              const st = STATUS_STYLES[b.status] || STATUS_STYLES.open;
              const cat = CATEGORY_LABELS[b.category] || CATEGORY_LABELS.other;
              return (
                <button key={b.id} onClick={() => setSearchParams({ id: b.id })}
                  className="text-left bg-white rounded-xl border border-gray-200 p-5 hover:border-indigo-200 hover:shadow-sm transition">
                  <div className="flex items-center justify-between mb-2">
                    <div className="flex items-center gap-2">
                      <span className={`px-2 py-0.5 rounded-full text-[10px] font-medium border ${st.class}`}>{st.label}</span>
                      <span className="text-xs text-gray-400">{cat.icon} {cat.label}</span>
                    </div>
                    <span className="text-lg font-bold text-emerald-600">{formatReward(b.reward_amount, b.reward_currency)}</span>
                  </div>
                  <h3 className="font-semibold text-gray-900 mb-1.5 truncate">{b.title}</h3>
                  <p className="text-sm text-gray-500 line-clamp-2 mb-3">{b.description}</p>
                  <div className="flex items-center gap-4 text-xs text-gray-400">
                    <span className="flex items-center gap-1"><User className="w-3 h-3" />{b.creator_name}</span>
                    <span className="flex items-center gap-1"><Clock className="w-3 h-3" />{timeAgo(b.created_at)}</span>
                  </div>
                </button>
              );
            })}
          </div>
        )}
        {total > bounties.length && (
          <p className="text-center text-sm text-gray-400 pt-6">共 {total} 个赏金任务</p>
        )}
      </div>
      <Footer />
    </div>
  );
}
