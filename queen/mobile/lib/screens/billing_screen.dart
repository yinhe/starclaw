import 'package:flutter/material.dart';
import '../services/api_service.dart';
import '../theme.dart';

class BillingScreen extends StatefulWidget {
  const BillingScreen({super.key});

  @override
  State<BillingScreen> createState() => _BillingScreenState();
}

class _BillingScreenState extends State<BillingScreen> {
  Map<String, dynamic>? _balance;
  List<dynamic> _transactions = [];
  List<dynamic> _packages = [];
  bool _loading = true;

  static const _txTypeLabels = {
    'recharge': '充值',
    'consume': '消费',
    'refund': '退款',
    'admin_adjust': '人工调账',
    'bounty_freeze': '赏金冻结',
    'bounty_unfreeze': '赏金解冻',
    'bounty_settle': '赏金结算',
  };

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    try {
      final results = await Future.wait([
        ApiService().getBalance(),
        ApiService().getTransactions(),
        ApiService().getPackages(),
      ]);
      if (mounted) {
        setState(() {
          _balance = results[0].data is Map ? results[0].data as Map<String, dynamic> : null;
          _transactions = (results[1].data is Map ? results[1].data['transactions'] : null) ?? [];
          _packages = (results[2].data is Map ? results[2].data['packages'] : null) ?? [];
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
      child: _loading
          ? const Center(child: CircularProgressIndicator())
          : RefreshIndicator(
              onRefresh: _load,
              child: ListView(
                padding: const EdgeInsets.all(20),
                children: [
                  // Header
                  Row(
                    children: [
                      Container(
                        width: 36, height: 36,
                        decoration: BoxDecoration(
                          gradient: const LinearGradient(colors: [AppTheme.warningColor, Color(0xFFF97316)]),
                          borderRadius: BorderRadius.circular(10),
                        ),
                        child: const Icon(Icons.account_balance_wallet_rounded, color: Colors.white, size: 18),
                      ),
                      const SizedBox(width: 10),
                      const Text('我的钱包', style: TextStyle(fontSize: 20, fontWeight: FontWeight.bold, color: Colors.white)),
                    ],
                  ),
                  const SizedBox(height: 20),

                  // Balance card
                  Container(
                    padding: const EdgeInsets.all(24),
                    decoration: BoxDecoration(
                      gradient: const LinearGradient(
                        colors: [AppTheme.primaryColor, AppTheme.secondaryColor],
                        begin: Alignment.topLeft,
                        end: Alignment.bottomRight,
                      ),
                      borderRadius: BorderRadius.circular(20),
                    ),
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        const Text('可用余额', style: TextStyle(color: Colors.white70, fontSize: 13)),
                        const SizedBox(height: 8),
                        Text(
                          '¥${((_balance?['balance'] ?? 0) / 100).toStringAsFixed(2)}',
                          style: const TextStyle(color: Colors.white, fontSize: 32, fontWeight: FontWeight.bold),
                        ),
                        const SizedBox(height: 8),
                        Text(
                          '冻结: ¥${((_balance?['frozen'] ?? 0) / 100).toStringAsFixed(2)}',
                          style: const TextStyle(color: Colors.white54, fontSize: 12),
                        ),
                      ],
                    ),
                  ),
                  const SizedBox(height: 24),

                  // Recharge packages
                  const Text('充值套餐', style: TextStyle(color: Colors.white, fontSize: 16, fontWeight: FontWeight.w600)),
                  const SizedBox(height: 12),
                  if (_packages.isNotEmpty)
                    Wrap(
                      spacing: 10,
                      runSpacing: 10,
                      children: _packages.map((pkg) {
                        final amount = (pkg['amount'] ?? 0) / 100;
                        final bonus = pkg['bonus_percent'] ?? 0;
                        return GestureDetector(
                          onTap: () => _showRechargeSheet(pkg),
                          child: Container(
                            width: (MediaQuery.of(context).size.width - 50) / 3,
                            padding: const EdgeInsets.symmetric(vertical: 16),
                            decoration: BoxDecoration(
                              color: const Color(0xFF1E293B),
                              borderRadius: BorderRadius.circular(14),
                              border: Border.all(color: const Color(0xFF334155)),
                            ),
                            child: Column(
                              children: [
                                Text('¥${amount.toStringAsFixed(0)}', style: const TextStyle(color: Colors.white, fontSize: 18, fontWeight: FontWeight.bold)),
                                if (bonus > 0)
                                  Padding(
                                    padding: const EdgeInsets.only(top: 4),
                                    child: Container(
                                      padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
                                      decoration: BoxDecoration(
                                        color: AppTheme.warningColor.withOpacity(0.15),
                                        borderRadius: BorderRadius.circular(4),
                                      ),
                                      child: Text('送$bonus%', style: const TextStyle(color: AppTheme.warningColor, fontSize: 10, fontWeight: FontWeight.w600)),
                                    ),
                                  ),
                              ],
                            ),
                          ),
                        );
                      }).toList(),
                    ),
                  const SizedBox(height: 28),

                  // Transactions
                  const Text('账单明细', style: TextStyle(color: Colors.white, fontSize: 16, fontWeight: FontWeight.w600)),
                  const SizedBox(height: 12),
                  if (_transactions.isEmpty)
                    Container(
                      padding: const EdgeInsets.all(32),
                      decoration: BoxDecoration(
                        color: const Color(0xFF1E293B),
                        borderRadius: BorderRadius.circular(14),
                      ),
                      child: Center(child: Text('暂无交易记录', style: TextStyle(color: Colors.grey[500], fontSize: 13))),
                    )
                  else
                    ..._transactions.take(20).map((tx) => _buildTxRow(tx)),
                ],
              ),
            ),
    );
  }

  Widget _buildTxRow(Map<String, dynamic> tx) {
    final type = tx['type'] ?? '';
    final label = _txTypeLabels[type] ?? type;
    final amount = (tx['amount'] ?? 0) / 100;
    final isPositive = type == 'recharge' || type == 'refund' || type == 'bounty_unfreeze' || type == 'admin_adjust';

    return Container(
      margin: const EdgeInsets.only(bottom: 8),
      padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 12),
      decoration: BoxDecoration(
        color: const Color(0xFF1E293B),
        borderRadius: BorderRadius.circular(12),
      ),
      child: Row(
        children: [
          Container(
            width: 36, height: 36,
            decoration: BoxDecoration(
              color: (isPositive ? AppTheme.successColor : AppTheme.errorColor).withOpacity(0.12),
              borderRadius: BorderRadius.circular(10),
            ),
            child: Icon(
              isPositive ? Icons.arrow_downward_rounded : Icons.arrow_upward_rounded,
              color: isPositive ? AppTheme.successColor : AppTheme.errorColor,
              size: 18,
            ),
          ),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(label, style: const TextStyle(color: Colors.white, fontSize: 14, fontWeight: FontWeight.w500)),
                if (tx['description'] != null)
                  Text(tx['description'], style: TextStyle(color: Colors.grey[600], fontSize: 11), maxLines: 1),
              ],
            ),
          ),
          Text(
            '${isPositive ? '+' : '-'}¥${amount.abs().toStringAsFixed(2)}',
            style: TextStyle(
              color: isPositive ? AppTheme.successColor : AppTheme.errorColor,
              fontSize: 15,
              fontWeight: FontWeight.w600,
            ),
          ),
        ],
      ),
    );
  }

  void _showRechargeSheet(Map<String, dynamic> pkg) {
    showModalBottomSheet(
      context: context,
      backgroundColor: const Color(0xFF1E293B),
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(20)),
      ),
      builder: (ctx) {
        final amount = (pkg['amount'] ?? 0) / 100;
        return Padding(
          padding: const EdgeInsets.all(24),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Text('充值 ¥${amount.toStringAsFixed(0)}', style: const TextStyle(color: Colors.white, fontSize: 18, fontWeight: FontWeight.bold)),
              const SizedBox(height: 20),
              SizedBox(
                width: double.infinity,
                child: ElevatedButton.icon(
                  onPressed: () {
                    Navigator.pop(ctx);
                    ScaffoldMessenger.of(context).showSnackBar(
                      const SnackBar(content: Text('支付功能即将上线')),
                    );
                  },
                  icon: const Icon(Icons.payment_rounded),
                  label: const Text('支付宝支付'),
                ),
              ),
              const SizedBox(height: 10),
              SizedBox(
                width: double.infinity,
                child: OutlinedButton.icon(
                  onPressed: () {
                    Navigator.pop(ctx);
                    ScaffoldMessenger.of(context).showSnackBar(
                      const SnackBar(content: Text('支付功能即将上线')),
                    );
                  },
                  icon: const Icon(Icons.chat_rounded, color: AppTheme.successColor),
                  label: const Text('微信支付', style: TextStyle(color: AppTheme.successColor)),
                  style: OutlinedButton.styleFrom(
                    side: const BorderSide(color: AppTheme.successColor),
                    padding: const EdgeInsets.symmetric(vertical: 14),
                    shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
                  ),
                ),
              ),
              const SizedBox(height: 16),
            ],
          ),
        );
      },
    );
  }
}
