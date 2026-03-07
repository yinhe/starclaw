import 'package:flutter/material.dart';
import 'package:video_player/video_player.dart';
import '../services/api_service.dart';
import '../theme.dart';

class VideosScreen extends StatefulWidget {
  const VideosScreen({super.key});

  @override
  State<VideosScreen> createState() => _VideosScreenState();
}

class _VideosScreenState extends State<VideosScreen>
    with SingleTickerProviderStateMixin {
  late TabController _tabCtrl;
  List<Map<String, dynamic>> _videos = [];
  bool _loading = true;

  @override
  void initState() {
    super.initState();
    _tabCtrl = TabController(length: 3, vsync: this);
    _load();
  }

  Future<void> _load() async {
    setState(() => _loading = true);
    try {
      final res = await ApiService().listVideos();
      setState(() {
        _videos = List<Map<String, dynamic>>.from(res.data['videos'] ?? []);
      });
    } catch (e) {
      debugPrint('Failed to load videos: $e');
    } finally {
      if (mounted) setState(() => _loading = false);
    }
  }

  List<Map<String, dynamic>> _filterByType(String type) {
    if (type == 'clip') {
      return _videos
          .where(
            (v) => v['type'] == 'clip' || v['type'] == '' || v['type'] == null,
          )
          .toList();
    }
    if (type == 'merged') {
      return _videos
          .where((v) => v['type'] == 'merged' || v['type'] == 'mv')
          .toList();
    }
    if (type == 'narrated') {
      return _videos.where((v) => v['type'] == 'narrated').toList();
    }
    return _videos;
  }

  Future<void> _deleteVideo(String id) async {
    try {
      await ApiService().deleteVideo(id);
      _load();
      if (mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(const SnackBar(content: Text('已删除')));
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(const SnackBar(content: Text('删除失败')));
      }
    }
  }

  void _playVideo(Map<String, dynamic> video) {
    final url = video['video_url'] ?? '';
    if (url.isEmpty) return;
    Navigator.push(
      context,
      MaterialPageRoute(
        builder: (_) => _VideoPlayerPage(
          url: ApiService().resolveUrl(url),
          title: video['prompt'] ?? '视频',
        ),
      ),
    );
  }

  void _showActions(Map<String, dynamic> video) {
    final type = video['type'] ?? 'clip';
    final status = video['status'] ?? '';

    showModalBottomSheet(
      context: context,
      backgroundColor: const Color(0xFF1E293B),
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(20)),
      ),
      builder: (ctx) => SafeArea(
        child: Padding(
          padding: const EdgeInsets.symmetric(vertical: 8),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Container(
                width: 36,
                height: 4,
                margin: const EdgeInsets.only(bottom: 16),
                decoration: BoxDecoration(
                  color: Colors.grey[600],
                  borderRadius: BorderRadius.circular(2),
                ),
              ),
              if (status == 'succeeded' && video['video_url'] != null)
                _actionTile(
                  ctx,
                  Icons.play_arrow,
                  '播放',
                  () => _playVideo(video),
                ),
              if (status == 'succeeded' &&
                  (type == 'clip' || type == '' || type == null))
                _actionTile(ctx, Icons.record_voice_over, '配音', () {
                  Navigator.pop(ctx);
                  _showDubDialog(video);
                }),
              if (status == 'succeeded' && (type == 'merged' || type == 'mv'))
                _actionTile(ctx, Icons.music_note, '配乐', () {
                  Navigator.pop(ctx);
                  _showMusicDialog(video);
                }),
              if (status == 'succeeded' &&
                  (type == 'clip' || type == '' || type == null))
                _actionTile(ctx, Icons.refresh, '重做', () async {
                  Navigator.pop(ctx);
                  final confirm = await _confirm('重新生成', '确定重新生成此片段？');
                  if (confirm) {
                    await ApiService().regenerateVideo(video['id']);
                    _showToast('正在重新生成...');
                    _load();
                  }
                }),
              if (status == 'succeeded' && (type == 'merged' || type == 'mv'))
                _actionTile(ctx, Icons.merge, '重新合成', () async {
                  Navigator.pop(ctx);
                  final confirm = await _confirm('重新合成', '确定重新合成此视频？');
                  if (confirm) {
                    await ApiService().remergeVideo(video['id']);
                    _showToast('正在重新合成...');
                    _load();
                  }
                }),
              if (status == 'failed')
                _actionTile(ctx, Icons.replay, '重试', () async {
                  Navigator.pop(ctx);
                  await ApiService().retryVideo(video['id']);
                  _showToast('正在重试...');
                  _load();
                }),
              _actionTile(ctx, Icons.delete, '删除', () {
                Navigator.pop(ctx);
                _deleteVideo(video['id']);
              }, color: AppTheme.errorColor),
            ],
          ),
        ),
      ),
    );
  }

  Widget _actionTile(
    BuildContext ctx,
    IconData icon,
    String label,
    VoidCallback onTap, {
    Color? color,
  }) {
    return ListTile(
      leading: Icon(icon, color: color ?? Colors.white),
      title: Text(label, style: TextStyle(color: color ?? Colors.white)),
      onTap: onTap,
    );
  }

  Future<bool> _confirm(String title, String message) async {
    return await showDialog<bool>(
          context: context,
          builder: (ctx) => AlertDialog(
            backgroundColor: const Color(0xFF1E293B),
            title: Text(title, style: const TextStyle(color: Colors.white)),
            content: Text(message, style: const TextStyle(color: Colors.grey)),
            actions: [
              TextButton(
                onPressed: () => Navigator.pop(ctx, false),
                child: const Text('取消'),
              ),
              TextButton(
                onPressed: () => Navigator.pop(ctx, true),
                child: const Text(
                  '确定',
                  style: TextStyle(color: AppTheme.primaryColor),
                ),
              ),
            ],
          ),
        ) ??
        false;
  }

  void _showToast(String msg) {
    if (mounted) {
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(msg)));
    }
  }

  void _showDubDialog(Map<String, dynamic> video) {
    final textCtrl = TextEditingController(text: video['prompt'] ?? '');
    String voice = 'longyuan';
    bool subtitle = true;

    showDialog(
      context: context,
      builder: (ctx) => StatefulBuilder(
        builder: (ctx, setDialogState) => AlertDialog(
          backgroundColor: const Color(0xFF1E293B),
          title: Row(
            children: [
              const Icon(
                Icons.record_voice_over,
                color: AppTheme.accentColor,
                size: 20,
              ),
              const SizedBox(width: 8),
              const Text(
                '视频配音',
                style: TextStyle(color: Colors.white, fontSize: 16),
              ),
            ],
          ),
          content: SingleChildScrollView(
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                TextField(
                  controller: textCtrl,
                  maxLines: 4,
                  decoration: const InputDecoration(hintText: '输入配音文案...'),
                ),
                const SizedBox(height: 16),
                const Text(
                  '音色',
                  style: TextStyle(color: Colors.grey, fontSize: 12),
                ),
                const SizedBox(height: 8),
                Wrap(
                  spacing: 8,
                  runSpacing: 8,
                  children: [
                    for (final v in _voices)
                      ChoiceChip(
                        label: Text(
                          v['label']!,
                          style: TextStyle(
                            fontSize: 12,
                            color: voice == v['id']
                                ? Colors.white
                                : Colors.grey[400],
                          ),
                        ),
                        selected: voice == v['id'],
                        selectedColor: AppTheme.primaryColor,
                        backgroundColor: const Color(0xFF334155),
                        onSelected: (_) =>
                            setDialogState(() => voice = v['id']!),
                      ),
                  ],
                ),
                const SizedBox(height: 12),
                Row(
                  children: [
                    const Text(
                      '字幕',
                      style: TextStyle(color: Colors.grey, fontSize: 12),
                    ),
                    const Spacer(),
                    Switch(
                      value: subtitle,
                      activeColor: AppTheme.primaryColor,
                      onChanged: (v) => setDialogState(() => subtitle = v),
                    ),
                  ],
                ),
              ],
            ),
          ),
          actions: [
            TextButton(
              onPressed: () => Navigator.pop(ctx),
              child: const Text('取消'),
            ),
            ElevatedButton(
              onPressed: () async {
                Navigator.pop(ctx);
                try {
                  await ApiService().dubVideo(
                    video['id'],
                    textCtrl.text.trim(),
                    voice,
                    subtitleStyle: subtitle ? 'auto' : 'none',
                  );
                  _showToast('配音任务已开始');
                  _load();
                  Future.delayed(const Duration(seconds: 5), _load);
                } catch (e) {
                  _showToast('配音失败');
                }
              },
              child: const Text('开始配音'),
            ),
          ],
        ),
      ),
    );
  }

  void _showMusicDialog(Map<String, dynamic> video) async {
    List<Map<String, dynamic>> musicList = [];
    try {
      final res = await ApiService().listMusic();
      musicList = List<Map<String, dynamic>>.from(res.data['music'] ?? [])
          .where(
            (m) =>
                m['status'] == 'succeeded' && (m['local_url'] ?? '').isNotEmpty,
          )
          .toList();
    } catch (e) {
      _showToast('加载音乐列表失败');
      return;
    }

    if (!mounted) return;

    String? selectedId;
    showDialog(
      context: context,
      builder: (ctx) => StatefulBuilder(
        builder: (ctx, setDialogState) => AlertDialog(
          backgroundColor: const Color(0xFF1E293B),
          title: Row(
            children: [
              const Icon(
                Icons.music_note,
                color: AppTheme.warningColor,
                size: 20,
              ),
              const SizedBox(width: 8),
              const Text(
                '视频配乐',
                style: TextStyle(color: Colors.white, fontSize: 16),
              ),
            ],
          ),
          content: SizedBox(
            width: double.maxFinite,
            height: 300,
            child: musicList.isEmpty
                ? const Center(
                    child: Text(
                      '还没有已完成的音乐',
                      style: TextStyle(color: Colors.grey),
                    ),
                  )
                : ListView.builder(
                    itemCount: musicList.length,
                    itemBuilder: (_, i) {
                      final m = musicList[i];
                      final isSelected = selectedId == m['id'];
                      return Card(
                        color: isSelected
                            ? AppTheme.warningColor.withValues(alpha: 0.15)
                            : const Color(0xFF334155),
                        child: ListTile(
                          leading: Icon(
                            isSelected ? Icons.check_circle : Icons.music_note,
                            color: isSelected
                                ? AppTheme.warningColor
                                : Colors.grey,
                          ),
                          title: Text(
                            m['prompt'] ?? '(无描述)',
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                            style: const TextStyle(
                              color: Colors.white,
                              fontSize: 13,
                            ),
                          ),
                          subtitle: Text(
                            '${m['model'] ?? ''} · ${m['duration'] ?? 0}秒',
                            style: const TextStyle(
                              color: Colors.grey,
                              fontSize: 11,
                            ),
                          ),
                          onTap: () =>
                              setDialogState(() => selectedId = m['id']),
                        ),
                      );
                    },
                  ),
          ),
          actions: [
            TextButton(
              onPressed: () => Navigator.pop(ctx),
              child: const Text('取消'),
            ),
            ElevatedButton(
              style: ElevatedButton.styleFrom(
                backgroundColor: AppTheme.warningColor,
              ),
              onPressed: selectedId == null
                  ? null
                  : () async {
                      Navigator.pop(ctx);
                      try {
                        await ApiService().addMusicToVideo(
                          video['id'],
                          selectedId!,
                        );
                        _showToast('配乐任务已开始');
                        _load();
                        Future.delayed(const Duration(seconds: 5), _load);
                      } catch (e) {
                        _showToast('配乐失败');
                      }
                    },
              child: const Text('开始配乐'),
            ),
          ],
        ),
      ),
    );
  }

  static const _voices = [
    {'id': 'longxiaochun', 'label': '小春♀'},
    {'id': 'longxiaoxia', 'label': '小夏♀'},
    {'id': 'longlaotie', 'label': '老铁♀'},
    {'id': 'longmiao', 'label': '妙♀'},
    {'id': 'longyuan', 'label': '远♂'},
    {'id': 'longhua', 'label': '华♂'},
    {'id': 'longjing', 'label': '靖♂'},
    {'id': 'longshuo', 'label': '硕♂'},
  ];

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('视频'),
        actions: [
          IconButton(icon: const Icon(Icons.refresh), onPressed: _load),
        ],
        bottom: TabBar(
          controller: _tabCtrl,
          indicatorColor: AppTheme.primaryColor,
          labelColor: AppTheme.primaryColor,
          unselectedLabelColor: Colors.grey,
          tabs: const [
            Tab(text: '片段'),
            Tab(text: '合成'),
            Tab(text: '配音'),
          ],
        ),
      ),
      body: _loading
          ? const Center(child: CircularProgressIndicator())
          : TabBarView(
              controller: _tabCtrl,
              children: [
                _buildVideoList(_filterByType('clip')),
                _buildVideoList(_filterByType('merged')),
                _buildVideoList(_filterByType('narrated')),
              ],
            ),
    );
  }

  Widget _buildVideoList(List<Map<String, dynamic>> items) {
    if (items.isEmpty) {
      return Center(
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(Icons.videocam_off, size: 48, color: Colors.grey[700]),
            const SizedBox(height: 12),
            Text('暂无视频', style: TextStyle(color: Colors.grey[500])),
          ],
        ),
      );
    }
    return RefreshIndicator(
      onRefresh: _load,
      child: GridView.builder(
        padding: const EdgeInsets.all(12),
        gridDelegate: const SliverGridDelegateWithFixedCrossAxisCount(
          crossAxisCount: 2,
          mainAxisSpacing: 12,
          crossAxisSpacing: 12,
          childAspectRatio: 0.75,
        ),
        itemCount: items.length,
        itemBuilder: (context, index) => _buildVideoCard(items[index]),
      ),
    );
  }

  Widget _buildVideoCard(Map<String, dynamic> video) {
    final status = video['status'] ?? 'pending';
    final statusInfo = _statusMap[status];
    final thumbUrl = video['img_url'] ?? '';

    return GestureDetector(
      onTap: () {
        if (status == 'succeeded') _playVideo(video);
      },
      onLongPress: () => _showActions(video),
      child: Container(
        decoration: BoxDecoration(
          color: const Color(0xFF1E293B),
          borderRadius: BorderRadius.circular(16),
        ),
        clipBehavior: Clip.hardEdge,
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Expanded(
              child: Stack(
                fit: StackFit.expand,
                children: [
                  if (thumbUrl.isNotEmpty)
                    Image.network(
                      ApiService().resolveUrl(thumbUrl),
                      fit: BoxFit.cover,
                      errorBuilder: (_, __, ___) => _placeholder(),
                    )
                  else
                    _placeholder(),
                  if (status == 'succeeded')
                    Center(
                      child: Container(
                        width: 44,
                        height: 44,
                        decoration: BoxDecoration(
                          color: Colors.black45,
                          borderRadius: BorderRadius.circular(22),
                        ),
                        child: const Icon(
                          Icons.play_arrow,
                          color: Colors.white,
                          size: 28,
                        ),
                      ),
                    ),
                  if (status == 'running')
                    const Center(
                      child: CircularProgressIndicator(strokeWidth: 2),
                    ),
                  // Status badge
                  Positioned(
                    top: 8,
                    left: 8,
                    child: Container(
                      padding: const EdgeInsets.symmetric(
                        horizontal: 8,
                        vertical: 3,
                      ),
                      decoration: BoxDecoration(
                        color:
                            (statusInfo?['color'] as Color?)?.withValues(
                              alpha: 0.9,
                            ) ??
                            Colors.grey,
                        borderRadius: BorderRadius.circular(8),
                      ),
                      child: Text(
                        statusInfo?['label'] ?? status,
                        style: const TextStyle(
                          color: Colors.white,
                          fontSize: 10,
                          fontWeight: FontWeight.w600,
                        ),
                      ),
                    ),
                  ),
                  // Actions button
                  Positioned(
                    top: 4,
                    right: 4,
                    child: GestureDetector(
                      onTap: () => _showActions(video),
                      child: Container(
                        width: 28,
                        height: 28,
                        decoration: BoxDecoration(
                          color: Colors.black38,
                          borderRadius: BorderRadius.circular(14),
                        ),
                        child: const Icon(
                          Icons.more_vert,
                          color: Colors.white,
                          size: 16,
                        ),
                      ),
                    ),
                  ),
                ],
              ),
            ),
            Padding(
              padding: const EdgeInsets.all(10),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    video['prompt'] ?? '无描述',
                    maxLines: 2,
                    overflow: TextOverflow.ellipsis,
                    style: const TextStyle(
                      color: Colors.white,
                      fontSize: 12,
                      height: 1.3,
                    ),
                  ),
                  const SizedBox(height: 4),
                  Text(
                    '${video['model'] ?? ''} · ${video['duration'] ?? 0}s',
                    style: TextStyle(color: Colors.grey[500], fontSize: 10),
                  ),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _placeholder() {
    return Container(
      color: const Color(0xFF0F172A),
      child: const Center(
        child: Icon(Icons.videocam, color: Color(0xFF334155), size: 32),
      ),
    );
  }

  static final Map<String, Map<String, dynamic>> _statusMap = {
    'running': {'label': '生成中', 'color': Colors.blue},
    'pending': {'label': '等待中', 'color': Colors.orange},
    'succeeded': {'label': '完成', 'color': Colors.green},
    'failed': {'label': '失败', 'color': Colors.red},
    'cancelled': {'label': '已取消', 'color': Colors.grey},
  };

  @override
  void dispose() {
    _tabCtrl.dispose();
    super.dispose();
  }
}

class _VideoPlayerPage extends StatefulWidget {
  final String url;
  final String title;
  const _VideoPlayerPage({required this.url, required this.title});

  @override
  State<_VideoPlayerPage> createState() => _VideoPlayerPageState();
}

class _VideoPlayerPageState extends State<_VideoPlayerPage> {
  late VideoPlayerController _controller;
  bool _initialized = false;

  @override
  void initState() {
    super.initState();
    _controller = VideoPlayerController.networkUrl(Uri.parse(widget.url))
      ..initialize().then((_) {
        if (mounted) {
          setState(() => _initialized = true);
          _controller.play();
        }
      });
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: Colors.black,
      appBar: AppBar(
        backgroundColor: Colors.black,
        title: Text(widget.title, maxLines: 1, overflow: TextOverflow.ellipsis),
      ),
      body: Center(
        child: _initialized
            ? AspectRatio(
                aspectRatio: _controller.value.aspectRatio,
                child: Stack(
                  alignment: Alignment.center,
                  children: [
                    VideoPlayer(_controller),
                    GestureDetector(
                      onTap: () {
                        setState(() {
                          _controller.value.isPlaying
                              ? _controller.pause()
                              : _controller.play();
                        });
                      },
                      child: AnimatedOpacity(
                        opacity: _controller.value.isPlaying ? 0 : 1,
                        duration: const Duration(milliseconds: 200),
                        child: Container(
                          width: 64,
                          height: 64,
                          decoration: BoxDecoration(
                            color: Colors.black45,
                            borderRadius: BorderRadius.circular(32),
                          ),
                          child: const Icon(
                            Icons.play_arrow,
                            color: Colors.white,
                            size: 40,
                          ),
                        ),
                      ),
                    ),
                    Positioned(
                      bottom: 0,
                      left: 0,
                      right: 0,
                      child: VideoProgressIndicator(
                        _controller,
                        allowScrubbing: true,
                        colors: const VideoProgressColors(
                          playedColor: AppTheme.primaryColor,
                          bufferedColor: Colors.white24,
                          backgroundColor: Colors.white10,
                        ),
                      ),
                    ),
                  ],
                ),
              )
            : const CircularProgressIndicator(),
      ),
    );
  }

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }
}
