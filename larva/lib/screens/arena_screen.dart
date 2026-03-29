import 'package:flutter/material.dart';
import '../services/api_service.dart';
import '../theme.dart';

class ArenaScreen extends StatefulWidget {
  const ArenaScreen({super.key});

  @override
  State<ArenaScreen> createState() => _ArenaScreenState();
}

class _ArenaScreenState extends State<ArenaScreen> with SingleTickerProviderStateMixin {
  late TabController _tabController;
  List<dynamic> _leaderboard = [];
  Map<String, dynamic>? _season;
  List<dynamic> _shop = [];
  bool _loading = true;

  @override
  void initState() {
    super.initState();
    _tabController = TabController(length: 3, vsync: this);
    _load();
  }

  @override
  void dispose() {
    _tabController.dispose();
    super.dispose();
  }

  Future<void> _load() async {
    setState(() => _loading = true);
    try {
      dynamic lbRes, seasonRes, shopRes;
      try { lbRes = await ApiService().getArenaLeaderboard(); } catch (_) {}
      try { seasonRes = await ApiService().getArenaSeason(); } catch (_) {}
      try { shopRes = await ApiService().getArenaShop(); } catch (_) {}
      if (mounted) {
        setState(() {
          final lbData = lbRes?.data;
          _leaderboard = lbData is Map
              ? (lbData['leaderboard'] ?? lbData['fighters'] ?? [])
              : (lbData is List ? lbData : []);
          _season = seasonRes?.data is Map ? seasonRes.data : null;
          final shopData = shopRes?.data;
          _shop = shopData is Map
              ? (shopData['shop'] ?? shopData['items'] ?? [])
              : (shopData is List ? shopData : []);
          _loading = false;
        });
      }
    } catch (_) {
      if (mounted) setState(() => _loading = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return SafeArea(
      child: Column(
        children: [
          // Header
          Padding(
            padding: const EdgeInsets.fromLTRB(16, 12, 16, 0),
            child: Row(
              children: [
                const Text('⚔️', style: TextStyle(fontSize: 24)),
                const SizedBox(width: 8),
                const Text('龙虾竞技场', style: TextStyle(fontSize: 20, fontWeight: FontWeight.bold, color: Colors.white)),
                const Spacer(),
                IconButton(
                  icon: const Icon(Icons.refresh, color: Color(0xFF94A3B8), size: 20),
                  onPressed: _load,
                ),
              ],
            ),
          ),
          // Tabs
          TabBar(
            controller: _tabController,
            indicatorColor: AppTheme.primaryColor,
            labelColor: Colors.white,
            unselectedLabelColor: const Color(0xFF64748B),
            tabs: const [
              Tab(text: '排行榜'),
              Tab(text: '赛季'),
              Tab(text: '商店'),
            ],
          ),
          // Content
          Expanded(
            child: _loading
                ? const Center(child: CircularProgressIndicator())
                : TabBarView(
                    controller: _tabController,
                    children: [
                      _buildLeaderboardTab(),
                      _buildSeasonTab(),
                      _buildShopTab(),
                    ],
                  ),
          ),
        ],
      ),
    );
  }

  Widget _buildLeaderboardTab() {
    if (_leaderboard.isEmpty) {
      return const Center(child: Text('暂无战士', style: TextStyle(color: Color(0xFF64748B))));
    }
    return RefreshIndicator(
      onRefresh: _load,
      child: ListView.builder(
        padding: const EdgeInsets.all(12),
        itemCount: _leaderboard.length,
        itemBuilder: (context, index) {
          final f = _leaderboard[index];
          final name = f['name'] ?? '未知';
          final elo = f['elo'] ?? 0;
          final wins = f['wins'] ?? 0;
          final losses = f['losses'] ?? 0;
          final path = f['evolution_path'] ?? '';
          final pathEmoji = {'larva': '🥚', 'abyss': '🦑', 'terrain': '🦂', 'sky': '🦅'}[path] ?? '🦞';
          final medals = ['🥇', '🥈', '🥉'];

          return Card(
            margin: const EdgeInsets.only(bottom: 8),
            child: ListTile(
              leading: CircleAvatar(
                backgroundColor: const Color(0xFF334155),
                child: Text(index < 3 ? medals[index] : '${index + 1}', style: const TextStyle(fontSize: 16)),
              ),
              title: Row(
                children: [
                  Text(pathEmoji, style: const TextStyle(fontSize: 18)),
                  const SizedBox(width: 6),
                  Expanded(child: Text(name, style: const TextStyle(color: Colors.white, fontWeight: FontWeight.w500))),
                ],
              ),
              subtitle: Text('$wins胜 $losses负', style: const TextStyle(fontSize: 12, color: Color(0xFF94A3B8))),
              trailing: Container(
                padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
                decoration: BoxDecoration(
                  color: AppTheme.warningColor.withOpacity(0.15),
                  borderRadius: BorderRadius.circular(8),
                ),
                child: Text('$elo', style: const TextStyle(fontSize: 13, fontWeight: FontWeight.bold, color: AppTheme.warningColor)),
              ),
            ),
          );
        },
      ),
    );
  }

  Widget _buildSeasonTab() {
    if (_season == null) {
      return const Center(child: Text('赛季信息加载中...', style: TextStyle(color: Color(0xFF64748B))));
    }
    final s = _season!['season'] ?? _season!;
    final name = s['name'] ?? '未知';
    final env = s['environment'] ?? '';
    final envEmoji = {'abyss': '🌊', 'terrain': '🏔️', 'sky': '☁️'}[env] ?? '🌍';
    final active = s['active'] == true;

    return ListView(
      padding: const EdgeInsets.all(16),
      children: [
        Card(
          child: Padding(
            padding: const EdgeInsets.all(20),
            child: Column(
              children: [
                Text(envEmoji, style: const TextStyle(fontSize: 48)),
                const SizedBox(height: 12),
                Text(name, style: const TextStyle(fontSize: 20, fontWeight: FontWeight.bold, color: Colors.white)),
                const SizedBox(height: 8),
                Row(
                  mainAxisAlignment: MainAxisAlignment.center,
                  children: [
                    _infoChip('环境', env),
                    const SizedBox(width: 12),
                    _infoChip('状态', active ? '进行中' : '已结束'),
                  ],
                ),
              ],
            ),
          ),
        ),
        const SizedBox(height: 16),
        Card(
          child: Padding(
            padding: const EdgeInsets.all(16),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                const Text('赛季奖励', style: TextStyle(fontSize: 16, fontWeight: FontWeight.bold, color: Colors.white)),
                const SizedBox(height: 12),
                _rewardRow('🥇 第1名', '500 星尘'),
                _rewardRow('🥈 第2名', '300 星尘'),
                _rewardRow('🥉 第3名', '200 星尘'),
                _rewardRow('4-5名', '100 星尘'),
                _rewardRow('6-10名', '50 星尘'),
              ],
            ),
          ),
        ),
      ],
    );
  }

  Widget _infoChip(String label, String value) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
      decoration: BoxDecoration(
        color: const Color(0xFF334155),
        borderRadius: BorderRadius.circular(8),
      ),
      child: Text('$label: $value', style: const TextStyle(fontSize: 12, color: Color(0xFF94A3B8))),
    );
  }

  Widget _rewardRow(String rank, String reward) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 4),
      child: Row(
        mainAxisAlignment: MainAxisAlignment.spaceBetween,
        children: [
          Text(rank, style: const TextStyle(color: Colors.white)),
          Text(reward, style: const TextStyle(color: AppTheme.warningColor, fontWeight: FontWeight.w500)),
        ],
      ),
    );
  }

  Widget _buildShopTab() {
    if (_shop.isEmpty) {
      return const Center(child: Text('商店暂无商品', style: TextStyle(color: Color(0xFF64748B))));
    }
    return ListView.builder(
      padding: const EdgeInsets.all(12),
      itemCount: _shop.length,
      itemBuilder: (context, index) {
        final item = _shop[index];
        final name = item['name'] ?? '未知装备';
        final slot = item['slot'] ?? '';
        final price = item['price'] ?? 0;
        final rarity = item['rarity'] ?? 'common';
        final rarityColor = {
          'common': const Color(0xFF94A3B8),
          'uncommon': AppTheme.successColor,
          'rare': AppTheme.primaryColor,
          'epic': AppTheme.secondaryColor,
          'legendary': AppTheme.warningColor,
        }[rarity] ?? const Color(0xFF94A3B8);

        return Card(
          margin: const EdgeInsets.only(bottom: 8),
          child: ListTile(
            leading: CircleAvatar(
              backgroundColor: rarityColor.withOpacity(0.15),
              child: Icon(Icons.shield_rounded, color: rarityColor, size: 20),
            ),
            title: Text(name, style: TextStyle(color: rarityColor, fontWeight: FontWeight.w600)),
            subtitle: Text('部位: $slot', style: const TextStyle(fontSize: 12, color: Color(0xFF94A3B8))),
            trailing: Text('⚡$price', style: const TextStyle(color: AppTheme.warningColor, fontWeight: FontWeight.bold)),
          ),
        );
      },
    );
  }
}
