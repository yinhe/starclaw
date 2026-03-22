import 'package:flutter/material.dart';
import '../services/api_service.dart';
import '../services/auth_service.dart';
import '../theme.dart';

class BountyDetailScreen extends StatefulWidget {
  final String bountyId;
  const BountyDetailScreen({super.key, required this.bountyId});

  @override
  State<BountyDetailScreen> createState() => _BountyDetailScreenState();
}

class _BountyDetailScreenState extends State<BountyDetailScreen> {
  Map<String, dynamic>? _bounty;
  bool _loading = true;
  bool _actionLoading = false;
  String? _actionMsg;

  static const _statusStyles = {
    'open': ('开放', AppTheme.successColor),
    'claimed': ('已领取', Color(0xFF3B82F6)),
    'delivered': ('已交付', AppTheme.warningColor),
    'completed': ('已完成', Color(0xFF22C55E)),
    'disputed': ('争议中', AppTheme.errorColor),
    'cancelled': ('已取消', Color(0xFF6B7280)),
    'expired': ('已过期', Color(0xFF6B7280)),
  };

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    try {
      final res = await ApiService().getBounty(widget.bountyId);
      if (mounted) setState(() { _bounty = res.data['bounty']; _loading = false; });
    } catch (_) {
      if (mounted) setState(() => _loading = false);
    }
  }

  Future<void> _doAction(String action, {String? notes, String? reason}) async {
    final auth = AuthService();
    setState(() { _actionLoading = true; _actionMsg = null; });
    try {
      switch (action) {
        case 'claim':
          await ApiService().claimBounty(
            widget.bountyId,
            auth.userId ?? '',
            auth.username ?? 'User',
          );
          break;
        case 'deliver':
          await ApiService().deliverBounty(widget.bountyId, notes ?? '');
          break;
        case 'cancel':
          await ApiService().cancelBounty(widget.bountyId);
          break;
        case 'dispute':
          await ApiService().disputeBounty(widget.bountyId, reason ?? '');
          break;
      }
      setState(() => _actionMsg = '操作成功');
      await _load();
    } catch (e) {
      setState(() => _actionMsg = '操作失败: $e');
    }
    setState(() => _actionLoading = false);
  }

  String _formatReward(dynamic amount, String currency) {
    final a = (amount is int ? amount : (amount as num).toInt());
    if (currency == 'CNY' || currency == 'cny') return '¥${(a / 100).toStringAsFixed(2)}';
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
    return Scaffold(
      appBar: AppBar(
        title: const Text('赏金详情'),
        actions: [
          if (_bounty != null)
            IconButton(
              icon: const Icon(Icons.flag_outlined, size: 20),
              onPressed: _showReportSheet,
              tooltip: '举报',
            ),
        ],
      ),
      body: _loading
          ? const Center(child: CircularProgressIndicator())
          : _bounty == null
              ? const Center(child: Text('加载失败', style: TextStyle(color: Colors.grey)))
              : _buildContent(),
    );
  }

  Widget _buildContent() {
    final b = _bounty!;
    final status = b['status'] ?? 'open';
    final (statusLabel, statusColor) = _statusStyles[status] ?? ('未知', Colors.grey);
    final auth = AuthService();
    final isCreator = auth.userId == b['creator_id'];
    final isClaimer = auth.userId == b['claimed_by'];

    return RefreshIndicator(
      onRefresh: _load,
      child: ListView(
        padding: const EdgeInsets.all(20),
        children: [
          // Status + reward
          Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Container(
                      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
                      decoration: BoxDecoration(
                        color: statusColor.withValues(alpha: 0.15),
                        borderRadius: BorderRadius.circular(8),
                      ),
                      child: Text(statusLabel, style: TextStyle(color: statusColor, fontSize: 12, fontWeight: FontWeight.w600)),
                    ),
                    const SizedBox(height: 12),
                    Text(b['title'] ?? '', style: const TextStyle(color: Colors.white, fontSize: 20, fontWeight: FontWeight.bold)),
                  ],
                ),
              ),
              const SizedBox(width: 16),
              Column(
                crossAxisAlignment: CrossAxisAlignment.end,
                children: [
                  Text(
                    _formatReward(b['reward_amount'] ?? 0, b['reward_currency'] ?? 'CNY'),
                    style: const TextStyle(color: AppTheme.successColor, fontSize: 24, fontWeight: FontWeight.bold),
                  ),
                  const SizedBox(height: 2),
                  Text('赏金', style: TextStyle(color: Colors.grey[500], fontSize: 11)),
                ],
              ),
            ],
          ),
          const SizedBox(height: 16),

          // Meta
          Wrap(
            spacing: 16,
            runSpacing: 8,
            children: [
              _metaChip(Icons.person_outline, '发布者: ${b['creator_name'] ?? ''}'),
              _metaChip(Icons.access_time, _timeAgo(b['created_at'])),
              if (b['deadline'] != null)
                _metaChip(Icons.event, '截止: ${DateTime.tryParse(b['deadline'] ?? '')?.toLocal().toString().substring(0, 10) ?? ''}'),
            ],
          ),
          const SizedBox(height: 20),

          // Description
          Container(
            padding: const EdgeInsets.all(16),
            decoration: BoxDecoration(
              color: const Color(0xFF1E293B),
              borderRadius: BorderRadius.circular(14),
            ),
            child: Text(b['description'] ?? '', style: TextStyle(color: Colors.grey[300], fontSize: 14, height: 1.6)),
          ),
          const SizedBox(height: 16),

          // Claimed by
          if (b['claimed_by_name'] != null)
            _infoBox(Icons.assignment_ind_outlined, '领取者: ${b['claimed_by_name']}', const Color(0xFF3B82F6)),

          // Delivery notes
          if (b['delivery_notes'] != null && (b['delivery_notes'] as String).isNotEmpty)
            _infoBox(Icons.description_outlined, '交付说明: ${b['delivery_notes']}', AppTheme.warningColor),

          if (_actionMsg != null)
            Padding(
              padding: const EdgeInsets.only(top: 12),
              child: Text(_actionMsg!, style: TextStyle(color: AppTheme.primaryColor, fontSize: 13)),
            ),

          const SizedBox(height: 24),

          // Action buttons
          if (auth.isLoggedIn) ..._buildActions(status, isCreator, isClaimer),
        ],
      ),
    );
  }

  Widget _metaChip(IconData icon, String text) {
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        Icon(icon, size: 14, color: Colors.grey[500]),
        const SizedBox(width: 4),
        Text(text, style: TextStyle(color: Colors.grey[400], fontSize: 12)),
      ],
    );
  }

  Widget _infoBox(IconData icon, String text, Color color) {
    return Container(
      margin: const EdgeInsets.only(bottom: 10),
      padding: const EdgeInsets.all(14),
      decoration: BoxDecoration(
        color: color.withValues(alpha: 0.08),
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: color.withValues(alpha: 0.2)),
      ),
      child: Row(
        children: [
          Icon(icon, size: 18, color: color),
          const SizedBox(width: 10),
          Expanded(child: Text(text, style: TextStyle(color: color, fontSize: 13))),
        ],
      ),
    );
  }

  List<Widget> _buildActions(String status, bool isCreator, bool isClaimer) {
    final List<Widget> widgets = [];

    if (status == 'open' && !isCreator) {
      widgets.add(SizedBox(
        width: double.infinity,
        child: ElevatedButton.icon(
          onPressed: _actionLoading ? null : () => _doAction('claim'),
          icon: const Icon(Icons.back_hand_rounded, size: 18),
          label: Text(_actionLoading ? '处理中...' : '领取任务'),
        ),
      ));
    }

    if (status == 'claimed' && isClaimer) {
      widgets.add(SizedBox(
        width: double.infinity,
        child: ElevatedButton.icon(
          onPressed: _actionLoading ? null : _showDeliverSheet,
          icon: const Icon(Icons.send_rounded, size: 18),
          label: const Text('提交交付'),
          style: ElevatedButton.styleFrom(backgroundColor: AppTheme.successColor),
        ),
      ));
    }

    if (status == 'delivered' && isCreator) {
      widgets.add(Row(
        children: [
          Expanded(
            child: ElevatedButton.icon(
              onPressed: _actionLoading ? null : () async {
                final res = await ApiService().dio.post('/bounty/${widget.bountyId}/accept');
                if (res.statusCode == 200) {
                  setState(() => _actionMsg = '已验收通过');
                  _load();
                }
              },
              icon: const Icon(Icons.check_circle_outline, size: 18),
              label: const Text('验收通过'),
              style: ElevatedButton.styleFrom(backgroundColor: AppTheme.successColor),
            ),
          ),
          const SizedBox(width: 10),
          Expanded(
            child: OutlinedButton.icon(
              onPressed: _actionLoading ? null : _showDisputeSheet,
              icon: const Icon(Icons.warning_amber_rounded, size: 18, color: AppTheme.errorColor),
              label: const Text('发起争议', style: TextStyle(color: AppTheme.errorColor)),
              style: OutlinedButton.styleFrom(
                side: const BorderSide(color: AppTheme.errorColor),
                shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
                padding: const EdgeInsets.symmetric(vertical: 14),
              ),
            ),
          ),
        ],
      ));
    }

    if (['open', 'claimed'].contains(status) && isCreator) {
      widgets.add(Padding(
        padding: const EdgeInsets.only(top: 10),
        child: TextButton(
          onPressed: _actionLoading ? null : () => _doAction('cancel'),
          child: const Text('取消赏金', style: TextStyle(color: Colors.grey)),
        ),
      ));
    }

    return widgets;
  }

  void _showDeliverSheet() {
    final ctrl = TextEditingController();
    showModalBottomSheet(
      context: context,
      backgroundColor: const Color(0xFF1E293B),
      shape: const RoundedRectangleBorder(borderRadius: BorderRadius.vertical(top: Radius.circular(20))),
      isScrollControlled: true,
      builder: (ctx) => Padding(
        padding: EdgeInsets.fromLTRB(24, 24, 24, MediaQuery.of(ctx).viewInsets.bottom + 24),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            const Text('提交交付', style: TextStyle(color: Colors.white, fontSize: 18, fontWeight: FontWeight.bold)),
            const SizedBox(height: 16),
            TextField(
              controller: ctrl,
              maxLines: 4,
              decoration: const InputDecoration(hintText: '描述你的交付内容和成果...'),
            ),
            const SizedBox(height: 16),
            SizedBox(
              width: double.infinity,
              child: ElevatedButton(
                onPressed: () {
                  Navigator.pop(ctx);
                  _doAction('deliver', notes: ctrl.text);
                },
                style: ElevatedButton.styleFrom(backgroundColor: AppTheme.successColor),
                child: const Text('确认提交'),
              ),
            ),
          ],
        ),
      ),
    );
  }

  void _showDisputeSheet() {
    final ctrl = TextEditingController();
    showModalBottomSheet(
      context: context,
      backgroundColor: const Color(0xFF1E293B),
      shape: const RoundedRectangleBorder(borderRadius: BorderRadius.vertical(top: Radius.circular(20))),
      isScrollControlled: true,
      builder: (ctx) => Padding(
        padding: EdgeInsets.fromLTRB(24, 24, 24, MediaQuery.of(ctx).viewInsets.bottom + 24),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            const Text('发起争议', style: TextStyle(color: AppTheme.errorColor, fontSize: 18, fontWeight: FontWeight.bold)),
            const SizedBox(height: 16),
            TextField(
              controller: ctrl,
              maxLines: 4,
              decoration: const InputDecoration(hintText: '请描述争议原因...'),
            ),
            const SizedBox(height: 16),
            SizedBox(
              width: double.infinity,
              child: ElevatedButton(
                onPressed: () {
                  Navigator.pop(ctx);
                  _doAction('dispute', reason: ctrl.text);
                },
                style: ElevatedButton.styleFrom(backgroundColor: AppTheme.errorColor),
                child: const Text('提交争议'),
              ),
            ),
          ],
        ),
      ),
    );
  }

  void _showReportSheet() {
    String? selectedReason;
    final detailCtrl = TextEditingController();
    final reasons = [
      ('spam', '垃圾信息'),
      ('abuse', '辱骂/人身攻击'),
      ('nsfw', '不当内容'),
      ('illegal', '违法违规'),
      ('other', '其他'),
    ];

    showModalBottomSheet(
      context: context,
      backgroundColor: const Color(0xFF1E293B),
      shape: const RoundedRectangleBorder(borderRadius: BorderRadius.vertical(top: Radius.circular(20))),
      isScrollControlled: true,
      builder: (ctx) => StatefulBuilder(
        builder: (ctx, setSheetState) => Padding(
          padding: EdgeInsets.fromLTRB(24, 24, 24, MediaQuery.of(ctx).viewInsets.bottom + 24),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              const Row(
                children: [
                  Icon(Icons.flag_rounded, color: AppTheme.errorColor, size: 20),
                  SizedBox(width: 8),
                  Text('举报内容', style: TextStyle(color: Colors.white, fontSize: 18, fontWeight: FontWeight.bold)),
                ],
              ),
              const SizedBox(height: 16),
              Wrap(
                spacing: 8,
                runSpacing: 8,
                children: reasons.map((r) => GestureDetector(
                  onTap: () => setSheetState(() => selectedReason = r.$1),
                  child: Container(
                    padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 8),
                    decoration: BoxDecoration(
                      color: selectedReason == r.$1 ? AppTheme.errorColor.withValues(alpha: 0.15) : const Color(0xFF334155),
                      borderRadius: BorderRadius.circular(20),
                      border: Border.all(
                        color: selectedReason == r.$1 ? AppTheme.errorColor : Colors.transparent,
                      ),
                    ),
                    child: Text(r.$2, style: TextStyle(
                      color: selectedReason == r.$1 ? AppTheme.errorColor : Colors.grey[400],
                      fontSize: 13,
                    )),
                  ),
                )).toList(),
              ),
              const SizedBox(height: 14),
              TextField(
                controller: detailCtrl,
                maxLines: 3,
                decoration: const InputDecoration(hintText: '补充说明（可选）'),
              ),
              const SizedBox(height: 16),
              SizedBox(
                width: double.infinity,
                child: ElevatedButton(
                  onPressed: selectedReason == null ? null : () async {
                    Navigator.pop(ctx);
                    try {
                      await ApiService().submitReport(
                        targetType: 'bounty',
                        targetId: widget.bountyId,
                        reason: selectedReason!,
                        targetTitle: _bounty?['title'],
                        authorId: _bounty?['creator_id'],
                        detail: detailCtrl.text.isNotEmpty ? detailCtrl.text : null,
                      );
                      if (mounted) {
                        ScaffoldMessenger.of(context).showSnackBar(
                          const SnackBar(content: Text('举报已提交，我们会尽快处理')),
                        );
                      }
                    } catch (_) {
                      if (mounted) {
                        ScaffoldMessenger.of(context).showSnackBar(
                          const SnackBar(content: Text('提交失败，请稍后重试')),
                        );
                      }
                    }
                  },
                  style: ElevatedButton.styleFrom(backgroundColor: AppTheme.errorColor),
                  child: const Text('提交举报'),
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
