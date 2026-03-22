import { NavLink, Outlet, useNavigate } from 'react-router-dom';
import { LayoutDashboard, Users, FileText, CreditCard, LogOut, Shield, Cpu, Lock } from 'lucide-react';
import { clearToken } from '../lib/api';

const nav = [
  { to: '/', icon: LayoutDashboard, label: '总览' },
  { to: '/users', icon: Users, label: '用户管理' },
  { to: '/logs', icon: FileText, label: '请求日志' },
  { to: '/orders', icon: CreditCard, label: '订单管理' },
  { to: '/providers', icon: Cpu, label: '模型/Provider' },
  { to: '/roles', icon: Lock, label: '角色权限' },
];

export default function Layout() {
  const navigate = useNavigate();

  const logout = () => {
    clearToken();
    navigate('/login');
  };

  return (
    <div className="min-h-screen bg-gray-950 text-gray-100 flex">
      <aside className="w-56 bg-gray-900 border-r border-gray-800 flex flex-col">
        <div className="p-5 flex items-center gap-2.5">
          <div className="w-8 h-8 bg-gradient-to-br from-rose-500 to-pink-600 rounded-lg flex items-center justify-center">
            <Shield className="w-4 h-4 text-white" />
          </div>
          <span className="text-lg font-bold tracking-tight">Star<span className="bg-gradient-to-r from-rose-400 to-pink-400 bg-clip-text text-transparent">AI</span> <span className="text-xs text-gray-500 font-normal">Admin</span></span>
        </div>
        <nav className="flex-1 px-3 space-y-1">
          {nav.map(({ to, icon: Icon, label }) => (
            <NavLink
              key={to}
              to={to}
              end={to === '/'}
              className={({ isActive }) =>
                `flex items-center gap-3 px-3 py-2.5 rounded-lg text-sm transition-colors ${
                  isActive
                    ? 'bg-rose-500/10 text-rose-400 font-medium'
                    : 'text-gray-400 hover:text-gray-200 hover:bg-gray-800'
                }`
              }
            >
              <Icon className="w-4 h-4" />
              {label}
            </NavLink>
          ))}
        </nav>
        <div className="p-3 border-t border-gray-800">
          <button
            onClick={logout}
            className="flex items-center gap-3 px-3 py-2.5 rounded-lg text-sm text-gray-400 hover:text-red-400 hover:bg-gray-800 w-full transition-colors cursor-pointer"
          >
            <LogOut className="w-4 h-4" />
            退出登录
          </button>
        </div>
      </aside>

      <main className="flex-1 overflow-auto">
        <div className="max-w-6xl mx-auto p-8">
          <Outlet />
        </div>
      </main>
    </div>
  );
}
