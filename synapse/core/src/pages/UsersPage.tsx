import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { Search } from 'lucide-react';
import { admin, type User } from '../lib/api';

export default function UsersPage() {
  const navigate = useNavigate();
  const [users, setUsers] = useState<User[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [pages, setPages] = useState(1);
  const [query, setQuery] = useState('');
  const [search, setSearch] = useState('');

  useEffect(() => {
    admin.users({ page, q: search, page_size: 20 }).then((res) => {
      setUsers(res.users || []);
      setTotal(res.total);
      setPages(res.pages);
    });
  }, [page, search]);

  const handleSearch = (e: React.FormEvent) => {
    e.preventDefault();
    setPage(1);
    setSearch(query);
  };

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-bold">用户管理 <span className="text-sm font-normal text-gray-500">({total})</span></h1>
        <form onSubmit={handleSearch} className="flex gap-2">
          <div className="relative">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-500" />
            <input
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder="搜索邮箱/姓名"
              className="bg-gray-900 border border-gray-800 rounded-lg pl-9 pr-4 py-2 text-sm text-white placeholder-gray-600 focus:outline-none focus:border-rose-500/50 w-60"
            />
          </div>
          <button type="submit" className="bg-rose-600 hover:bg-rose-700 text-white text-sm px-4 py-2 rounded-lg cursor-pointer">搜索</button>
        </form>
      </div>

      <div className="bg-gray-900 border border-gray-800 rounded-xl overflow-hidden">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-gray-800 text-gray-400">
              <th className="text-left px-4 py-3 font-medium">邮箱</th>
              <th className="text-left px-4 py-3 font-medium">姓名</th>
              <th className="text-right px-4 py-3 font-medium">余额</th>
              <th className="text-center px-4 py-3 font-medium">状态</th>
              <th className="text-center px-4 py-3 font-medium">Admin</th>
              <th className="text-left px-4 py-3 font-medium">注册时间</th>
            </tr>
          </thead>
          <tbody>
            {users.map((u) => (
              <tr
                key={u.id}
                onClick={() => navigate(`/users/${u.id}`)}
                className="border-b border-gray-800/50 hover:bg-gray-800/50 cursor-pointer transition-colors"
              >
                <td className="px-4 py-3 text-white">{u.email}</td>
                <td className="px-4 py-3 text-gray-300">{u.name}</td>
                <td className="px-4 py-3 text-right text-gray-300">¥{(u.balance / 100).toFixed(2)}</td>
                <td className="px-4 py-3 text-center">
                  <span className={`inline-block px-2 py-0.5 rounded text-xs ${u.status === 'active' ? 'bg-emerald-500/10 text-emerald-400' : 'bg-red-500/10 text-red-400'}`}>
                    {u.status}
                  </span>
                </td>
                <td className="px-4 py-3 text-center">
                  {u.is_admin && <span className="inline-block px-2 py-0.5 rounded text-xs bg-rose-500/10 text-rose-400">Admin</span>}
                </td>
                <td className="px-4 py-3 text-gray-500">{new Date(u.created_at).toLocaleDateString()}</td>
              </tr>
            ))}
            {users.length === 0 && (
              <tr><td colSpan={6} className="px-4 py-8 text-center text-gray-500">暂无数据</td></tr>
            )}
          </tbody>
        </table>
      </div>

      {pages > 1 && (
        <div className="flex items-center justify-center gap-2 mt-4">
          <button onClick={() => setPage(Math.max(1, page - 1))} disabled={page <= 1} className="px-3 py-1.5 rounded bg-gray-800 text-gray-300 text-sm disabled:opacity-40 cursor-pointer">上一页</button>
          <span className="text-sm text-gray-500">{page} / {pages}</span>
          <button onClick={() => setPage(Math.min(pages, page + 1))} disabled={page >= pages} className="px-3 py-1.5 rounded bg-gray-800 text-gray-300 text-sm disabled:opacity-40 cursor-pointer">下一页</button>
        </div>
      )}
    </div>
  );
}
