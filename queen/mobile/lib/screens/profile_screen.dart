import 'package:flutter/material.dart';
import '../services/api_service.dart';
import '../services/auth_service.dart';
import '../theme.dart';

class ProfileScreen extends StatefulWidget {
  const ProfileScreen({super.key});

  @override
  State<ProfileScreen> createState() => _ProfileScreenState();
}

class _ProfileScreenState extends State<ProfileScreen> {
  Map<String, dynamic>? _profile;
  List<dynamic> _nodes = [];
  bool _loading = true;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    try {
      final results = await Future.wait([
        ApiService().getProfile(),
        ApiService().listNodes(),
      ]);
      if (mounted) {
        setState(() {
          final profileData = results[0].data;
          _profile = profileData is Map
              ? (profileData['user'] ?? profileData) as Map<String, dynamic>
              : null;
          final nodeData = results[1].data;
          _nodes = nodeData is Map
              ? (nodeData['data']?['nodes'] ?? nodeData['nodes'] ?? [])
              : [];
          _loading = false;
        });
      }
    } catch (_) {
      if (mounted) setState(() => _loading = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final auth = AuthService();

    return SafeArea(
      child: _loading
          ? const Center(child: CircularProgressIndicator())
          : RefreshIndicator(
              onRefresh: _load,
              child: ListView(
                padding: const EdgeInsets.all(20),
                children: [
                  const SizedBox(height: 8),
                  // Avatar + name
                  Center(
                    child: Column(
                      children: [
                        Container(
                          width: 72,
                          height: 72,
                          decoration: BoxDecoration(
                            gradient: const LinearGradient(
                              colors: [
                                AppTheme.primaryColor,
                                AppTheme.secondaryColor,
                              ],
                            ),
                            borderRadius: BorderRadius.circular(22),
                          ),
                          child: Center(
                            child: Text(
                              (auth.username ?? 'U').isNotEmpty
                                  ? (auth.username ?? 'U')[0].toUpperCase()
                                  : 'U',
                              style: const TextStyle(
                                color: Colors.white,
                                fontSize: 28,
                                fontWeight: FontWeight.bold,
                              ),
                            ),
                          ),
                        ),
                        const SizedBox(height: 12),
                        Text(
                          _profile?['nickname'] ?? auth.username ?? '用户',
                          style: const TextStyle(
                            color: Colors.white,
                            fontSize: 20,
                            fontWeight: FontWeight.bold,
                          ),
                        ),
                        const SizedBox(height: 4),
                        Text(
                          _profile?['email'] ?? auth.email ?? '',
                          style: TextStyle(
                            color: Colors.grey[500],
                            fontSize: 13,
                          ),
                        ),
                        if (_profile?['bio'] != null &&
                            (_profile!['bio'] as String).isNotEmpty) ...[
                          const SizedBox(height: 6),
                          Text(
                            _profile!['bio'],
                            style: TextStyle(
                              color: Colors.grey[400],
                              fontSize: 13,
                            ),
                            textAlign: TextAlign.center,
                          ),
                        ],
                      ],
                    ),
                  ),
                  const SizedBox(height: 28),

                  // My nodes
                  Row(
                    children: [
                      const Text(
                        '我的节点',
                        style: TextStyle(
                          color: Colors.white,
                          fontSize: 16,
                          fontWeight: FontWeight.w600,
                        ),
                      ),
                      const Spacer(),
                      Text(
                        '${_nodes.length} 个',
                        style: TextStyle(color: Colors.grey[500], fontSize: 13),
                      ),
                    ],
                  ),
                  const SizedBox(height: 12),
                  if (_nodes.isEmpty)
                    Container(
                      padding: const EdgeInsets.all(24),
                      decoration: BoxDecoration(
                        color: const Color(0xFF1E293B),
                        borderRadius: BorderRadius.circular(14),
                      ),
                      child: Center(
                        child: Column(
                          children: [
                            Icon(
                              Icons.devices_rounded,
                              size: 36,
                              color: Colors.grey[700],
                            ),
                            const SizedBox(height: 8),
                            Text(
                              '暂无绑定节点',
                              style: TextStyle(
                                color: Colors.grey[500],
                                fontSize: 13,
                              ),
                            ),
                          ],
                        ),
                      ),
                    )
                  else
                    ..._nodes.map(
                      (n) => _buildNodeCard(n as Map<String, dynamic>),
                    ),
                  const SizedBox(height: 28),

                  // Menu items
                  _buildMenuItem(
                    Icons.person_outline,
                    '编辑资料',
                    _showEditProfile,
                  ),
                  _buildMenuItem(
                    Icons.lock_outline,
                    '修改密码',
                    _showChangePassword,
                  ),
                  _buildMenuItem(Icons.devices_rounded, '绑定节点', _showBindNode),
                  _buildMenuItem(Icons.flag_outlined, '我的举报', () {}),
                  _buildMenuItem(Icons.info_outline, '关于 StarClaw', _showAbout),
                  const SizedBox(height: 20),
                  // Logout
                  SizedBox(
                    width: double.infinity,
                    child: TextButton(
                      onPressed: () async {
                        await AuthService().logout();
                      },
                      child: const Text(
                        '退出登录',
                        style: TextStyle(
                          color: AppTheme.errorColor,
                          fontSize: 15,
                        ),
                      ),
                    ),
                  ),
                ],
              ),
            ),
    );
  }

  Widget _buildNodeCard(Map<String, dynamic> node) {
    final lastSeen = node['last_seen'] as String?;
    final isOnline =
        lastSeen != null &&
        DateTime.now()
                .difference(DateTime.tryParse(lastSeen) ?? DateTime(2000))
                .inMinutes <
            5;

    return Container(
      margin: const EdgeInsets.only(bottom: 8),
      padding: const EdgeInsets.all(14),
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
              color: (isOnline ? AppTheme.successColor : Colors.grey)
                  .withValues(alpha: 0.12),
              borderRadius: BorderRadius.circular(10),
            ),
            child: Icon(
              isOnline ? Icons.wifi_rounded : Icons.wifi_off_rounded,
              color: isOnline ? AppTheme.successColor : Colors.grey,
              size: 20,
            ),
          ),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  children: [
                    Text(
                      node['node_name'] ?? '未命名',
                      style: const TextStyle(
                        color: Colors.white,
                        fontSize: 14,
                        fontWeight: FontWeight.w600,
                      ),
                    ),
                    const SizedBox(width: 6),
                    Container(
                      padding: const EdgeInsets.symmetric(
                        horizontal: 6,
                        vertical: 2,
                      ),
                      decoration: BoxDecoration(
                        color: (isOnline ? AppTheme.successColor : Colors.grey)
                            .withValues(alpha: 0.15),
                        borderRadius: BorderRadius.circular(4),
                      ),
                      child: Text(
                        isOnline ? '在线' : '离线',
                        style: TextStyle(
                          color: isOnline ? AppTheme.successColor : Colors.grey,
                          fontSize: 10,
                        ),
                      ),
                    ),
                    if (node['node_version'] != null) ...[
                      const SizedBox(width: 4),
                      Text(
                        node['node_version'],
                        style: TextStyle(color: Colors.grey[600], fontSize: 10),
                      ),
                    ],
                  ],
                ),
                const SizedBox(height: 2),
                Text(
                  node['node_id'] ?? '',
                  style: TextStyle(
                    color: Colors.grey[600],
                    fontSize: 11,
                    fontFamily: 'monospace',
                  ),
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }

  void _showEditProfile() {
    final nicknameCtrl = TextEditingController(
      text: _profile?['nickname'] ?? '',
    );
    final bioCtrl = TextEditingController(text: _profile?['bio'] ?? '');
    showModalBottomSheet(
      context: context,
      backgroundColor: const Color(0xFF1E293B),
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(20)),
      ),
      isScrollControlled: true,
      builder: (ctx) => Padding(
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
            const Text(
              '编辑资料',
              style: TextStyle(
                color: Colors.white,
                fontSize: 18,
                fontWeight: FontWeight.bold,
              ),
            ),
            const SizedBox(height: 16),
            TextField(
              controller: nicknameCtrl,
              decoration: const InputDecoration(hintText: '昵称'),
            ),
            const SizedBox(height: 12),
            TextField(
              controller: bioCtrl,
              maxLines: 3,
              decoration: const InputDecoration(hintText: '个人简介'),
            ),
            const SizedBox(height: 16),
            SizedBox(
              width: double.infinity,
              child: ElevatedButton(
                onPressed: () async {
                  Navigator.pop(ctx);
                  try {
                    await ApiService().updateProfile(
                      nickname: nicknameCtrl.text.trim().isNotEmpty
                          ? nicknameCtrl.text.trim()
                          : null,
                      bio: bioCtrl.text.trim(),
                    );
                    _load();
                    if (mounted) {
                      ScaffoldMessenger.of(
                        context,
                      ).showSnackBar(const SnackBar(content: Text('资料已更新')));
                    }
                  } catch (_) {
                    if (mounted) {
                      ScaffoldMessenger.of(
                        context,
                      ).showSnackBar(const SnackBar(content: Text('更新失败')));
                    }
                  }
                },
                child: const Text('保存'),
              ),
            ),
          ],
        ),
      ),
    );
  }

  void _showChangePassword() {
    final oldCtrl = TextEditingController();
    final newCtrl = TextEditingController();
    showModalBottomSheet(
      context: context,
      backgroundColor: const Color(0xFF1E293B),
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(20)),
      ),
      isScrollControlled: true,
      builder: (ctx) => Padding(
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
            const Text(
              '修改密码',
              style: TextStyle(
                color: Colors.white,
                fontSize: 18,
                fontWeight: FontWeight.bold,
              ),
            ),
            const SizedBox(height: 16),
            TextField(
              controller: oldCtrl,
              obscureText: true,
              decoration: const InputDecoration(hintText: '当前密码'),
            ),
            const SizedBox(height: 12),
            TextField(
              controller: newCtrl,
              obscureText: true,
              decoration: const InputDecoration(hintText: '新密码'),
            ),
            const SizedBox(height: 16),
            SizedBox(
              width: double.infinity,
              child: ElevatedButton(
                onPressed: () async {
                  if (oldCtrl.text.isEmpty || newCtrl.text.isEmpty) return;
                  Navigator.pop(ctx);
                  try {
                    await ApiService().changePassword(
                      oldCtrl.text,
                      newCtrl.text,
                    );
                    if (mounted) {
                      ScaffoldMessenger.of(
                        context,
                      ).showSnackBar(const SnackBar(content: Text('密码已修改')));
                    }
                  } catch (_) {
                    if (mounted) {
                      ScaffoldMessenger.of(context).showSnackBar(
                        const SnackBar(content: Text('修改失败，请检查旧密码')),
                      );
                    }
                  }
                },
                child: const Text('确认修改'),
              ),
            ),
          ],
        ),
      ),
    );
  }

  void _showBindNode() {
    final nodeIdCtrl = TextEditingController();
    final localUserCtrl = TextEditingController();
    final nameCtrl = TextEditingController();
    showModalBottomSheet(
      context: context,
      backgroundColor: const Color(0xFF1E293B),
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(20)),
      ),
      isScrollControlled: true,
      builder: (ctx) => Padding(
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
            const Text(
              '绑定节点',
              style: TextStyle(
                color: Colors.white,
                fontSize: 18,
                fontWeight: FontWeight.bold,
              ),
            ),
            const SizedBox(height: 6),
            Text(
              '将你的 Claw 小龙虾节点绑定到 Queen 账号',
              style: TextStyle(color: Colors.grey[500], fontSize: 12),
            ),
            const SizedBox(height: 16),
            TextField(
              controller: nodeIdCtrl,
              decoration: const InputDecoration(
                hintText: '节点 ID（如 claw:b49edd...）',
              ),
            ),
            const SizedBox(height: 12),
            TextField(
              controller: localUserCtrl,
              decoration: const InputDecoration(hintText: '节点上的用户 ID'),
            ),
            const SizedBox(height: 12),
            TextField(
              controller: nameCtrl,
              decoration: const InputDecoration(hintText: '备注名称（可选）'),
            ),
            const SizedBox(height: 16),
            SizedBox(
              width: double.infinity,
              child: ElevatedButton(
                onPressed: () async {
                  if (nodeIdCtrl.text.trim().isEmpty ||
                      localUserCtrl.text.trim().isEmpty) {
                    return;
                  }
                  Navigator.pop(ctx);
                  try {
                    await ApiService().bindNode(
                      nodeId: nodeIdCtrl.text.trim(),
                      localUserId: localUserCtrl.text.trim(),
                      nodeName: nameCtrl.text.trim().isNotEmpty
                          ? nameCtrl.text.trim()
                          : null,
                    );
                    _load();
                    if (mounted) {
                      ScaffoldMessenger.of(
                        context,
                      ).showSnackBar(const SnackBar(content: Text('节点绑定成功')));
                    }
                  } catch (_) {
                    if (mounted) {
                      ScaffoldMessenger.of(
                        context,
                      ).showSnackBar(const SnackBar(content: Text('绑定失败')));
                    }
                  }
                },
                child: const Text('确认绑定'),
              ),
            ),
          ],
        ),
      ),
    );
  }

  void _showAbout() {
    showDialog(
      context: context,
      builder: (ctx) => AlertDialog(
        backgroundColor: const Color(0xFF1E293B),
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(16)),
        title: const Text('关于 StarClaw', style: TextStyle(color: Colors.white)),
        content: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(
              'StarClaw 是一个去中心化 AI 创作平台',
              style: TextStyle(color: Colors.grey[400], fontSize: 13),
            ),
            const SizedBox(height: 12),
            Text(
              '版本: 1.0.0',
              style: TextStyle(color: Colors.grey[500], fontSize: 12),
            ),
            Text(
              '架构: Queen 👑 / Overlord 👁️ / Claw 🦞',
              style: TextStyle(color: Colors.grey[500], fontSize: 12),
            ),
          ],
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(ctx),
            child: const Text('关闭'),
          ),
        ],
      ),
    );
  }

  Widget _buildMenuItem(IconData icon, String title, VoidCallback onTap) {
    return InkWell(
      onTap: onTap,
      borderRadius: BorderRadius.circular(12),
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 14),
        margin: const EdgeInsets.only(bottom: 4),
        child: Row(
          children: [
            Icon(icon, color: Colors.grey[400], size: 20),
            const SizedBox(width: 14),
            Expanded(
              child: Text(
                title,
                style: const TextStyle(color: Colors.white, fontSize: 14),
              ),
            ),
            Icon(
              Icons.chevron_right_rounded,
              color: Colors.grey[700],
              size: 20,
            ),
          ],
        ),
      ),
    );
  }
}
