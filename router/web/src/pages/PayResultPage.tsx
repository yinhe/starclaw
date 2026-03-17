import { useEffect, useState } from 'react';
import { useSearchParams, Link } from 'react-router-dom';
import { CheckCircle, XCircle, Loader2, ArrowLeft } from 'lucide-react';
import { dash } from '../lib/api';

export default function PayResultPage() {
  const [searchParams] = useSearchParams();
  const [status, setStatus] = useState<'loading' | 'success' | 'pending' | 'failed'>('loading');
  const [orderNo] = useState(searchParams.get('out_trade_no') || '');

  useEffect(() => {
    // Poll order status for a few seconds
    let attempts = 0;
    const maxAttempts = 10;

    const checkOrder = async () => {
      try {
        const res = await dash.orders();
        const orders = res.orders || [];
        const order = orders.find((o: { order_no: string }) => o.order_no === orderNo);
        if (order) {
          if (order.status === 'paid') {
            setStatus('success');
            return;
          } else if (order.status === 'failed') {
            setStatus('failed');
            return;
          }
        }
      } catch {
        // ignore
      }

      attempts++;
      if (attempts >= maxAttempts) {
        // After polling, if still not paid, show pending (Alipay callback may be delayed)
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
            <p className="text-gray-400">余额已到账，可以开始使用了</p>
            {orderNo && <p className="text-gray-500 text-xs font-mono">订单号: {orderNo}</p>}
          </>
        )}

        {status === 'pending' && (
          <>
            <CheckCircle className="w-16 h-16 text-amber-400 mx-auto" />
            <h1 className="text-2xl font-bold text-white">支付处理中</h1>
            <p className="text-gray-400">支付宝正在处理你的订单，余额将在 1-2 分钟内到账</p>
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
