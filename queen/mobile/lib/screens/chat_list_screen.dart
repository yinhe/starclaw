import 'package:flutter/material.dart';
import '../services/api_service.dart';
import '../theme.dart';
import 'chat_screen.dart';

class ChatListScreen extends StatefulWidget {
  const ChatListScreen({super.key});

  @override
  State<ChatListScreen> createState() => _ChatListScreenState();
}

class _ChatListScreenState extends State<ChatListScreen> {
  List<Map<String, dynamic>> _conversations = [];
  bool _loading = true;
  String? _superAgentId;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    setState(() => _loading = true);
    try {
      final results = await Future.wait([
        ApiService().listConversations(),
        ApiService().ensureSuperAgent(),
      ]);
      final convRes = results[0];
      final agentRes = results[1];
      setState(() {
        _conversations = List<Map<String, dynamic>>.from(
          convRes.data['conversations'] ?? [],
        );
        _superAgentId = agentRes.data['agent']?['id'] ?? agentRes.data['id'];
      });
    } catch (e) {
      debugPrint('Failed to load conversations: $e');
    } finally {
      if (mounted) setState(() => _loading = false);
    }
  }

  void _newChat() {
    if (_superAgentId == null) return;
    Navigator.push(
      context,
      MaterialPageRoute(builder: (_) => ChatScreen(agentId: _superAgentId!)),
    ).then((_) => _load());
  }

  void _openChat(Map<String, dynamic> conv) {
    Navigator.push(
      context,
      MaterialPageRoute(
        builder: (_) => ChatScreen(
          agentId: conv['agent_id'] ?? _superAgentId ?? '',
          conversationId: conv['id'],
          title: conv['title'] ?? '对话',
        ),
      ),
    ).then((_) => _load());
  }

  Future<void> _deleteConversation(String id) async {
    try {
      await ApiService().deleteConversation(id);
      _load();
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(const SnackBar(content: Text('删除失败')));
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('对话'),
        actions: [
          IconButton(icon: const Icon(Icons.refresh), onPressed: _load),
        ],
      ),
      body: _loading
          ? const Center(child: CircularProgressIndicator())
          : _conversations.isEmpty
          ? Center(
              child: Column(
                mainAxisAlignment: MainAxisAlignment.center,
                children: [
                  Icon(
                    Icons.chat_bubble_outline,
                    size: 64,
                    color: Colors.grey[600],
                  ),
                  const SizedBox(height: 16),
                  Text(
                    '还没有对话',
                    style: TextStyle(color: Colors.grey[400], fontSize: 16),
                  ),
                  const SizedBox(height: 8),
                  Text(
                    '点击右下角开始新对话',
                    style: TextStyle(color: Colors.grey[600], fontSize: 13),
                  ),
                ],
              ),
            )
          : RefreshIndicator(
              onRefresh: _load,
              child: ListView.builder(
                padding: const EdgeInsets.symmetric(vertical: 8),
                itemCount: _conversations.length,
                itemBuilder: (context, index) {
                  final conv = _conversations[index];
                  final title = conv['title'] ?? '新对话';
                  final updatedAt = conv['updated_at'] ?? '';
                  final agentName = conv['agent_name'] ?? '';
                  return Dismissible(
                    key: Key(conv['id']),
                    direction: DismissDirection.endToStart,
                    background: Container(
                      alignment: Alignment.centerRight,
                      padding: const EdgeInsets.only(right: 20),
                      color: AppTheme.errorColor,
                      child: const Icon(Icons.delete, color: Colors.white),
                    ),
                    confirmDismiss: (_) async {
                      return await showDialog<bool>(
                            context: context,
                            builder: (ctx) => AlertDialog(
                              title: const Text('删除对话'),
                              content: const Text('确定要删除这个对话吗？'),
                              actions: [
                                TextButton(
                                  onPressed: () => Navigator.pop(ctx, false),
                                  child: const Text('取消'),
                                ),
                                TextButton(
                                  onPressed: () => Navigator.pop(ctx, true),
                                  child: const Text(
                                    '删除',
                                    style: TextStyle(
                                      color: AppTheme.errorColor,
                                    ),
                                  ),
                                ),
                              ],
                            ),
                          ) ??
                          false;
                    },
                    onDismissed: (_) => _deleteConversation(conv['id']),
                    child: ListTile(
                      contentPadding: const EdgeInsets.symmetric(
                        horizontal: 20,
                        vertical: 4,
                      ),
                      leading: Container(
                        width: 44,
                        height: 44,
                        decoration: BoxDecoration(
                          color: AppTheme.primaryColor.withValues(alpha: 0.15),
                          borderRadius: BorderRadius.circular(12),
                        ),
                        child: const Icon(
                          Icons.chat,
                          color: AppTheme.primaryColor,
                          size: 22,
                        ),
                      ),
                      title: Text(
                        title,
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                        style: const TextStyle(fontWeight: FontWeight.w500),
                      ),
                      subtitle: Text(
                        agentName.isNotEmpty
                            ? agentName
                            : _formatTime(updatedAt),
                        style: TextStyle(color: Colors.grey[500], fontSize: 12),
                      ),
                      trailing: Text(
                        _formatTime(updatedAt),
                        style: TextStyle(color: Colors.grey[600], fontSize: 11),
                      ),
                      onTap: () => _openChat(conv),
                    ),
                  );
                },
              ),
            ),
      floatingActionButton: FloatingActionButton(
        onPressed: _newChat,
        backgroundColor: AppTheme.primaryColor,
        child: const Icon(Icons.add, color: Colors.white),
      ),
    );
  }

  String _formatTime(String iso) {
    if (iso.isEmpty) return '';
    try {
      final dt = DateTime.parse(iso).toLocal();
      final now = DateTime.now();
      if (dt.year == now.year && dt.month == now.month && dt.day == now.day) {
        return '${dt.hour.toString().padLeft(2, '0')}:${dt.minute.toString().padLeft(2, '0')}';
      }
      return '${dt.month}/${dt.day}';
    } catch (_) {
      return '';
    }
  }
}
