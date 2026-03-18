import { useEffect, useState } from 'react';
import { useSearchParams, Link, useNavigate } from 'react-router-dom';
import { CheckCircle, XCircle, Loader2, ArrowLeft } from 'lucide-react';
import { dash } from '../lib/api';

export default function PayResultPage() {
  const [searchParams] = useSearchParams();
  const navigate = useNavigate();
  const [status, setStatus] = useState<'loading' | 'success' | 'pending' | 'failed'>('loading');
  const [orderNo] = useState(searchParams.get('out_trade_no') || '');
  const [countdown, setCountdown] = useState(0);

  useEffect(() => {
    // Poll order status for a few seconds
    let attempts = 0;
    const maxAttempts = 10;

    const checkOrder = async () => {
      try {
        // Active query: backend checks Alipay/WeChat for real-time trade status
        const res = await dash.queryOrder(orderNo);
        if (res.status === 'paid') {
          setStatus('success');
          return;
        } else if (res.status === 'failed') {
          setStatus('failed');
          return;
        }
      } catch {
        // ignore — may not be logged in yet
      }

      attempts++;
      if (attempts >= maxAttempts) {
        setStatus('pending');
      } else {
        setTimeout(checkOrder, 2000);
      }
    };

    if (orderNo) {
      checkOrder();
    } else {
      // No order_no in URL params, likely just navigated here directly
      setStatus('pending');
    }
  }, [orderNo]);

  // Auto-redirect after status is determined
  useEffect(() => {
    if (status === 'success') {
      setCountdown(3);
      const t = setInterval(() => setCountdown(c => {
        if (c <= 1) { clearInterval(t); navigate('/dashboard'); }
        return c - 1;
      }), 1000);
      return () => clearInterval(t);
    }
    if (status === 'pending') {
      setCountdown(5);
      const t = setInterval(() => setCountdown(c => {
        if (c <= 1) { clearInterval(t); navigate('/billing'); }
        return c - 1;
      }), 1000);
      return () => clearInterval(t);
    }
  }, [status, navigate]);

  return (
    <div className="min-h-screen bg-gray-950 flex items-center justify-center px-4">
      <div className="max-w-md w-full text-center space-y-6">
        {status === 'loading' && (
          <>
            <Loader2 className="w-16 h-16 text-amber-400 mx-auto animate-spin" />
            <h1 className="text-2xl font-bold text-white">确认支付结果...</h1>
            <p className="text-gray-400">正在查询订单状态，请稍候</p>
          </>
        )}

        {status === 'success' && (
          <>
            <CheckCircle className="w-16 h-16 text-green-400 mx-auto" />
            <h1 className="text-2xl font-bold text-white">充值成功</h1>
            <p className="text-gray-400">余额已到账，{countdown > 0 ? `${countdown} 秒后自动跳转...` : '正在跳转...'}</p>
            {orderNo && <p className="text-gray-500 text-xs font-mono">订单号: {orderNo}</p>}
          </>
        )}

        {status === 'pending' && (
          <>
            <CheckCircle className="w-16 h-16 text-amber-400 mx-auto" />
            <h1 className="text-2xl font-bold text-white">支付处理中</h1>
            <p className="text-gray-400">支付平台正在处理你的订单，{countdown > 0 ? `${countdown} 秒后跳转充值页...` : '正在跳转...'}</p>
            {orderNo && <p className="text-gray-500 text-xs font-mono">订单号: {orderNo}</p>}
          </>
        )}

        {status === 'failed' && (
          <>
            <XCircle className="w-16 h-16 text-red-400 mx-auto" />
            <h1 className="text-2xl font-bold text-white">支付失败</h1>
            <p className="text-gray-400">订单未完成，请重新尝试</p>
            {orderNo && <p className="text-gray-500 text-xs font-mono">订单号: {orderNo}</p>}
          </>
        )}

        <div className="flex gap-3 justify-center pt-4">
          <Link
            to="/billing"
            className="inline-flex items-center gap-2 px-5 py-2.5 bg-gray-800 hover:bg-gray-700 text-white rounded-lg text-sm transition-colors"
          >
            <ArrowLeft className="w-4 h-4" />
            返回充值
          </Link>
          <Link
            to="/dashboard"
            className="inline-flex items-center gap-2 px-5 py-2.5 bg-amber-500 hover:bg-amber-400 text-black font-medium rounded-lg text-sm transition-colors"
          >
            进入控制台
          </Link>
        </div>
      </div>
    </div>
  );
}
