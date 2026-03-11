import { useState, useEffect } from 'react';
import { reportAPI, type ReportReason } from '../lib/api';
import { Flag, X } from 'lucide-react';

interface ReportDialogProps {
  targetType: string;
  targetId: string;
  targetTitle?: string;
  authorId?: string;
  onClose: () => void;
}

export function ReportDialog({ targetType, targetId, targetTitle, authorId, onClose }: ReportDialogProps) {
  const [reasons, setReasons] = useState<ReportReason[]>([]);
  const [selectedReason, setSelectedReason] = useState('');
  const [detail, setDetail] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [result, setResult] = useState<{ ok: boolean; msg: string } | null>(null);

  useEffect(() => {
    reportAPI.reasons().then(r => setReasons(r.data?.reasons || [])).catch(() => {
      setReasons([
        { id: 'spam', label: '垃圾信息' },
        { id: 'abuse', label: '辱骂/人身攻击' },
        { id: 'nsfw', label: '不当内容' },
        { id: 'illegal', label: '违法违规' },
        { id: 'other', label: '其他' },
      ]);
    });
  }, []);

  async function handleSubmit() {
    if (!selectedReason) return;
    setSubmitting(true);
    try {
      await reportAPI.create({
        target_type: targetType,
        target_id: targetId,
        target_title: targetTitle,
        author_id: authorId,
        reason: selectedReason,
        detail: detail || undefined,
      });
      setResult({ ok: true, msg: '举报已提交，我们会尽快处理' });
    } catch (e: any) {
      setResult({ ok: false, msg: e.message || '提交失败' });
    }
    setSubmitting(false);
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40" onClick={onClose}>
      <div className="bg-white rounded-2xl shadow-xl w-full max-w-md mx-4 p-6" onClick={e => e.stopPropagation()}>
        <div className="flex items-center justify-between mb-4">
          <div className="flex items-center gap-2">
            <Flag className="w-5 h-5 text-red-500" />
            <h3 className="font-bold text-lg">举报内容</h3>
          </div>
          <button onClick={onClose} className="p-1 rounded-lg hover:bg-gray-100 transition">
            <X className="w-5 h-5 text-gray-400" />
          </button>
        </div>

        {result ? (
          <div className={`p-4 rounded-xl text-sm ${result.ok ? 'bg-green-50 text-green-700' : 'bg-red-50 text-red-600'}`}>
            {result.msg}
            <button onClick={onClose} className="block mt-3 text-xs underline opacity-70">关闭</button>
          </div>
        ) : (
          <>
            {targetTitle && (
              <p className="text-sm text-gray-500 mb-4 truncate">举报对象：{targetTitle}</p>
            )}

            <p className="text-sm font-medium text-gray-700 mb-2">举报原因</p>
            <div className="flex flex-wrap gap-2 mb-4">
              {reasons.map(r => (
                <button key={r.id} onClick={() => setSelectedReason(r.id)}
                  className={`px-3 py-1.5 rounded-full text-sm border transition ${
                    selectedReason === r.id
                      ? 'border-red-400 bg-red-50 text-red-600 font-medium'
                      : 'border-gray-200 text-gray-500 hover:border-red-300'
                  }`}>
                  {r.label}
                </button>
              ))}
            </div>

            <p className="text-sm font-medium text-gray-700 mb-2">补充说明（可选）</p>
            <textarea
              value={detail} onChange={e => setDetail(e.target.value)}
              rows={3}
              className="w-full px-4 py-3 rounded-xl border border-gray-200 text-sm focus:outline-none focus:ring-2 focus:ring-red-400 transition resize-none"
              placeholder="请描述具体问题..."
            />

            <div className="flex gap-2 mt-4">
              <button onClick={handleSubmit} disabled={!selectedReason || submitting}
                className="px-5 py-2.5 rounded-xl bg-red-500 text-white text-sm font-medium hover:bg-red-600 disabled:opacity-50 transition">
                {submitting ? '提交中...' : '提交举报'}
              </button>
              <button onClick={onClose}
                className="px-5 py-2.5 rounded-xl border text-sm text-gray-500 hover:bg-gray-50 transition">
                取消
              </button>
            </div>
          </>
        )}
      </div>
    </div>
  );
}
