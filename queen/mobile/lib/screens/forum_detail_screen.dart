import 'package:flutter/material.dart';
import '../services/api_service.dart';
import '../services/auth_service.dart';
import '../theme.dart';

class ForumDetailScreen extends StatefulWidget {
  final String postId;
  const ForumDetailScreen({super.key, required this.postId});

  @override
  State<ForumDetailScreen> createState() => _ForumDetailScreenState();
}

class _ForumDetailScreenState extends State<ForumDetailScreen> {
  Map<String, dynamic>? _post;
  bool _loading = true;
  final _replyCtrl = TextEditingController();
  bool _replying = false;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    try {
      final res = await ApiService().getForumPost(widget.postId);
      if (mounted) {
        setState(() {
          _post = res.data['post'];
          _loading = false;
        });
      }
    } catch (_) {
      if (mounted) setState(() => _loading = false);
    }
  }

  Future<void> _submitReply() async {
    final text = _replyCtrl.text.trim();
    if (text.isEmpty) return;
    final auth = AuthService();
    if (!auth.isLoggedIn) return;
    setState(() => _replying = true);
    try {
      await ApiService().createForumReply(
        widget.postId,
        authorId: auth.userId ?? '',
        authorName: auth.username ?? 'User',
        content: text,
      );
      _replyCtrl.clear();
      if (mounted) FocusScope.of(context).unfocus();
      await _load();
    } catch (_) {}
    if (mounted) setState(() => _replying = false);
  }

  Future<void> _likePost() async {
    final auth = AuthService();
    if (!auth.isLoggedIn) return;
    try {
      await ApiService().likeForumPost(widget.postId, auth.userId ?? '');
      await _load();
    } catch (_) {}
  }

  String _timeAgo(String? dateStr) {
    if (dateStr == null) return '';
    final d = DateTime.tryParse(dateStr);
    if (d == null) return '';
    final diff = DateTime.now().difference(d).inSeconds;
    if (diff < 60) return '刚刚';
    if (diff < 3600) return '${diff ~/ 60}分钟前';
    if (diff < 86400) return '${diff ~/ 3600}小时前';
    return '${diff ~/ 86400}天前';
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('帖子详情'),
        actions: [
          if (_post != null)
            IconButton(
              icon: const Icon(Icons.flag_outlined, size: 20),
              onPressed: () => _showReportSheet(
                'forum_post',
                widget.postId,
                _post?['title'],
                _post?['author_id'],
              ),
              tooltip: '举报',
            ),
        ],
      ),
      body: _loading
          ? const Center(child: CircularProgressIndicator())
          : _post == null
          ? const Center(
              child: Text('加载失败', style: TextStyle(color: Colors.grey)),
            )
          : Column(
              children: [
                Expanded(child: _buildContent()),
                if (AuthService().isLoggedIn) _buildReplyBar(),
              ],
            ),
    );
  }

  Widget _buildContent() {
    final p = _post!;
    final replies = (p['replies'] as List?) ?? [];

    return RefreshIndicator(
      onRefresh: _load,
      child: ListView(
        padding: const EdgeInsets.all(20),
        children: [
          // Title
          Text(
            p['title'] ?? '',
            style: const TextStyle(
              color: Colors.white,
              fontSize: 20,
              fontWeight: FontWeight.bold,
            ),
          ),
          const SizedBox(height: 12),

          // Author + meta
          Row(
            children: [
              Container(
                width: 32,
                height: 32,
                decoration: BoxDecoration(
                  color: AppTheme.primaryColor.withValues(alpha: 0.2),
                  borderRadius: BorderRadius.circular(10),
                ),
                child: Center(
                  child: Text(
                    (p['author_name'] ?? 'U').toString().isNotEmpty
                        ? p['author_name'][0].toUpperCase()
                        : 'U',
                    style: const TextStyle(
                      color: AppTheme.primaryColor,
                      fontSize: 14,
                      fontWeight: FontWeight.bold,
                    ),
                  ),
                ),
              ),
              const SizedBox(width: 10),
              Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    p['author_name'] ?? '',
                    style: const TextStyle(
                      color: Colors.white,
                      fontSize: 14,
                      fontWeight: FontWeight.w500,
                    ),
                  ),
                  Text(
                    _timeAgo(p['created_at']),
                    style: TextStyle(color: Colors.grey[600], fontSize: 11),
                  ),
                ],
              ),
            ],
          ),
          const SizedBox(height: 16),

          // Tags
          if (p['tags'] != null && (p['tags'] as String).isNotEmpty) ...[
            Wrap(
              spacing: 6,
              children: (p['tags'] as String)
                  .split(',')
                  .map(
                    (t) => Container(
                      padding: const EdgeInsets.symmetric(
                        horizontal: 8,
                        vertical: 3,
                      ),
                      decoration: BoxDecoration(
                        color: const Color(0xFF334155),
                        borderRadius: BorderRadius.circular(6),
                      ),
                      child: Text(
                        t.trim(),
                        style: TextStyle(color: Colors.grey[400], fontSize: 11),
                      ),
                    ),
                  )
                  .toList(),
            ),
            const SizedBox(height: 16),
          ],

          // Content
          Container(
            padding: const EdgeInsets.all(16),
            decoration: BoxDecoration(
              color: const Color(0xFF1E293B),
              borderRadius: BorderRadius.circular(14),
            ),
            child: Text(
              p['content'] ?? '',
              style: TextStyle(
                color: Colors.grey[300],
                fontSize: 14,
                height: 1.6,
              ),
            ),
          ),
          const SizedBox(height: 12),

          // Like + stats
          Row(
            children: [
              GestureDetector(
                onTap: _likePost,
                child: Container(
                  padding: const EdgeInsets.symmetric(
                    horizontal: 12,
                    vertical: 6,
                  ),
                  decoration: BoxDecoration(
                    color: const Color(0xFF1E293B),
                    borderRadius: BorderRadius.circular(20),
                  ),
                  child: Row(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      const Icon(
                        Icons.thumb_up_outlined,
                        size: 16,
                        color: AppTheme.primaryColor,
                      ),
                      const SizedBox(width: 6),
                      Text(
                        '${p['like_count'] ?? 0}',
                        style: const TextStyle(
                          color: AppTheme.primaryColor,
                          fontSize: 13,
                        ),
                      ),
                    ],
                  ),
                ),
              ),
              const SizedBox(width: 12),
              Icon(
                Icons.remove_red_eye_outlined,
                size: 14,
                color: Colors.grey[600],
              ),
              const SizedBox(width: 4),
              Text(
                '${p['views'] ?? p['view_count'] ?? 0}',
                style: TextStyle(color: Colors.grey[500], fontSize: 12),
              ),
              const SizedBox(width: 12),
              Icon(
                Icons.chat_bubble_outline,
                size: 14,
                color: Colors.grey[600],
              ),
              const SizedBox(width: 4),
              Text(
                '${replies.length}',
                style: TextStyle(color: Colors.grey[500], fontSize: 12),
              ),
            ],
          ),
          const SizedBox(height: 24),

          // Replies
          Text(
            '回复 (${replies.length})',
            style: const TextStyle(
              color: Colors.white,
              fontSize: 16,
              fontWeight: FontWeight.w600,
            ),
          ),
          const SizedBox(height: 12),

          if (replies.isEmpty)
            Container(
              padding: const EdgeInsets.all(24),
              decoration: BoxDecoration(
                color: const Color(0xFF1E293B),
                borderRadius: BorderRadius.circular(14),
              ),
              child: Center(
                child: Text(
                  '暂无回复',
                  style: TextStyle(color: Colors.grey[600], fontSize: 13),
                ),
              ),
            )
          else
            ...replies.map((r) => _buildReplyCard(r as Map<String, dynamic>)),
        ],
      ),
    );
  }

  Widget _buildReplyCard(Map<String, dynamic> r) {
    return Container(
      margin: const EdgeInsets.only(bottom: 8),
      padding: const EdgeInsets.all(14),
      decoration: BoxDecoration(
        color: const Color(0xFF1E293B),
        borderRadius: BorderRadius.circular(12),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Container(
                width: 26,
                height: 26,
                decoration: BoxDecoration(
                  color: AppTheme.accentColor.withValues(alpha: 0.2),
                  borderRadius: BorderRadius.circular(8),
                ),
                child: Center(
                  child: Text(
                    (r['author_name'] ?? 'U').toString().isNotEmpty
                        ? r['author_name'][0].toUpperCase()
                        : 'U',
                    style: const TextStyle(
                      color: AppTheme.accentColor,
                      fontSize: 11,
                      fontWeight: FontWeight.bold,
                    ),
                  ),
                ),
              ),
              const SizedBox(width: 8),
              Text(
                r['author_name'] ?? '',
                style: const TextStyle(
                  color: Colors.white,
                  fontSize: 13,
                  fontWeight: FontWeight.w500,
                ),
              ),
              const Spacer(),
              Text(
                _timeAgo(r['created_at']),
                style: TextStyle(color: Colors.grey[600], fontSize: 11),
              ),
              if (AuthService().isLoggedIn) ...[
                const SizedBox(width: 8),
                GestureDetector(
                  onTap: () => _showReportSheet(
                    'forum_reply',
                    r['id'],
                    (r['content'] as String?)?.substring(
                      0,
                      (r['content'] as String?)?.length.clamp(0, 50) ?? 0,
                    ),
                    r['author_id'],
                  ),
                  child: Icon(
                    Icons.flag_outlined,
                    size: 14,
                    color: Colors.grey[700],
                  ),
                ),
              ],
            ],
          ),
          const SizedBox(height: 8),
          Text(
            r['content'] ?? '',
            style: TextStyle(
              color: Colors.grey[300],
              fontSize: 13,
              height: 1.5,
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildReplyBar() {
    return Container(
      padding: EdgeInsets.fromLTRB(
        16,
        10,
        16,
        MediaQuery.of(context).padding.bottom + 10,
      ),
      decoration: const BoxDecoration(
        color: Color(0xFF1E293B),
        border: Border(top: BorderSide(color: Color(0xFF334155))),
      ),
      child: Row(
        children: [
          Expanded(
            child: TextField(
              controller: _replyCtrl,
              style: const TextStyle(fontSize: 14),
              decoration: InputDecoration(
                hintText: '写下你的回复...',
                filled: true,
                fillColor: const Color(0xFF334155),
                contentPadding: const EdgeInsets.symmetric(
                  horizontal: 14,
                  vertical: 10,
                ),
                border: OutlineInputBorder(
                  borderRadius: BorderRadius.circular(20),
                  borderSide: BorderSide.none,
                ),
              ),
              onSubmitted: (_) => _submitReply(),
            ),
          ),
          const SizedBox(width: 8),
          GestureDetector(
            onTap: _replying ? null : _submitReply,
            child: Container(
              width: 40,
              height: 40,
              decoration: BoxDecoration(
                color: AppTheme.primaryColor,
                borderRadius: BorderRadius.circular(20),
              ),
              child: _replying
                  ? const Padding(
                      padding: EdgeInsets.all(10),
                      child: CircularProgressIndicator(
                        strokeWidth: 2,
                        color: Colors.white,
                      ),
                    )
                  : const Icon(
                      Icons.send_rounded,
                      color: Colors.white,
                      size: 18,
                    ),
            ),
          ),
        ],
      ),
    );
  }

  void _showReportSheet(
    String targetType,
    String targetId,
    String? title,
    String? authorId,
  ) {
    String? selectedReason;
    final detailCtrl = TextEditingController();
    final reasons = [
      ('spam', '垃圾信息'),
      ('abuse', '辱骂/人身攻击'),
      ('nsfw', '不当内容'),
      ('illegal', '违法违规'),
      ('other', '其他'),
    ];

    showModalBottomSheet(
      context: context,
      backgroundColor: const Color(0xFF1E293B),
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(20)),
      ),
      isScrollControlled: true,
      builder: (ctx) => StatefulBuilder(
        builder: (ctx, setSheetState) => Padding(
          padding: EdgeInsets.fromLTRB(
            24,
            24,
            24,
            MediaQuery.of(ctx).viewInsets.bottom + 24,
          ),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              const Row(
                children: [
                  Icon(
                    Icons.flag_rounded,
                    color: AppTheme.errorColor,
                    size: 20,
                  ),
                  SizedBox(width: 8),
                  Text(
                    '举报内容',
                    style: TextStyle(
                      color: Colors.white,
                      fontSize: 18,
                      fontWeight: FontWeight.bold,
                    ),
                  ),
                ],
              ),
              if (title != null && title.isNotEmpty) ...[
                const SizedBox(height: 8),
                Text(
                  '目标: $title',
                  style: TextStyle(color: Colors.grey[500], fontSize: 12),
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                ),
              ],
              const SizedBox(height: 16),
              Wrap(
                spacing: 8,
                runSpacing: 8,
                children: reasons
                    .map(
                      (r) => GestureDetector(
                        onTap: () => setSheetState(() => selectedReason = r.$1),
                        child: Container(
                          padding: const EdgeInsets.symmetric(
                            horizontal: 14,
                            vertical: 8,
                          ),
                          decoration: BoxDecoration(
                            color: selectedReason == r.$1
                                ? AppTheme.errorColor.withValues(alpha: 0.15)
                                : const Color(0xFF334155),
                            borderRadius: BorderRadius.circular(20),
                            border: Border.all(
                              color: selectedReason == r.$1
                                  ? AppTheme.errorColor
                                  : Colors.transparent,
                            ),
                          ),
                          child: Text(
                            r.$2,
                            style: TextStyle(
                              color: selectedReason == r.$1
                                  ? AppTheme.errorColor
                                  : Colors.grey[400],
                              fontSize: 13,
                            ),
                          ),
                        ),
                      ),
                    )
                    .toList(),
              ),
              const SizedBox(height: 14),
              TextField(
                controller: detailCtrl,
                maxLines: 3,
                decoration: const InputDecoration(hintText: '补充说明（可选）'),
              ),
              const SizedBox(height: 16),
              SizedBox(
                width: double.infinity,
                child: ElevatedButton(
                  onPressed: selectedReason == null
                      ? null
                      : () async {
                          Navigator.pop(ctx);
                          try {
                            await ApiService().submitReport(
                              targetType: targetType,
                              targetId: targetId,
                              reason: selectedReason!,
                              targetTitle: title,
                              authorId: authorId,
                              detail: detailCtrl.text.isNotEmpty
                                  ? detailCtrl.text
                                  : null,
                            );
                            if (mounted) {
                              ScaffoldMessenger.of(context).showSnackBar(
                                const SnackBar(content: Text('举报已提交，我们会尽快处理')),
                              );
                            }
                          } catch (_) {
                            if (mounted) {
                              ScaffoldMessenger.of(context).showSnackBar(
                                const SnackBar(content: Text('提交失败')),
                              );
                            }
                          }
                        },
                  style: ElevatedButton.styleFrom(
                    backgroundColor: AppTheme.errorColor,
                  ),
                  child: const Text('提交举报'),
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }

  @override
  void dispose() {
    _replyCtrl.dispose();
    super.dispose();
  }
}
