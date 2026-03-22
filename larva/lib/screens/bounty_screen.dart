import 'package:flutter/material.dart';
import '../services/api_service.dart';
import '../theme.dart';
import 'bounty_detail_screen.dart';

class BountyScreen extends StatefulWidget {
  const BountyScreen({super.key});

  @override
  State<BountyScreen> createState() => _BountyScreenState();
}

class _BountyScreenState extends State<BountyScreen> {
  List<dynamic> _bounties = [];
  bool _loading = true;
  String? _statusFilter;

  static const _statusLabels = {
    null: '全部',
    'open': '开放',
    'claimed': '已领取',
    'delivered': '已交付',
    'completed': '已完成',
  };

  static const _statusColors = {
    'open': AppTheme.successColor,
    'claimed': Color(0xFF3B82F6),
    'delivered': AppTheme.warningColor,
    'completed': Color(0xFF22C55E),
    'disputed': AppTheme.errorColor,
    'cancelled': Color(0xFF6B7280),
    'expired': Color(0xFF6B7280),
  };

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    setState(() => _loading = true);
    try {
      final res = await ApiService().listBounties(status: _statusFilter);
      final data = res.data;
      setState(() {
        _bounties = data['bounties'] ?? [];
        _loading = false;
      });
    } catch (_) {
      if (mounted) setState(() => _loading = false);
    }
  }

  String _formatReward(dynamic amount, String currency) {
    final a = (amount is int ? amount : (amount as num).toInt());
    if (currency == 'CNY' || currency == 'cny') {
      return '¥${(a / 100).toStringAsFixed(2)}';
    }
    return '$a $currency';
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
    return SafeArea(
      child: Column(
        children: [
          // Header
          Padding(
            padding: const EdgeInsets.fromLTRB(20, 16, 20, 0),
            child: Row(
              children: [
                Container(
                  width: 36,
                  height: 36,
                  decoration: BoxDecoration(
                    gradient: const LinearGradient(
                      colors: [AppTheme.successColor, Color(0xFF14B8A6)],
                    ),
                    borderRadius: BorderRadius.circular(10),
                  ),
                  child: const Icon(
                    Icons.emoji_events_rounded,
                    color: Colors.white,
                    size: 18,
                  ),
                ),
                const SizedBox(width: 10),
                const Text(
                  '赏金市场',
                  style: TextStyle(
                    fontSize: 20,
                    fontWeight: FontWeight.bold,
                    color: Colors.white,
                  ),
                ),
              ],
            ),
          ),
          const SizedBox(height: 14),

          // Filters
          SizedBox(
            height: 34,
            child: ListView(
              scrollDirection: Axis.horizontal,
              padding: const EdgeInsets.symmetric(horizontal: 20),
              children: _statusLabels.entries.map((e) {
                final selected = _statusFilter == e.key;
                return Padding(
                  padding: const EdgeInsets.only(right: 8),
                  child: GestureDetector(
                    onTap: () {
                      setState(() => _statusFilter = e.key);
                      _load();
                    },
                    child: Container(
                      padding: const EdgeInsets.symmetric(
                        horizontal: 14,
                        vertical: 6,
                      ),
                      decoration: BoxDecoration(
                        color: selected
                            ? AppTheme.primaryColor
                            : const Color(0xFF1E293B),
                        borderRadius: BorderRadius.circular(20),
                      ),
                      child: Text(
                        e.value,
                        style: TextStyle(
                          color: selected ? Colors.white : Colors.grey[400],
                          fontSize: 13,
                          fontWeight: selected
                              ? FontWeight.w600
                              : FontWeight.normal,
                        ),
                      ),
                    ),
                  ),
                );
              }).toList(),
            ),
          ),
          const SizedBox(height: 12),

          // List
          Expanded(
            child: _loading
                ? const Center(child: CircularProgressIndicator())
                : _bounties.isEmpty
                ? Center(
                    child: Column(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Icon(
                          Icons.emoji_events_outlined,
                          size: 48,
                          color: Colors.grey[700],
                        ),
                        const SizedBox(height: 12),
                        Text(
                          '暂无赏金任务',
                          style: TextStyle(color: Colors.grey[500]),
                        ),
                      ],
                    ),
                  )
                : RefreshIndicator(
                    onRefresh: _load,
                    child: ListView.builder(
                      padding: const EdgeInsets.symmetric(horizontal: 20),
                      itemCount: _bounties.length,
                      itemBuilder: (ctx, i) => _buildBountyCard(_bounties[i]),
                    ),
                  ),
          ),
        ],
      ),
    );
  }

  Widget _buildBountyCard(Map<String, dynamic> b) {
    final status = b['status'] ?? 'open';
    final statusColor = _statusColors[status] ?? Colors.grey;
    final statusLabel = _statusLabels[status] ?? status;

    return GestureDetector(
      onTap: () async {
        await Navigator.push(
          context,
          MaterialPageRoute(
            builder: (_) => BountyDetailScreen(bountyId: b['id']),
          ),
        );
        _load();
      },
      child: Container(
        margin: const EdgeInsets.only(bottom: 10),
        padding: const EdgeInsets.all(16),
        decoration: BoxDecoration(
          color: const Color(0xFF1E293B),
          borderRadius: BorderRadius.circular(14),
        ),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Container(
                  padding: const EdgeInsets.symmetric(
                    horizontal: 8,
                    vertical: 3,
                  ),
                  decoration: BoxDecoration(
                    color: statusColor.withValues(alpha: 0.15),
                    borderRadius: BorderRadius.circular(6),
                  ),
                  child: Text(
                    statusLabel,
                    style: TextStyle(
                      color: statusColor,
                      fontSize: 11,
                      fontWeight: FontWeight.w600,
                    ),
                  ),
                ),
                const Spacer(),
                Text(
                  _formatReward(
                    b['reward_amount'] ?? 0,
                    b['reward_currency'] ?? 'CNY',
                  ),
                  style: TextStyle(
                    color: AppTheme.successColor,
                    fontSize: 18,
                    fontWeight: FontWeight.bold,
                  ),
                ),
              ],
            ),
            const SizedBox(height: 10),
            Text(
              b['title'] ?? '',
              style: const TextStyle(
                color: Colors.white,
                fontSize: 15,
                fontWeight: FontWeight.w600,
              ),
              maxLines: 2,
              overflow: TextOverflow.ellipsis,
            ),
            const SizedBox(height: 6),
            Text(
              b['description'] ?? '',
              style: TextStyle(color: Colors.grey[500], fontSize: 13),
              maxLines: 2,
              overflow: TextOverflow.ellipsis,
            ),
            const SizedBox(height: 10),
            Row(
              children: [
                Icon(Icons.person_outline, size: 14, color: Colors.grey[600]),
                const SizedBox(width: 4),
                Text(
                  b['creator_name'] ?? '',
                  style: TextStyle(color: Colors.grey[500], fontSize: 12),
                ),
                const SizedBox(width: 16),
                Icon(Icons.access_time, size: 14, color: Colors.grey[600]),
                const SizedBox(width: 4),
                Text(
                  _timeAgo(b['created_at']),
                  style: TextStyle(color: Colors.grey[500], fontSize: 12),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }
}
