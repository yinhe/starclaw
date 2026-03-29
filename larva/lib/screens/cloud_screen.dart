import 'package:flutter/material.dart';
import '../services/api_service.dart';
import '../theme.dart';

class CloudScreen extends StatefulWidget {
  const CloudScreen({super.key});

  @override
  State<CloudScreen> createState() => _CloudScreenState();
}

class _CloudScreenState extends State<CloudScreen> {
  List<dynamic> _plans = [];
  List<dynamic> _instances = [];
  bool _loading = true;
  bool _creating = false;
  final _slugController = TextEditingController();
  String? _selectedPlanId;

  static const _modeLabels = {'lite': '🔥 Spark', 'hive': '⚡ Pulse', 'ecs': '🚀 Surge'};
  static const _statusColors = {
    'running': AppTheme.successColor,
    'creating': AppTheme.warningColor,
    'stopped': Color(0xFF64748B),
    'error': AppTheme.errorColor,
  };

  @override
  void initState() {
    super.initState();
    _load();
  }

  @override
  void dispose() {
    _slugController.dispose();
    super.dispose();
  }

  Future<void> _load() async {
    setState(() => _loading = true);
    try {
      dynamic plansRes, instancesRes;
      try { plansRes = await ApiService().getCloudPlans(); } catch (_) {}
      try { instancesRes = await ApiService().getCloudInstances(); } catch (_) {}
      if (mounted) {
        setState(() {
          final pd = plansRes?.data;
          _plans = pd is Map ? (pd['plans'] ?? []) : (pd is List ? pd : []);
          final id = instancesRes?.data;
          _instances = id is Map ? (id['instances'] ?? []) : (id is List ? id : []);
          _loading = false;
        });
      }
    } catch (_) {
      if (mounted) setState(() => _loading = false);
    }
  }

  Future<void> _create() async {
    final slug = _slugController.text.trim().toLowerCase();
    if (slug.isEmpty) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('请输入子域名')),
      );
      return;
    }
    setState(() => _creating = true);
    try {
      await ApiService().createCloudInstance(
        slug: slug,
        planId: _selectedPlanId,
      );
      _slugController.clear();
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('✅ 实例创建中...')),
      );
      await Future.delayed(const Duration(seconds: 2));
      _load();
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('创建失败: $e')),
        );
      }
    }
    if (mounted) setState(() => _creating = false);
  }

  @override
  Widget build(BuildContext context) {
    return SafeArea(
      child: RefreshIndicator(
        onRefresh: _load,
        child: _loading
            ? const Center(child: CircularProgressIndicator())
            : ListView(
                padding: const EdgeInsets.all(16),
                children: [
                  // Header
                  const Row(
                    children: [
                      Text('🐝', style: TextStyle(fontSize: 24)),
                      SizedBox(width: 8),
                      Text('云船队', style: TextStyle(fontSize: 20, fontWeight: FontWeight.bold, color: Colors.white)),
                    ],
                  ),
                  const SizedBox(height: 4),
                  const Text('一键创建你的 AI 智能体节点', style: TextStyle(fontSize: 12, color: Color(0xFF64748B))),
                  const SizedBox(height: 16),

                  // Plans
                  SizedBox(
                    height: 160,
                    child: ListView.builder(
                      scrollDirection: Axis.horizontal,
                      itemCount: _plans.length,
                      itemBuilder: (context, index) {
                        final p = _plans[index];
                        final id = p['id'] ?? '';
                        final selected = _selectedPlanId == id;
                        final price = p['price_monthly'] ?? 0;
                        final mode = p['deploy_mode'] ?? '';
                        return GestureDetector(
                          onTap: () => setState(() => _selectedPlanId = id),
                          child: Container(
                            width: 140,
                            margin: const EdgeInsets.only(right: 10),
                            padding: const EdgeInsets.all(12),
                            decoration: BoxDecoration(
                              color: const Color(0xFF1E293B),
                              borderRadius: BorderRadius.circular(14),
                              border: Border.all(
                                color: selected ? AppTheme.primaryColor : const Color(0xFF334155),
                                width: selected ? 2 : 1,
                              ),
                            ),
                            child: Column(
                              crossAxisAlignment: CrossAxisAlignment.start,
                              children: [
                                Text(p['display_name'] ?? id, style: const TextStyle(fontSize: 14, fontWeight: FontWeight.bold, color: Colors.white)),
                                const SizedBox(height: 4),
                                Text(_modeLabels[mode] ?? mode, style: const TextStyle(fontSize: 10, color: Color(0xFF94A3B8))),
                                const Spacer(),
                                Text(
                                  price > 0 ? '⚡$price/月' : '免费',
                                  style: TextStyle(
                                    fontSize: 16,
                                    fontWeight: FontWeight.bold,
                                    color: price > 0 ? AppTheme.accentColor : AppTheme.successColor,
                                  ),
                                ),
                                const SizedBox(height: 4),
                                Text('${p['cpu'] ?? 0}核 · ${p['memory_mb'] ?? 0}MB', style: const TextStyle(fontSize: 10, color: Color(0xFF64748B))),
                                if (selected)
                                  const Padding(
                                    padding: EdgeInsets.only(top: 4),
                                    child: Text('✓ 已选', style: TextStyle(fontSize: 10, color: AppTheme.primaryColor)),
                                  ),
                              ],
                            ),
                          ),
                        );
                      },
                    ),
                  ),
                  const SizedBox(height: 16),

                  // Create form
                  Card(
                    child: Padding(
                      padding: const EdgeInsets.all(16),
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          const Text('创建实例', style: TextStyle(fontSize: 16, fontWeight: FontWeight.bold, color: Colors.white)),
                          const SizedBox(height: 12),
                          Row(
                            children: [
                              Expanded(
                                child: TextField(
                                  controller: _slugController,
                                  decoration: const InputDecoration(hintText: 'my-claw'),
                                  style: const TextStyle(color: Colors.white),
                                ),
                              ),
                              const SizedBox(width: 8),
                              const Text('.starclaw.me', style: TextStyle(fontSize: 12, color: Color(0xFF64748B))),
                            ],
                          ),
                          const SizedBox(height: 12),
                          SizedBox(
                            width: double.infinity,
                            child: ElevatedButton(
                              onPressed: _creating ? null : _create,
                              child: Text(_creating ? '创建中...' : '🚀 创建'),
                            ),
                          ),
                        ],
                      ),
                    ),
                  ),
                  const SizedBox(height: 20),

                  // My instances
                  const Text('我的实例', style: TextStyle(fontSize: 16, fontWeight: FontWeight.bold, color: Colors.white)),
                  const SizedBox(height: 8),
                  if (_instances.isEmpty)
                    const Card(
                      child: Padding(
                        padding: EdgeInsets.all(32),
                        child: Center(child: Text('还没有实例', style: TextStyle(color: Color(0xFF64748B)))),
                      ),
                    )
                  else
                    ..._instances.map(_buildInstanceCard),
                ],
              ),
      ),
    );
  }

  Widget _buildInstanceCard(dynamic inst) {
    final slug = inst['slug'] ?? '';
    final status = inst['status'] ?? 'unknown';
    final mode = inst['deploy_mode'] ?? '';
    final statusColor = _statusColors[status] ?? const Color(0xFF64748B);

    return Card(
      margin: const EdgeInsets.only(bottom: 10),
      child: Padding(
        padding: const EdgeInsets.all(14),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Container(
                  width: 8, height: 8,
                  decoration: BoxDecoration(color: statusColor, shape: BoxShape.circle),
                ),
                const SizedBox(width: 8),
                Expanded(
                  child: Text(
                    inst['display_name'] ?? slug,
                    style: const TextStyle(fontSize: 15, fontWeight: FontWeight.w600, color: Colors.white),
                  ),
                ),
                Container(
                  padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
                  decoration: BoxDecoration(
                    color: statusColor.withOpacity(0.15),
                    borderRadius: BorderRadius.circular(6),
                  ),
                  child: Text(status, style: TextStyle(fontSize: 10, color: statusColor, fontWeight: FontWeight.w500)),
                ),
              ],
            ),
            const SizedBox(height: 6),
            Text('$slug.starclaw.me · ${_modeLabels[mode] ?? mode}',
                style: const TextStyle(fontSize: 11, color: Color(0xFF64748B))),
            if (status == 'running' || status == 'stopped')
              Padding(
                padding: const EdgeInsets.only(top: 10),
                child: Row(
                  children: [
                    if (status == 'running') ...[
                      _actionBtn('停止', Icons.stop_rounded, () async {
                        await ApiService().stopCloudInstance(slug);
                        _load();
                      }),
                      const SizedBox(width: 8),
                    ],
                    if (status == 'stopped')
                      _actionBtn('启动', Icons.play_arrow_rounded, () async {
                        await ApiService().startCloudInstance(slug);
                        _load();
                      }),
                  ],
                ),
              ),
          ],
        ),
      ),
    );
  }

  Widget _actionBtn(String label, IconData icon, VoidCallback onTap) {
    return InkWell(
      onTap: onTap,
      borderRadius: BorderRadius.circular(8),
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
        decoration: BoxDecoration(
          color: const Color(0xFF334155),
          borderRadius: BorderRadius.circular(8),
        ),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(icon, size: 14, color: const Color(0xFF94A3B8)),
            const SizedBox(width: 4),
            Text(label, style: const TextStyle(fontSize: 12, color: Color(0xFF94A3B8))),
          ],
        ),
      ),
    );
  }
}
