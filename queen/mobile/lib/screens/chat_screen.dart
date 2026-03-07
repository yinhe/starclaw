import 'dart:async';
import 'dart:convert';
import 'package:flutter/material.dart';
import 'package:dio/dio.dart';
import 'package:flutter_markdown/flutter_markdown.dart';
import '../services/api_service.dart';
import '../services/auth_service.dart';
import '../theme.dart';

class ChatScreen extends StatefulWidget {
  final String agentId;
  final String? conversationId;
  final String? title;

  const ChatScreen({
    super.key,
    required this.agentId,
    this.conversationId,
    this.title,
  });

  @override
  State<ChatScreen> createState() => _ChatScreenState();
}

class _ChatScreenState extends State<ChatScreen> {
  final _inputCtrl = TextEditingController();
  final _scrollCtrl = ScrollController();
  final List<Map<String, dynamic>> _messages = [];
  String? _conversationId;
  bool _loading = true;
  bool _sending = false;
  String _streamingContent = '';
  String _title = '新对话';

  @override
  void initState() {
    super.initState();
    _conversationId = widget.conversationId;
    _title = widget.title ?? '新对话';
    if (_conversationId != null) {
      _loadMessages();
    } else {
      setState(() => _loading = false);
    }
  }

  Future<void> _loadMessages() async {
    if (_conversationId == null) return;
    setState(() => _loading = true);
    try {
      final res = await ApiService().getMessages(_conversationId!);
      setState(() {
        _messages.clear();
        _messages.addAll(
          List<Map<String, dynamic>>.from(res.data['messages'] ?? []),
        );
      });
      _scrollToBottom();
    } catch (e) {
      debugPrint('Failed to load messages: $e');
    } finally {
      if (mounted) setState(() => _loading = false);
    }
  }

  void _scrollToBottom() {
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (_scrollCtrl.hasClients) {
        _scrollCtrl.animateTo(
          _scrollCtrl.position.maxScrollExtent,
          duration: const Duration(milliseconds: 300),
          curve: Curves.easeOut,
        );
      }
    });
  }

  Future<void> _send() async {
    final text = _inputCtrl.text.trim();
    if (text.isEmpty || _sending) return;

    _inputCtrl.clear();
    setState(() {
      _sending = true;
      _streamingContent = '';
      _messages.add({
        'role': 'user',
        'content': text,
        'id': 'temp_user_${DateTime.now().millisecondsSinceEpoch}',
      });
    });
    _scrollToBottom();

    try {
      final token = await AuthService().getToken();
      final response = await Dio().post(
        ApiService.getChatStreamUrl(),
        data: jsonEncode({
          'agent_id': widget.agentId,
          'conversation_id': _conversationId ?? '',
          'message': text,
          'stream': true,
        }),
        options: Options(
          headers: {
            'Content-Type': 'application/json',
            'Authorization': 'Bearer $token',
            'Accept': 'text/event-stream',
          },
          responseType: ResponseType.stream,
        ),
      );

      String buffer = '';
      final stream = response.data.stream as Stream<List<int>>;
      await for (final chunk in stream) {
        buffer += utf8.decode(chunk);
        final lines = buffer.split('\n');
        buffer = lines.last;

        for (int i = 0; i < lines.length - 1; i++) {
          final line = lines[i].trim();
          if (line.startsWith('data: ')) {
            final jsonStr = line.substring(6);
            if (jsonStr == '[DONE]') continue;
            try {
              final data = jsonDecode(jsonStr);
              // Extract conversation_id
              if (_conversationId == null && data['conversation_id'] != null) {
                _conversationId = data['conversation_id'];
              }
              // Extract title
              if (data['title'] != null && data['title'] != '') {
                _title = data['title'];
              }
              // Extract content delta
              final delta = data['choices']?[0]?['delta']?['content'] ?? '';
              if (delta.isNotEmpty) {
                setState(() => _streamingContent += delta);
                _scrollToBottom();
              }
            } catch (_) {}
          }
        }
      }

      // Finalize streaming message
      if (_streamingContent.isNotEmpty) {
        setState(() {
          _messages.add({
            'role': 'assistant',
            'content': _streamingContent,
            'id': 'temp_asst_${DateTime.now().millisecondsSinceEpoch}',
          });
          _streamingContent = '';
        });
      }
    } catch (e) {
      debugPrint('Stream error: $e');
      // Fallback to non-streaming
      try {
        final res = await ApiService().sendChat(
          agentId: widget.agentId,
          conversationId: _conversationId,
          message: text,
        );
        final data = res.data;
        if (_conversationId == null && data['conversation_id'] != null) {
          _conversationId = data['conversation_id'];
        }
        final content =
            data['choices']?[0]?['message']?['content'] ??
            data['response'] ??
            '';
        if (content.isNotEmpty) {
          setState(() {
            _messages.add({'role': 'assistant', 'content': content});
          });
        }
      } catch (e2) {
        setState(() {
          _messages.add({'role': 'assistant', 'content': '❌ 发送失败，请重试'});
        });
      }
    } finally {
      if (mounted) setState(() => _sending = false);
      _scrollToBottom();
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: Text(_title, maxLines: 1, overflow: TextOverflow.ellipsis),
      ),
      body: Column(
        children: [
          Expanded(
            child: _loading
                ? const Center(child: CircularProgressIndicator())
                : _messages.isEmpty && _streamingContent.isEmpty
                ? Center(
                    child: Column(
                      mainAxisAlignment: MainAxisAlignment.center,
                      children: [
                        Icon(
                          Icons.auto_awesome,
                          size: 48,
                          color: AppTheme.primaryColor.withValues(alpha: 0.5),
                        ),
                        const SizedBox(height: 16),
                        Text(
                          '有什么我可以帮助你的？',
                          style: TextStyle(color: Colors.grey[400]),
                        ),
                      ],
                    ),
                  )
                : ListView.builder(
                    controller: _scrollCtrl,
                    padding: const EdgeInsets.symmetric(
                      horizontal: 16,
                      vertical: 8,
                    ),
                    itemCount:
                        _messages.length +
                        (_streamingContent.isNotEmpty ? 1 : 0),
                    itemBuilder: (context, index) {
                      if (index < _messages.length) {
                        return _buildMessage(_messages[index]);
                      }
                      // Streaming message
                      return _buildMessage({
                        'role': 'assistant',
                        'content': _streamingContent,
                      });
                    },
                  ),
          ),
          _buildInputBar(),
        ],
      ),
    );
  }

  Widget _buildMessage(Map<String, dynamic> msg) {
    final isUser = msg['role'] == 'user';
    return Padding(
      padding: const EdgeInsets.only(bottom: 12),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        mainAxisAlignment: isUser
            ? MainAxisAlignment.end
            : MainAxisAlignment.start,
        children: [
          if (!isUser) ...[
            CircleAvatar(
              radius: 16,
              backgroundColor: AppTheme.primaryColor.withValues(alpha: 0.2),
              child: const Icon(
                Icons.auto_awesome,
                size: 16,
                color: AppTheme.primaryColor,
              ),
            ),
            const SizedBox(width: 8),
          ],
          Flexible(
            child: Container(
              padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 10),
              decoration: BoxDecoration(
                color: isUser ? AppTheme.primaryColor : const Color(0xFF1E293B),
                borderRadius: BorderRadius.only(
                  topLeft: const Radius.circular(16),
                  topRight: const Radius.circular(16),
                  bottomLeft: Radius.circular(isUser ? 16 : 4),
                  bottomRight: Radius.circular(isUser ? 4 : 16),
                ),
              ),
              child: isUser
                  ? Text(
                      msg['content'] ?? '',
                      style: const TextStyle(color: Colors.white),
                    )
                  : MarkdownBody(
                      data: msg['content'] ?? '',
                      styleSheet: MarkdownStyleSheet(
                        p: const TextStyle(
                          color: Colors.white,
                          fontSize: 14,
                          height: 1.5,
                        ),
                        code: TextStyle(
                          color: Colors.green[300],
                          backgroundColor: const Color(0xFF0F172A),
                          fontSize: 13,
                        ),
                        codeblockDecoration: BoxDecoration(
                          color: const Color(0xFF0F172A),
                          borderRadius: BorderRadius.circular(8),
                        ),
                        h1: const TextStyle(
                          color: Colors.white,
                          fontSize: 20,
                          fontWeight: FontWeight.bold,
                        ),
                        h2: const TextStyle(
                          color: Colors.white,
                          fontSize: 18,
                          fontWeight: FontWeight.bold,
                        ),
                        h3: const TextStyle(
                          color: Colors.white,
                          fontSize: 16,
                          fontWeight: FontWeight.bold,
                        ),
                        listBullet: const TextStyle(color: Colors.white),
                        strong: const TextStyle(
                          color: Colors.white,
                          fontWeight: FontWeight.bold,
                        ),
                        em: const TextStyle(
                          color: Colors.white,
                          fontStyle: FontStyle.italic,
                        ),
                        a: TextStyle(color: Colors.blue[300]),
                      ),
                    ),
            ),
          ),
          if (isUser) const SizedBox(width: 8),
          if (isUser)
            CircleAvatar(
              radius: 16,
              backgroundColor: AppTheme.secondaryColor.withValues(alpha: 0.2),
              child: const Icon(
                Icons.person,
                size: 16,
                color: AppTheme.secondaryColor,
              ),
            ),
        ],
      ),
    );
  }

  Widget _buildInputBar() {
    return Container(
      padding: EdgeInsets.only(
        left: 12,
        right: 8,
        top: 8,
        bottom: MediaQuery.of(context).padding.bottom + 8,
      ),
      decoration: const BoxDecoration(
        color: Color(0xFF1E293B),
        border: Border(top: BorderSide(color: Color(0xFF334155))),
      ),
      child: Row(
        children: [
          Expanded(
            child: TextField(
              controller: _inputCtrl,
              maxLines: 4,
              minLines: 1,
              textInputAction: TextInputAction.send,
              onSubmitted: (_) => _send(),
              decoration: InputDecoration(
                hintText: '输入消息...',
                filled: true,
                fillColor: const Color(0xFF0F172A),
                border: OutlineInputBorder(
                  borderRadius: BorderRadius.circular(24),
                  borderSide: BorderSide.none,
                ),
                contentPadding: const EdgeInsets.symmetric(
                  horizontal: 16,
                  vertical: 10,
                ),
              ),
            ),
          ),
          const SizedBox(width: 8),
          Material(
            color: _sending ? Colors.grey : AppTheme.primaryColor,
            borderRadius: BorderRadius.circular(24),
            child: InkWell(
              onTap: _sending ? null : _send,
              borderRadius: BorderRadius.circular(24),
              child: Container(
                width: 44,
                height: 44,
                alignment: Alignment.center,
                child: _sending
                    ? const SizedBox(
                        width: 20,
                        height: 20,
                        child: CircularProgressIndicator(
                          strokeWidth: 2,
                          color: Colors.white,
                        ),
                      )
                    : const Icon(Icons.send, color: Colors.white, size: 20),
              ),
            ),
          ),
        ],
      ),
    );
  }

  @override
  void dispose() {
    _inputCtrl.dispose();
    _scrollCtrl.dispose();
    super.dispose();
  }
}
