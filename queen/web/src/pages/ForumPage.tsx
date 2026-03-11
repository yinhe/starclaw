import { useState, useEffect } from 'react';
import { Link, useSearchParams } from 'react-router-dom';
import { Navbar } from '../components/Navbar';
import { Footer } from '../components/Footer';
import { forumAPI, type ForumPost, type ForumCategory, type ForumReply } from '../lib/api';
import { isLoggedIn, getUser } from '../lib/auth';
import { MessageSquare, ThumbsUp, Eye, Search, ChevronLeft, Send, Pin, Clock, Tag, Flag } from 'lucide-react';
import { ReportDialog } from '../components/ReportDialog';

const CATEGORY_COLORS: Record<string, string> = {
  'agent-tips': 'bg-indigo-50 text-indigo-700 border-indigo-200',
  'workflow-share': 'bg-emerald-50 text-emerald-700 border-emerald-200',
  'tech-discuss': 'bg-amber-50 text-amber-700 border-amber-200',
  'bug-report': 'bg-red-50 text-red-700 border-red-200',
  'feature-request': 'bg-purple-50 text-purple-700 border-purple-200',
  'showcase': 'bg-cyan-50 text-cyan-700 border-cyan-200',
};

function timeAgo(dateStr: string) {
  const d = new Date(dateStr);
  const now = new Date();
  const diff = Math.floor((now.getTime() - d.getTime()) / 1000);
  if (diff < 60) return '刚刚';
  if (diff < 3600) return `${Math.floor(diff / 60)} 分钟前`;
  if (diff < 86400) return `${Math.floor(diff / 3600)} 小时前`;
  if (diff < 604800) return `${Math.floor(diff / 86400)} 天前`;
  return d.toLocaleDateString('zh-CN');
}

export function ForumPage() {
  const [searchParams, setSearchParams] = useSearchParams();
  const postId = searchParams.get('post');
  const catFilter = searchParams.get('category') || '';

  const [categories, setCategories] = useState<ForumCategory[]>([]);
  const [posts, setPosts] = useState<ForumPost[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [searchQuery, setSearchQuery] = useState('');

  // Detail view state
  const [currentPost, setCurrentPost] = useState<ForumPost | null>(null);
  const [replyText, setReplyText] = useState('');
  const [replying, setReplying] = useState(false);

  // New post state
  const [showNewPost, setShowNewPost] = useState(false);
  const [newTitle, setNewTitle] = useState('');
  const [newContent, setNewContent] = useState('');
  const [newCategory, setNewCategory] = useState('');
  const [newTags, setNewTags] = useState('');
  const [posting, setPosting] = useState(false);
  const [reportTarget, setReportTarget] = useState<{ type: string; id: string; title?: string; authorId?: string } | null>(null);

  useEffect(() => {
    forumAPI.categories().then(r => setCategories(r.categories || [])).catch(() => {});
  }, []);

  useEffect(() => {
    if (postId) {
      setLoading(true);
      forumAPI.getPost(postId).then(r => { setCurrentPost(r.post); setLoading(false); }).catch(() => setLoading(false));
    } else {
      setCurrentPost(null);
      setLoading(true);
      forumAPI.listPosts({ category_id: catFilter || undefined }).then(r => {
        setPosts(r.posts || []);
        setTotal(r.total || 0);
        setLoading(false);
      }).catch(() => setLoading(false));
    }
  }, [postId, catFilter]);

  function handleSearch(e: React.FormEvent) {
    e.preventDefault();
    if (!searchQuery.trim()) return;
    setLoading(true);
    forumAPI.search(searchQuery).then(r => {
      setPosts(r.posts || []);
      setTotal(r.total || 0);
      setLoading(false);
    }).catch(() => setLoading(false));
  }

  async function handleCreatePost() {
    const user = getUser();
    if (!user) return;
    setPosting(true);
    try {
      await forumAPI.createPost({
        author_id: user.id,
        author_name: user.nickname || user.email || 'Anonymous',
        category_id: newCategory,
        title: newTitle,
        content: newContent,
        tags: newTags,
      });
      setShowNewPost(false);
      setNewTitle(''); setNewContent(''); setNewCategory(''); setNewTags('');
      // Refresh
      const r = await forumAPI.listPosts({ category_id: catFilter || undefined });
      setPosts(r.posts || []); setTotal(r.total || 0);
    } catch { /* ignore */ }
    setPosting(false);
  }

  async function handleReply() {
    if (!currentPost || !replyText.trim()) return;
    const user = getUser();
    if (!user) return;
    setReplying(true);
    try {
      await forumAPI.createReply(currentPost.id, {
        author_id: user.id,
        author_name: user.nickname || user.email || 'Anonymous',
        content: replyText,
      });
      setReplyText('');
      const r = await forumAPI.getPost(currentPost.id);
      setCurrentPost(r.post);
    } catch { /* ignore */ }
    setReplying(false);
  }

  async function handleLike(id: string) {
    const user = getUser();
    if (!user) return;
    try {
      await forumAPI.likePost(id, { user_id: user.id });
      if (currentPost?.id === id) {
        setCurrentPost({ ...currentPost, like_count: currentPost.like_count + 1 });
      }
    } catch { /* ignore */ }
  }

  // ─── Post Detail View ───
  if (currentPost) {
    return (
      <div className="min-h-screen bg-gray-50">
        <Navbar />
        <div className="max-w-4xl mx-auto px-6 pt-24 pb-16">
          <button onClick={() => setSearchParams({})} className="flex items-center gap-1.5 text-sm text-gray-500 hover:text-indigo-600 mb-6 transition">
            <ChevronLeft className="w-4 h-4" /> 返回论坛
          </button>
          <article className="bg-white rounded-xl border border-gray-200 p-8">
            <div className="flex items-start justify-between mb-4">
              <h1 className="text-2xl font-bold text-gray-900">{currentPost.title}</h1>
              {currentPost.is_pinned && <Pin className="w-4 h-4 text-amber-500 mt-1.5" />}
            </div>
            <div className="flex items-center gap-4 text-sm text-gray-500 mb-6">
              <span className="font-medium text-gray-700">{currentPost.author_name}</span>
              <span className="flex items-center gap-1"><Clock className="w-3.5 h-3.5" />{timeAgo(currentPost.created_at)}</span>
              <span className="flex items-center gap-1"><Eye className="w-3.5 h-3.5" />{currentPost.views}</span>
              <span className="flex items-center gap-1"><ThumbsUp className="w-3.5 h-3.5" />{currentPost.like_count}</span>
            </div>
            {currentPost.tags && (
              <div className="flex gap-2 mb-4">
                {currentPost.tags.split(',').map(t => (
                  <span key={t} className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full bg-gray-100 text-xs text-gray-600">
                    <Tag className="w-3 h-3" />{t.trim()}
                  </span>
                ))}
              </div>
            )}
            <div className="prose prose-gray max-w-none whitespace-pre-wrap">{currentPost.content}</div>
            <div className="mt-6 flex gap-3">
              <button onClick={() => handleLike(currentPost.id)} className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg border text-sm text-gray-600 hover:text-indigo-600 hover:border-indigo-300 transition">
                <ThumbsUp className="w-4 h-4" /> 点赞
              </button>
              {isLoggedIn() && (
                <button onClick={() => setReportTarget({ type: 'forum_post', id: currentPost.id, title: currentPost.title, authorId: currentPost.author_id })}
                  className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg border text-sm text-gray-400 hover:text-red-500 hover:border-red-300 transition">
                  <Flag className="w-4 h-4" /> 举报
                </button>
              )}
            </div>
          </article>

          {/* Replies */}
          <div className="mt-8">
            <h3 className="text-lg font-bold mb-4">回复 ({currentPost.replies?.length || currentPost.reply_count || 0})</h3>
            <div className="space-y-4">
              {(currentPost.replies || []).map((r: ForumReply) => (
                <div key={r.id} className="bg-white rounded-xl border border-gray-200 p-5">
                  <div className="flex items-center gap-3 mb-2">
                    <div className="w-8 h-8 rounded-full bg-indigo-100 text-indigo-600 flex items-center justify-center text-sm font-bold">
                      {r.author_name.charAt(0).toUpperCase()}
                    </div>
                    <span className="font-medium text-sm">{r.author_name}</span>
                    <span className="text-xs text-gray-400">{timeAgo(r.created_at)}</span>
                  </div>
                  <div className="flex items-center justify-between">
                    <p className="text-gray-700 text-sm whitespace-pre-wrap flex-1">{r.content}</p>
                    {isLoggedIn() && (
                      <button onClick={() => setReportTarget({ type: 'forum_reply', id: r.id, title: r.content.slice(0, 50), authorId: r.author_id })}
                        className="ml-2 p-1 rounded text-gray-300 hover:text-red-400 transition" title="举报">
                        <Flag className="w-3.5 h-3.5" />
                      </button>
                    )}
                  </div>
                </div>
              ))}
            </div>
            {isLoggedIn() && (
              <div className="mt-6 flex gap-3">
                <input
                  value={replyText} onChange={e => setReplyText(e.target.value)}
                  placeholder="写下你的回复..."
                  className="flex-1 px-4 py-2.5 rounded-lg border border-gray-200 text-sm focus:outline-none focus:border-indigo-400"
                  onKeyDown={e => e.key === 'Enter' && !e.shiftKey && handleReply()}
                />
                <button onClick={handleReply} disabled={replying || !replyText.trim()}
                  className="px-4 py-2.5 rounded-lg bg-indigo-500 text-white text-sm font-medium hover:bg-indigo-600 disabled:opacity-50 transition flex items-center gap-1.5">
                  <Send className="w-4 h-4" /> 回复
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

  // ─── Post List View ───
  return (
    <div className="min-h-screen bg-gray-50">
      <Navbar />
      <div className="max-w-6xl mx-auto px-6 pt-24 pb-16">
        {/* Header */}
        <div className="flex items-center justify-between mb-8">
          <div>
            <h1 className="text-3xl font-bold text-gray-900">社区论坛</h1>
            <p className="text-gray-500 mt-1">分享 Agent 玩法、工作流模板、技术讨论</p>
          </div>
          {isLoggedIn() && (
            <button onClick={() => setShowNewPost(true)}
              className="px-5 py-2.5 rounded-lg bg-indigo-500 text-white text-sm font-medium hover:bg-indigo-600 transition shadow-sm">
              发帖
            </button>
          )}
        </div>

        {/* Search + Categories */}
        <div className="flex flex-col md:flex-row gap-4 mb-6">
          <form onSubmit={handleSearch} className="flex-1 relative">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-400" />
            <input value={searchQuery} onChange={e => setSearchQuery(e.target.value)}
              placeholder="搜索帖子..." className="w-full pl-10 pr-4 py-2.5 rounded-lg border border-gray-200 text-sm focus:outline-none focus:border-indigo-400" />
          </form>
          <div className="flex gap-2 flex-wrap">
            <button onClick={() => { setSearchParams({}); setSearchQuery(''); }}
              className={`px-3 py-1.5 rounded-lg text-xs font-medium border transition ${!catFilter ? 'bg-indigo-50 text-indigo-700 border-indigo-200' : 'bg-white text-gray-600 border-gray-200 hover:border-indigo-200'}`}>
              全部
            </button>
            {categories.map(cat => (
              <button key={cat.id} onClick={() => setSearchParams({ category: cat.id })}
                className={`px-3 py-1.5 rounded-lg text-xs font-medium border transition ${catFilter === cat.id ? (CATEGORY_COLORS[cat.slug] || 'bg-gray-50 text-gray-700 border-gray-200') : 'bg-white text-gray-600 border-gray-200 hover:border-indigo-200'}`}>
                {cat.icon} {cat.name} <span className="text-gray-400 ml-1">{cat.post_count}</span>
              </button>
            ))}
          </div>
        </div>

        {/* Post List */}
        {loading ? (
          <div className="text-center py-16 text-gray-400">加载中...</div>
        ) : posts.length === 0 ? (
          <div className="text-center py-16">
            <MessageSquare className="w-12 h-12 text-gray-300 mx-auto mb-3" />
            <p className="text-gray-400">暂无帖子</p>
          </div>
        ) : (
          <div className="space-y-3">
            {posts.map(post => (
              <Link key={post.id} to={`/forum?post=${post.id}`}
                className="block bg-white rounded-xl border border-gray-200 p-5 hover:border-indigo-200 hover:shadow-sm transition">
                <div className="flex items-start justify-between">
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2 mb-1.5">
                      {post.is_pinned && <Pin className="w-3.5 h-3.5 text-amber-500" />}
                      <h3 className="font-semibold text-gray-900 truncate">{post.title}</h3>
                    </div>
                    <p className="text-sm text-gray-500 line-clamp-2">{post.content}</p>
                    <div className="flex items-center gap-4 mt-3 text-xs text-gray-400">
                      <span className="font-medium text-gray-600">{post.author_name}</span>
                      <span>{timeAgo(post.created_at)}</span>
                      <span className="flex items-center gap-1"><Eye className="w-3 h-3" />{post.views}</span>
                      <span className="flex items-center gap-1"><MessageSquare className="w-3 h-3" />{post.reply_count}</span>
                      <span className="flex items-center gap-1"><ThumbsUp className="w-3 h-3" />{post.like_count}</span>
                    </div>
                  </div>
                </div>
              </Link>
            ))}
            {total > posts.length && (
              <p className="text-center text-sm text-gray-400 pt-4">共 {total} 篇帖子</p>
            )}
          </div>
        )}

        {/* New Post Modal */}
        {showNewPost && (
          <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40" onClick={() => setShowNewPost(false)}>
            <div className="bg-white rounded-2xl w-full max-w-xl p-6 shadow-xl" onClick={e => e.stopPropagation()}>
              <h2 className="text-lg font-bold mb-4">发表新帖</h2>
              <input value={newTitle} onChange={e => setNewTitle(e.target.value)} placeholder="标题"
                className="w-full px-4 py-2.5 rounded-lg border border-gray-200 text-sm mb-3 focus:outline-none focus:border-indigo-400" />
              <select value={newCategory} onChange={e => setNewCategory(e.target.value)}
                className="w-full px-4 py-2.5 rounded-lg border border-gray-200 text-sm mb-3 focus:outline-none focus:border-indigo-400 bg-white">
                <option value="">选择板块（可选）</option>
                {categories.map(c => <option key={c.id} value={c.id}>{c.icon} {c.name}</option>)}
              </select>
              <textarea value={newContent} onChange={e => setNewContent(e.target.value)} placeholder="正文内容..." rows={8}
                className="w-full px-4 py-2.5 rounded-lg border border-gray-200 text-sm mb-3 focus:outline-none focus:border-indigo-400 resize-none" />
              <input value={newTags} onChange={e => setNewTags(e.target.value)} placeholder="标签（逗号分隔，可选）"
                className="w-full px-4 py-2.5 rounded-lg border border-gray-200 text-sm mb-4 focus:outline-none focus:border-indigo-400" />
              <div className="flex justify-end gap-3">
                <button onClick={() => setShowNewPost(false)} className="px-4 py-2 rounded-lg border text-sm text-gray-600 hover:bg-gray-50">取消</button>
                <button onClick={handleCreatePost} disabled={posting || !newTitle.trim() || !newContent.trim()}
                  className="px-5 py-2 rounded-lg bg-indigo-500 text-white text-sm font-medium hover:bg-indigo-600 disabled:opacity-50 transition">
                  {posting ? '发布中...' : '发布'}
                </button>
              </div>
            </div>
          </div>
        )}
      </div>
      <Footer />
    </div>
  );
}
