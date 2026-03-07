import 'package:flutter/material.dart';
import 'package:audioplayers/audioplayers.dart';
import '../services/api_service.dart';
import '../theme.dart';

class MusicScreen extends StatefulWidget {
  const MusicScreen({super.key});

  @override
  State<MusicScreen> createState() => _MusicScreenState();
}

class _MusicScreenState extends State<MusicScreen> {
  List<Map<String, dynamic>> _music = [];
  bool _loading = true;
  final AudioPlayer _player = AudioPlayer();
  String? _playingId;
  bool _isPlaying = false;

  @override
  void initState() {
    super.initState();
    _load();
    _player.onPlayerStateChanged.listen((state) {
      if (mounted) {
        setState(() => _isPlaying = state == PlayerState.playing);
      }
    });
    _player.onPlayerComplete.listen((_) {
      if (mounted) {
        setState(() {
          _playingId = null;
          _isPlaying = false;
        });
      }
    });
  }

  Future<void> _load() async {
    setState(() => _loading = true);
    try {
      final res = await ApiService().listMusic();
      setState(() {
        _music = List<Map<String, dynamic>>.from(res.data['music'] ?? []);
      });
    } catch (e) {
      debugPrint('Failed to load music: $e');
    } finally {
      if (mounted) setState(() => _loading = false);
    }
  }

  void _togglePlay(Map<String, dynamic> item) async {
    final id = item['id'] as String;
    final url = item['local_url'] ?? '';
    if (url.isEmpty) return;

    if (_playingId == id && _isPlaying) {
      await _player.pause();
      return;
    }

    if (_playingId != id) {
      await _player.stop();
      setState(() => _playingId = id);
      await _player.play(UrlSource(ApiService().resolveUrl(url)));
    } else {
      await _player.resume();
    }
  }

  Future<void> _delete(String id) async {
    final confirm = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        backgroundColor: const Color(0xFF1E293B),
        title: const Text('删除音乐', style: TextStyle(color: Colors.white)),
        content: const Text(
          '确定要删除这首音乐吗？',
          style: TextStyle(color: Colors.grey),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(ctx, false),
            child: const Text('取消'),
          ),
          TextButton(
            onPressed: () => Navigator.pop(ctx, true),
            child: const Text(
              '删除',
              style: TextStyle(color: AppTheme.errorColor),
            ),
          ),
        ],
      ),
    );
    if (confirm != true) return;
    try {
      if (_playingId == id) await _player.stop();
      await ApiService().deleteMusic(id);
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
        title: const Text('音乐'),
        actions: [
          IconButton(icon: const Icon(Icons.refresh), onPressed: _load),
        ],
      ),
      body: _loading
          ? const Center(child: CircularProgressIndicator())
          : _music.isEmpty
          ? Center(
              child: Column(
                mainAxisAlignment: MainAxisAlignment.center,
                children: [
                  Icon(Icons.music_off, size: 48, color: Colors.grey[700]),
                  const SizedBox(height: 12),
                  Text('暂无音乐', style: TextStyle(color: Colors.grey[500])),
                  const SizedBox(height: 4),
                  Text(
                    '在对话中让AI生成音乐',
                    style: TextStyle(color: Colors.grey[600], fontSize: 12),
                  ),
                ],
              ),
            )
          : RefreshIndicator(
              onRefresh: _load,
              child: ListView.builder(
                padding: const EdgeInsets.symmetric(vertical: 8),
                itemCount: _music.length,
                itemBuilder: (context, index) {
                  final item = _music[index];
                  final id = item['id'] as String;
                  final isCurrentPlaying = _playingId == id && _isPlaying;
                  final status = item['status'] ?? '';
                  final statusColor = status == 'succeeded'
                      ? Colors.green
                      : status == 'running'
                      ? Colors.blue
                      : status == 'failed'
                      ? Colors.red
                      : Colors.grey;

                  return Card(
                    margin: const EdgeInsets.symmetric(
                      horizontal: 12,
                      vertical: 4,
                    ),
                    child: ListTile(
                      contentPadding: const EdgeInsets.symmetric(
                        horizontal: 16,
                        vertical: 8,
                      ),
                      leading: GestureDetector(
                        onTap: status == 'succeeded'
                            ? () => _togglePlay(item)
                            : null,
                        child: Container(
                          width: 48,
                          height: 48,
                          decoration: BoxDecoration(
                            color: isCurrentPlaying
                                ? AppTheme.primaryColor.withValues(alpha: 0.2)
                                : const Color(0xFF334155),
                            borderRadius: BorderRadius.circular(12),
                          ),
                          child: Icon(
                            status == 'running'
                                ? Icons.hourglass_top
                                : isCurrentPlaying
                                ? Icons.pause
                                : Icons.play_arrow,
                            color: isCurrentPlaying
                                ? AppTheme.primaryColor
                                : Colors.white,
                          ),
                        ),
                      ),
                      title: Text(
                        item['prompt'] ?? '(无描述)',
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                        style: const TextStyle(
                          fontWeight: FontWeight.w500,
                          fontSize: 14,
                        ),
                      ),
                      subtitle: Padding(
                        padding: const EdgeInsets.only(top: 4),
                        child: Row(
                          children: [
                            Container(
                              padding: const EdgeInsets.symmetric(
                                horizontal: 6,
                                vertical: 2,
                              ),
                              decoration: BoxDecoration(
                                color: statusColor.withValues(alpha: 0.15),
                                borderRadius: BorderRadius.circular(4),
                              ),
                              child: Text(
                                status,
                                style: TextStyle(
                                  color: statusColor,
                                  fontSize: 10,
                                ),
                              ),
                            ),
                            const SizedBox(width: 8),
                            Text(
                              '${item['model'] ?? ''} · ${item['duration'] ?? 0}秒',
                              style: TextStyle(
                                color: Colors.grey[500],
                                fontSize: 11,
                              ),
                            ),
                          ],
                        ),
                      ),
                      trailing: IconButton(
                        icon: const Icon(Icons.delete_outline, size: 20),
                        color: Colors.grey[500],
                        onPressed: () => _delete(id),
                      ),
                    ),
                  );
                },
              ),
            ),
    );
  }

  @override
  void dispose() {
    _player.dispose();
    super.dispose();
  }
}
