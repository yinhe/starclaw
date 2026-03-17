import { useNavigate, Link } from 'react-router-dom';
import { Zap } from 'lucide-react';

export default function RegisterPage() {
  const navigate = useNavigate();
  return (
    <div className="min-h-screen bg-gray-950 flex items-center justify-center px-4">
      <div className="w-full max-w-sm">
        <div className="text-center mb-8">
          <div className="inline-flex items-center gap-3 mb-2">
            <div className="w-10 h-10 bg-gradient-to-br from-amber-400 to-orange-500 rounded-xl flex items-center justify-center shadow-lg shadow-amber-500/20">
              <Zap className="w-6 h-6 text-white" />
            </div>
            <span className="text-2xl font-bold tracking-tight text-white">Star<span className="bg-gradient-to-r from-amber-400 to-orange-400 bg-clip-text text-transparent">AI</span></span>
          </div>
          <p className="text-gray-400 text-sm">使用 Claw 地址创建并登录账号</p>
        </div>

        <div className="bg-gray-900 border border-gray-800 rounded-xl p-6 space-y-4">
          <h2 className="text-lg font-semibold text-white">Claw 节点注册</h2>

          <div className="bg-amber-500/10 border border-amber-500/20 text-amber-300 text-sm px-4 py-3 rounded-lg leading-relaxed">
            StarAI 现在仅支持通过 Claw 节点地址认证登录，不再支持手机号或邮箱密码注册。
          </div>

          <div className="bg-gray-800 border border-gray-700 rounded-lg p-4 space-y-2 text-sm text-gray-300">
            <p>1. 安装并打开你的 Claw 节点</p>
            <p>2. 在登录页输入你的 Claw 节点地址</p>
            <p>3. 在 Claw 界面确认授权，即可自动创建/登录账号</p>
          </div>

          <button
            type="button"
            onClick={() => navigate('/login')}
            className="w-full bg-amber-500 hover:bg-amber-400 text-gray-900 font-medium py-2.5 rounded-lg text-sm transition-colors cursor-pointer"
          >
            前往 Claw 登录
          </button>

          <a
            href="https://starclaw.net/download"
            className="block w-full text-center border border-gray-700 hover:bg-gray-800 text-gray-300 py-2.5 rounded-lg text-sm transition-colors"
          >
            下载安装 Claw
          </a>

          <p className="text-center text-sm text-gray-500">
            已有 Claw 地址？{' '}
            <Link to="/login" className="text-amber-400 hover:text-amber-300">
              去登录
            </Link>
          </p>
        </div>
      </div>
    </div>
  );
}
