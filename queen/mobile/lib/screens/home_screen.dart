import 'package:flutter/material.dart';
import '../services/api_service.dart';
import '../services/auth_service.dart';
import '../theme.dart';

class HomeScreen extends StatefulWidget {
  const HomeScreen({super.key});

  @override
  State<HomeScreen> createState() => _HomeScreenState();
}

class _HomeScreenState extends State<HomeScreen> {
  Map<String, dynamic>? _balance;
  Map<String, dynamic>? _bountyStats;
  bool _loading = true;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    try {
      final balRes = await ApiService().getBalance();
      final btyRes = await ApiService().getBountyStats();
      if (mounted) {
        setState(() {
          _balance = balRes.data is Map ? balRes.data : null;
          _bountyStats = btyRes.data is Map ? btyRes.data : null;
          _loading = false;
        });
      }
    } catch (_) {
      if (mounted) setState(() => _loading = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final username = AuthService().username ?? '用户';

    return SafeArea(
      child: RefreshIndicator(
        onRefresh: _load,
        child: ListView(
          padding: const EdgeInsets.all(20),
          children: [
            const SizedBox(height: 8),
            // Greeting
            Row(
              children: [
                Container(
                  width: 48,
                  height: 48,
                  decoration: BoxDecoration(
                    gradient: const LinearGradient(
                      colors: [AppTheme.primaryColor, AppTheme.secondaryColor],
                    ),
                    borderRadius: BorderRadius.circular(14),
                  ),
                  child: Center(
                    child: Text(
                      username.isNotEmpty ? username[0].toUpperCase() : 'U',
                      style: const TextStyle(
                        color: Colors.white,
                        fontSize: 20,
                        fontWeight: FontWeight.bold,
                      ),
                    ),
                  ),
                ),
                const SizedBox(width: 14),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        '你好，$username',
                        style: const TextStyle(
                          fontSize: 20,
                          fontWeight: FontWeight.bold,
                          color: Colors.white,
                        ),
                      ),
                      const SizedBox(height: 2),
                      Text(
                        '欢迎回到 StarClaw',
                        style: TextStyle(
                          color: Colors.grey[400],
                          fontSize: 13,
                        ),
                      ),
                    ],
                  ),
                ),
              ],
            ),
            const SizedBox(height: 28),

            // Stats cards
            if (_loading)
              const Center(
                child: Padding(
                  padding: EdgeInsets.all(40),
                  child: CircularProgressIndicator(),
                ),
              )
            else ...[
              // Balance card
              _buildCard(
                icon: Icons.account_balance_wallet_rounded,
                iconColor: AppTheme.accentColor,
                title: '账户余额',
                value: '¥${((_balance?['balance'] ?? 0) / 100).toStringAsFixed(2)}',
                subtitle: '冻结: ¥${((_balance?['frozen'] ?? 0) / 100).toStringAsFixed(2)}',
              ),
              const SizedBox(height: 12),

              // Bounty stats
              Row(
                children: [
                  Expanded(
                    child: _buildSmallCard(
                      '开放赏金',
                      '${_bountyStats?['open'] ?? 0}',
                      AppTheme.successColor,
                    ),
                  ),
                  const SizedBox(width: 12),
                  Expanded(
                    child: _buildSmallCard(
                      '已完成',
                      '${_bountyStats?['completed'] ?? 0}',
                      AppTheme.primaryColor,
                    ),
                  ),
                  const SizedBox(width: 12),
                  Expanded(
                    child: _buildSmallCard(
                      '赏金池',
                      '¥${((_bountyStats?['total_reward'] ?? 0) / 100).toStringAsFixed(0)}',
                      AppTheme.warningColor,
                    ),
                  ),
                ],
              ),
              const SizedBox(height: 28),

              // Quick actions
              const Text(
                '快速操作',
                style: TextStyle(
                  fontSize: 16,
                  fontWeight: FontWeight.w600,
                  color: Colors.white,
                ),
              ),
              const SizedBox(height: 12),
              _buildActionTile(
                icon: Icons.emoji_events_rounded,
                color: AppTheme.warningColor,
                title: '浏览赏金市场',
                sub: 'AI 做不了的事，悬赏让人类来做',
              ),
              const SizedBox(height: 8),
              _buildActionTile(
                icon: Icons.forum_rounded,
                color: AppTheme.accentColor,
                title: '社区论坛',
                sub: '分享 Agent 玩法、工作流、技术讨论',
              ),
              const SizedBox(height: 8),
              _buildActionTile(
                icon: Icons.devices_rounded,
                color: AppTheme.successColor,
                title: '管理我的节点',
                sub: '绑定和监控 Claw 小龙虾节点',
              ),
            ],
          ],
        ),
      ),
    );
  }

  Widget _buildCard({
    required IconData icon,
    required Color iconColor,
    required String title,
    required String value,
    String? subtitle,
  }) {
    return Container(
      padding: const EdgeInsets.all(20),
      decoration: BoxDecoration(
        color: const Color(0xFF1E293B),
        borderRadius: BorderRadius.circular(16),
      ),
      child: Row(
        children: [
          Container(
            width: 44,
            height: 44,
            decoration: BoxDecoration(
              color: iconColor.withOpacity(0.15),
              borderRadius: BorderRadius.circular(12),
            ),
            child: Icon(icon, color: iconColor, size: 22),
          ),
          const SizedBox(width: 16),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(title, style: TextStyle(color: Colors.grey[400], fontSize: 12)),
                const SizedBox(height: 4),
                Text(value, style: const TextStyle(color: Colors.white, fontSize: 22, fontWeight: FontWeight.bold)),
                if (subtitle != null)
                  Padding(
                    padding: const EdgeInsets.only(top: 2),
                    child: Text(subtitle, style: TextStyle(color: Colors.grey[500], fontSize: 11)),
                  ),
              ],
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildSmallCard(String label, String value, Color color) {
    return Container(
      padding: const EdgeInsets.all(14),
      decoration: BoxDecoration(
        color: const Color(0xFF1E293B),
        borderRadius: BorderRadius.circular(14),
      ),
      child: Column(
        children: [
          Text(value, style: TextStyle(color: color, fontSize: 20, fontWeight: FontWeight.bold)),
          const SizedBox(height: 4),
          Text(label, style: TextStyle(color: Colors.grey[500], fontSize: 11)),
        ],
      ),
    );
  }

  Widget _buildActionTile({
    required IconData icon,
    required Color color,
    required String title,
    required String sub,
  }) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 14),
      decoration: BoxDecoration(
        color: const Color(0xFF1E293B),
        borderRadius: BorderRadius.circular(14),
      ),
      child: Row(
        children: [
          Container(
            width: 40,
            height: 40,
            decoration: BoxDecoration(
              color: color.withOpacity(0.12),
              borderRadius: BorderRadius.circular(10),
            ),
            child: Icon(icon, color: color, size: 20),
          ),
          const SizedBox(width: 14),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(title, style: const TextStyle(color: Colors.white, fontSize: 14, fontWeight: FontWeight.w600)),
                const SizedBox(height: 2),
                Text(sub, style: TextStyle(color: Colors.grey[500], fontSize: 12)),
              ],
            ),
          ),
          Icon(Icons.chevron_right_rounded, color: Colors.grey[600], size: 20),
        ],
      ),
    );
  }
}
