import { Link, useLocation } from 'react-router-dom';
import { LogoFull } from './Logo';
import { isLoggedIn } from '../lib/auth';
import { Github } from 'lucide-react';

export function Navbar() {
  const logged = isLoggedIn();
  const { pathname } = useLocation();

  const navLink = (to: string, label: string) => (
    <Link to={to} className={`hover:text-indigo-600 transition ${pathname.startsWith(to) ? 'text-indigo-600 font-semibold' : ''}`}>{label}</Link>
  );

  return (
    <nav className="fixed top-0 inset-x-0 z-50 glass border-b border-gray-200/50">
      <div className="max-w-7xl mx-auto px-6 h-16 flex items-center justify-between">
        <Link to="/">
          <LogoFull />
        </Link>
        <div className="hidden md:flex items-center gap-6 text-sm font-medium text-gray-600">
          {navLink('/marketplace', '市场')}
          {navLink('/forum', '论坛')}
          {navLink('/arena', '龙虾社区')}
          {navLink('/growth', '成长')}
          {navLink('/chrysalis', '化蛹PK')}
          {navLink('/bounty', '赏金')}
          {navLink('/cloud', '云船队')}
          {navLink('/docs', '文档')}
        </div>
        <div className="flex items-center gap-3">
          <a href="https://github.com/yinhe/starclaw" target="_blank" rel="noreferrer" className="hidden sm:flex items-center gap-1.5 text-sm text-gray-600 hover:text-gray-900 transition">
            <Github className="w-5 h-5" />
          </a>
          {logged ? (
            <div className="flex items-center gap-2">
              <Link to="/billing" className="text-sm text-gray-600 hover:text-indigo-600 font-medium transition">充值</Link>
              <Link to="/dashboard" className="px-4 py-2 rounded-lg bg-indigo-500 text-white text-sm font-medium hover:bg-indigo-600 transition shadow-sm shadow-indigo-500/25">
                用户后台
              </Link>
            </div>
          ) : (
            <>
              <Link to="/auth" className="text-sm text-gray-600 hover:text-gray-900 font-medium transition">登录</Link>
              <Link to="/auth?tab=register" className="px-4 py-2 rounded-lg bg-indigo-500 text-white text-sm font-medium hover:bg-indigo-600 transition shadow-sm shadow-indigo-500/25">
                免费注册
              </Link>
            </>
          )}
        </div>
      </div>
    </nav>
  );
}
