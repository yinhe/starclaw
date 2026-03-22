import 'package:flutter/material.dart';
import '../services/api_service.dart';
import '../theme.dart';

class WorkflowScreen extends StatefulWidget {
  const WorkflowScreen({super.key});

  @override
  State<WorkflowScreen> createState() => _WorkflowScreenState();
}

class _WorkflowScreenState extends State<WorkflowScreen> with SingleTickerProviderStateMixin {
  late TabController _tabController;
  List<Map<String, dynamic>> _templates = [];
  List<Map<String, dynamic>> _runs = [];
  bool _loadingTemplates = true;
  bool _loadingRuns = true;

  @override
  void initState() {
    super.initState();
    _tabController = TabController(length: 2, vsync: this);
    _loadTemplates();
    _loadRuns();
  }

  @override
  void dispose() {
    _tabController.dispose();
    super.dispose();
  }

  Future<void> _loadTemplates() async {
    try {
      final res = await ApiService().dio.get('/marketplace/items', queryParameters: {'category': 'workflow'});
      if (mounted) {
        setState(() {
          _templates = _parseList(res.data, 'items');
          _loadingTemplates = false;
        });
      }
    } catch (_) {
      if (mounted) setState(() => _loadingTemplates = false);
    }
  }

  Future<void> _loadRuns() async {
    try {
      final res = await ApiService().dio.get('/user/workflow/runs');
      if (mounted) {
        setState(() {
          _runs = _parseList(res.data, 'runs');
          _loadingRuns = false;
        });
      }
    } catch (_) {
      if (mounted) setState(() => _loadingRuns = false);
    }
  }

  List<Map<String, dynamic>> _parseList(dynamic data, String key) {
    if (data is Map && data[key] is List) {
      return (data[key] as List).map((e) => e is Map<String, dynamic> ? e : <String, dynamic>{}).toList();
    }
    if (data is List) {
      return data.map((e) => e is Map<String, dynamic> ? e : <String, dynamic>{}).toList();
    }
    return [];
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
                const Icon(Icons.account_tree_rounded, color: AppTheme.secondaryColor, size: 22),
                const SizedBox(width: 10),
                const Text('工作流', style: TextStyle(color: Colors.white, fontSize: 20, fontWeight: FontWeight.bold)),
              ],
            ),
          ),
          const SizedBox(height: 12),

          // Tabs
          Container(
            margin: const EdgeInsets.symmetric(horizontal: 20),
            decoration: BoxDecoration(
              color: const Color(0xFF1E293B),
              borderRadius: BorderRadius.circular(10),
            ),
            child: TabBar(
              controller: _tabController,
              indicator: BoxDecoration(
                color: AppTheme.primaryColor.withValues(alpha: 0.2),
                borderRadius: BorderRadius.circular(10),
              ),
              labelColor: AppTheme.primaryColor,
              unselectedLabelColor: Colors.grey[500],
              labelStyle: const TextStyle(fontSize: 13, fontWeight: FontWeight.w600),
              dividerHeight: 0,
              tabs: const [
                Tab(text: '工作流模板'),
                Tab(text: '运行记录'),
              ],
            ),
          ),
          const SizedBox(height: 12),

          Expanded(
            child: TabBarView(
              controller: _tabController,
              children: [
                _buildTemplatesTab(),
                _buildRunsTab(),
              ],
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildTemplatesTab() {
    if (_loadingTemplates) return const Center(child: CircularProgressIndicator());
    if (_templates.isEmpty) {
      return Center(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(Icons.account_tree_outlined, color: Colors.grey[700], size: 48),
            const SizedBox(height: 12),
            Text('暂无工作流模板', style: TextStyle(color: Colors.grey[600], fontSize: 14)),
            const SizedBox(height: 6),
            Text('工作流模板会在 Agent 市场中发布', style: TextStyle(color: Colors.grey[700], fontSize: 12)),
          ],
        ),
      );
    }

    return RefreshIndicator(
      onRefresh: _loadTemplates,
      child: ListView.builder(
        padding: const EdgeInsets.symmetric(horizontal: 16),
        itemCount: _templates.length,
        itemBuilder: (ctx, i) => _buildTemplateCard(_templates[i]),
      ),
    );
  }

  Widget _buildTemplateCard(Map<String, dynamic> tpl) {
    final name = tpl['name']?.toString() ?? '工作流';
    final desc = tpl['description']?.toString() ?? '';
    final nodeCount = tpl['node_count'] ?? tpl['steps'] ?? 0;

    return Container(
      margin: const EdgeInsets.only(bottom: 12),
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: const Color(0xFF1E293B),
        borderRadius: BorderRadius.circular(14),
      ),
      child: Row(
        children: [
          Container(
            width: 44, height: 44,
            decoration: BoxDecoration(
              color: AppTheme.secondaryColor.withValues(alpha: 0.15),
              borderRadius: BorderRadius.circular(12),
            ),
            child: const Icon(Icons.account_tree_rounded, color: AppTheme.secondaryColor, size: 22),
          ),
          const SizedBox(width: 14),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(name, style: const TextStyle(color: Colors.white, fontSize: 14, fontWeight: FontWeight.w600)),
                if (desc.isNotEmpty) ...[
                  const SizedBox(height: 3),
                  Text(desc, maxLines: 2, overflow: TextOverflow.ellipsis,
                    style: TextStyle(color: Colors.grey[500], fontSize: 12)),
                ],
                const SizedBox(height: 6),
                Text('$nodeCount 个节点', style: TextStyle(color: Colors.grey[600], fontSize: 11)),
              ],
            ),
          ),
          const SizedBox(width: 8),
          ElevatedButton(
            onPressed: () => _triggerWorkflow(tpl),
            style: ElevatedButton.styleFrom(
              backgroundColor: AppTheme.primaryColor,
              padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 8),
              shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(8)),
              minimumSize: Size.zero,
            ),
            child: const Text('运行', style: TextStyle(fontSize: 12)),
          ),
        ],
      ),
    );
  }

  Future<void> _triggerWorkflow(Map<String, dynamic> tpl) async {
    final id = tpl['id']?.toString();
    if (id == null) return;

    try {
      await ApiService().dio.post('/user/workflow/run', data: {'template_id': id});
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('工作流已触发')),
        );
        _tabController.animateTo(1);
        _loadRuns();
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('触发失败: $e')),
        );
      }
    }
  }

  Widget _buildRunsTab() {
    if (_loadingRuns) return const Center(child: CircularProgressIndicator());
    if (_runs.isEmpty) {
      return Center(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(Icons.play_circle_outline, color: Colors.grey[700], size: 48),
            const SizedBox(height: 12),
            Text('暂无运行记录', style: TextStyle(color: Colors.grey[600], fontSize: 14)),
            const SizedBox(height: 6),
            Text('运行工作流后，记录会显示在这里', style: TextStyle(color: Colors.grey[700], fontSize: 12)),
          ],
        ),
      );
    }

    return RefreshIndicator(
      onRefresh: _loadRuns,
      child: ListView.builder(
        padding: const EdgeInsets.symmetric(horizontal: 16),
        itemCount: _runs.length,
        itemBuilder: (ctx, i) => _buildRunCard(_runs[i]),
      ),
    );
  }

  Widget _buildRunCard(Map<String, dynamic> run) {
    final name = run['name']?.toString() ?? run['template_name']?.toString() ?? '工作流';
    final status = run['status']?.toString() ?? 'unknown';
    final createdAt = run['created_at']?.toString() ?? '';
    final duration = run['duration_ms'];

    Color statusColor;
    IconData statusIcon;
    String statusLabel;
    switch (status) {
      case 'running':
        statusColor = AppTheme.warningColor;
        statusIcon = Icons.autorenew;
        statusLabel = '运行中';
        break;
      case 'completed':
      case 'success':
        statusColor = AppTheme.successColor;
        statusIcon = Icons.check_circle_outline;
        statusLabel = '已完成';
        break;
      case 'failed':
      case 'error':
        statusColor = AppTheme.errorColor;
        statusIcon = Icons.error_outline;
        statusLabel = '失败';
        break;
      default:
        statusColor = Colors.grey;
        statusIcon = Icons.pending;
        statusLabel = status;
    }

    return Container(
      margin: const EdgeInsets.only(bottom: 10),
      padding: const EdgeInsets.all(14),
      decoration: BoxDecoration(
        color: const Color(0xFF1E293B),
        borderRadius: BorderRadius.circular(14),
      ),
      child: Row(
        children: [
          Container(
            width: 38, height: 38,
            decoration: BoxDecoration(
              color: statusColor.withValues(alpha: 0.12),
              borderRadius: BorderRadius.circular(10),
            ),
            child: Icon(statusIcon, color: statusColor, size: 20),
          ),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(name, style: const TextStyle(color: Colors.white, fontSize: 13, fontWeight: FontWeight.w600)),
                const SizedBox(height: 3),
                Row(
                  children: [
                    Container(
                      padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
                      decoration: BoxDecoration(
                        color: statusColor.withValues(alpha: 0.1),
                        borderRadius: BorderRadius.circular(4),
                      ),
                      child: Text(statusLabel, style: TextStyle(color: statusColor, fontSize: 10)),
                    ),
                    if (duration != null) ...[
                      const SizedBox(width: 8),
                      Text('${(duration / 1000).toStringAsFixed(1)}s', style: TextStyle(color: Colors.grey[600], fontSize: 10)),
                    ],
                    const Spacer(),
                    if (createdAt.isNotEmpty)
                      Text(_formatTime(createdAt), style: TextStyle(color: Colors.grey[600], fontSize: 10)),
                  ],
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }

  String _formatTime(String iso) {
    try {
      final dt = DateTime.parse(iso).toLocal();
      return '${dt.month}/${dt.day} ${dt.hour.toString().padLeft(2, '0')}:${dt.minute.toString().padLeft(2, '0')}';
    } catch (_) {
      return iso;
    }
  }
}
