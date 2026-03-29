import 'package:flutter/material.dart';
import '../services/api_service.dart';
import '../theme.dart';

class GrowthScreen extends StatefulWidget {
  const GrowthScreen({super.key});

  @override
  State<GrowthScreen> createState() => _GrowthScreenState();
}

class _GrowthScreenState extends State<GrowthScreen> {
  Map<String, dynamic>? _season;
  List<dynamic>? _leaderboard;
  bool _loading = true;
  String? _error;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    setState(() => _loading = true);
    try {
      dynamic seasonRes;
      dynamic lbRes;
      try {
        seasonRes = await ApiService().getArenaSeason();
      } catch (_) {}
      try {
        lbRes = await ApiService().getArenaLeaderboard();
      } catch (_) {}
      if (mounted) {
        setState(() {
          _season = seasonRes?.data is Map ? seasonRes.data : null;
          final lbData = lbRes?.data;
          _leaderboard = lbData is Map
              ? (lbData['leaderboard'] ?? lbData['fighters'] ?? [])
              : (lbData is List ? lbData : []);
          _loading = false;
        });
      }
    } catch (e) {
      if (mounted)
        setState(() {
          _error = e.toString();
          _loading = false;
        });
    }
  }

  @override
  Widget build(BuildContext context) {
    return SafeArea(
      child: RefreshIndicator(
        onRefresh: _load,
        child: _loading
            ? const Center(child: CircularProgressIndicator())
            : _error != null
            ? Center(
                child: Text(_error!, style: const TextStyle(color: Colors.red)),
              )
            : ListView(
                padding: const EdgeInsets.all(16),
                children: [
                  // Season card
                  if (_season != null) _buildSeasonCard(),
                  const SizedBox(height: 16),
                  // Leaderboard
                  const Text(
                    '排行榜',
                    style: TextStyle(
                      fontSize: 18,
                      fontWeight: FontWeight.bold,
                      color: Colors.white,
                    ),
                  ),
                  const SizedBox(height: 8),
                  if (_leaderboard != null)
                    ..._leaderboard!.asMap().entries.map(
                      (e) => _buildLeaderboardItem(e.key, e.value),
                    ),
                  if (_leaderboard == null || _leaderboard!.isEmpty)
                    const Padding(
                      padding: EdgeInsets.all(32),
                      child: Center(
                        child: Text(
                          '暂无战士',
                          style: TextStyle(color: Color(0xFF64748B)),
                        ),
                      ),
                    ),
                ],
              ),
      ),
    );
  }

  Widget _buildSeasonCard() {
    final name = _season?['season']?['name'] ?? _season?['name'] ?? '未知赛季';
    final env =
        _season?['season']?['environment'] ?? _season?['environment'] ?? '';
    final envEmoji =
        {'abyss': '🌊', 'terrain': '🏔️', 'sky': '☁️'}[env] ?? '🌍';

    return Card(
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Row(
          children: [
            Text(envEmoji, style: const TextStyle(fontSize: 32)),
            const SizedBox(width: 12),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    name,
                    style: const TextStyle(
                      fontSize: 16,
                      fontWeight: FontWeight.bold,
                      color: Colors.white,
                    ),
                  ),
                  const SizedBox(height: 4),
                  Text(
                    '环境: $env',
                    style: const TextStyle(
                      fontSize: 12,
                      color: Color(0xFF94A3B8),
                    ),
                  ),
                ],
              ),
            ),
            Container(
              padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
              decoration: BoxDecoration(
                color: AppTheme.successColor.withOpacity(0.15),
                borderRadius: BorderRadius.circular(8),
              ),
              child: const Text(
                '进行中',
                style: TextStyle(fontSize: 12, color: AppTheme.successColor),
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildLeaderboardItem(int index, dynamic fighter) {
    final name = fighter['name'] ?? '未知';
    final elo = fighter['elo'] ?? 0;
    final wins = fighter['wins'] ?? 0;
    final losses = fighter['losses'] ?? 0;
    final path = fighter['evolution_path'] ?? '';
    final pathEmoji =
        {'larva': '🥚', 'abyss': '🦑', 'terrain': '🦂', 'sky': '🦅'}[path] ??
        '🦞';
    final medals = ['🥇', '🥈', '🥉'];

    return Card(
      margin: const EdgeInsets.only(bottom: 8),
      child: ListTile(
        leading: CircleAvatar(
          backgroundColor: const Color(0xFF334155),
          child: Text(
            index < 3 ? medals[index] : '${index + 1}',
            style: const TextStyle(fontSize: 16),
          ),
        ),
        title: Row(
          children: [
            Text(pathEmoji, style: const TextStyle(fontSize: 18)),
            const SizedBox(width: 6),
            Text(
              name,
              style: const TextStyle(
                color: Colors.white,
                fontWeight: FontWeight.w500,
              ),
            ),
          ],
        ),
        subtitle: Text(
          '$wins胜 $losses负',
          style: const TextStyle(fontSize: 12, color: Color(0xFF94A3B8)),
        ),
        trailing: Container(
          padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
          decoration: BoxDecoration(
            color: AppTheme.warningColor.withOpacity(0.15),
            borderRadius: BorderRadius.circular(8),
          ),
          child: Text(
            'ELO $elo',
            style: const TextStyle(
              fontSize: 12,
              fontWeight: FontWeight.bold,
              color: AppTheme.warningColor,
            ),
          ),
        ),
      ),
    );
  }
}
