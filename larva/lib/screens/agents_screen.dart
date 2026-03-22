import 'package:flutter/material.dart';
import '../services/api_service.dart';
import '../theme.dart';

class AgentsScreen extends StatefulWidget {
  const AgentsScreen({super.key});

  @override
  State<AgentsScreen> createState() => _AgentsScreenState();
}

class _AgentsScreenState extends State<AgentsScreen> {
  List<Map<String, dynamic>> _agents = [];
  List<Map<String, dynamic>> _categories = [];
  String _selectedCategory = '';
  String _search = '';
  bool _loading = true;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    setState(() => _loading = true);
    try {
      final res = await ApiService().dio.get(
        '/v1/marketplace/items',
        queryParameters: {
          'type': 'agent',
          if (_search.isNotEmpty) 'q': _search,
        },
      );

      if (mounted) {
        final items = _parseList(res.data, 'items');
        // Extract unique categories from item tags
        final catSet = <String>{};
        for (final item in items) {
          final tags = (item['tags'] ?? '').toString().split(',');
          for (final t in tags) {
            final trimmed = t.trim();
            if (trimmed.isNotEmpty) catSet.add(trimmed);
          }
        }
        final cats = catSet
            .map((c) => <String, dynamic>{'id': c, 'name': c})
            .toList();

        // Client-side category filter
        final filtered = _selectedCategory.isEmpty
            ? items
            : items
                  .where(
                    (a) => (a['tags'] ?? '').toString().contains(
                      _selectedCategory,
                    ),
                  )
                  .toList();

        setState(() {
          _agents = filtered;
          _categories = cats;
          _loading = false;
        });
      }
    } catch (_) {
      if (mounted) setState(() => _loading = false);
    }
  }

  List<Map<String, dynamic>> _parseList(dynamic data, String key) {
    if (data is Map && data[key] is List) {
      return (data[key] as List)
          .map((e) => e is Map<String, dynamic> ? e : <String, dynamic>{})
          .toList();
    }
    if (data is List) {
      return data
          .map((e) => e is Map<String, dynamic> ? e : <String, dynamic>{})
          .toList();
    }
    return [];
  }

  @override
  Widget build(BuildContext context) {
    return SafeArea(
      child: Column(
        children: [
          // Header
          Container(
            padding: const EdgeInsets.fromLTRB(20, 16, 20, 12),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                const Text(
                  'Agent 市场',
                  style: TextStyle(
                    color: Colors.white,
                    fontSize: 20,
                    fontWeight: FontWeight.bold,
                  ),
                ),
                const SizedBox(height: 4),
                Text(
                  '发现和使用 AI 智能体',
                  style: TextStyle(color: Colors.grey[500], fontSize: 13),
                ),
                const SizedBox(height: 14),
                // Search
                TextField(
                  style: const TextStyle(color: Colors.white, fontSize: 14),
                  decoration: InputDecoration(
                    hintText: '搜索 Agent...',
                    prefixIcon: Icon(
                      Icons.search,
                      color: Colors.grey[500],
                      size: 20,
                    ),
                    filled: true,
                    fillColor: const Color(0xFF1E293B),
                    border: OutlineInputBorder(
                      borderRadius: BorderRadius.circular(12),
                      borderSide: BorderSide.none,
                    ),
                    contentPadding: const EdgeInsets.symmetric(vertical: 10),
                  ),
                  onSubmitted: (v) {
                    setState(() => _search = v.trim());
                    _load();
                  },
                ),
              ],
            ),
          ),

          // Category chips
          if (_categories.isNotEmpty)
            SizedBox(
              height: 36,
              child: ListView(
                scrollDirection: Axis.horizontal,
                padding: const EdgeInsets.symmetric(horizontal: 20),
                children: [
                  _buildChip('全部', '', _selectedCategory.isEmpty),
                  ..._categories.map((c) {
                    final id = c['id']?.toString() ?? '';
                    final name = c['name']?.toString() ?? id;
                    return _buildChip(name, id, _selectedCategory == id);
                  }),
                ],
              ),
            ),

          const SizedBox(height: 12),

          // Grid
          Expanded(
            child: _loading
                ? const Center(child: CircularProgressIndicator())
                : _agents.isEmpty
                ? Center(
                    child: Column(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Icon(
                          Icons.smart_toy_outlined,
                          color: Colors.grey[700],
                          size: 48,
                        ),
                        const SizedBox(height: 12),
                        Text(
                          '暂无 Agent',
                          style: TextStyle(
                            color: Colors.grey[600],
                            fontSize: 14,
                          ),
                        ),
                      ],
                    ),
                  )
                : RefreshIndicator(
                    onRefresh: _load,
                    child: GridView.builder(
                      padding: const EdgeInsets.symmetric(horizontal: 16),
                      gridDelegate:
                          const SliverGridDelegateWithFixedCrossAxisCount(
                            crossAxisCount: 2,
                            mainAxisSpacing: 12,
                            crossAxisSpacing: 12,
                            childAspectRatio: 0.78,
                          ),
                      itemCount: _agents.length,
                      itemBuilder: (ctx, i) => _buildAgentCard(_agents[i]),
                    ),
                  ),
          ),
        ],
      ),
    );
  }

  Widget _buildChip(String label, String value, bool selected) {
    return GestureDetector(
      onTap: () {
        setState(() => _selectedCategory = value);
        _load();
      },
      child: Container(
        margin: const EdgeInsets.only(right: 8),
        padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 6),
        decoration: BoxDecoration(
          color: selected
              ? AppTheme.primaryColor.withValues(alpha: 0.2)
              : const Color(0xFF1E293B),
          borderRadius: BorderRadius.circular(20),
          border: Border.all(
            color: selected ? AppTheme.primaryColor : const Color(0xFF334155),
          ),
        ),
        child: Text(
          label,
          style: TextStyle(
            color: selected ? AppTheme.primaryColor : Colors.grey[400],
            fontSize: 12,
          ),
        ),
      ),
    );
  }

  Widget _buildAgentCard(Map<String, dynamic> agent) {
    final name = agent['name']?.toString() ?? 'Agent';
    final desc = agent['description']?.toString() ?? '';
    final icon = agent['icon']?.toString() ?? '';
    final category = agent['category']?.toString() ?? '';
    final downloads = agent['download_count'] ?? agent['use_count'] ?? 0;

    return Container(
      decoration: BoxDecoration(
        color: const Color(0xFF1E293B),
        borderRadius: BorderRadius.circular(16),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // Icon / Image area
          Container(
            height: 80,
            width: double.infinity,
            decoration: BoxDecoration(
              gradient: LinearGradient(
                colors: [
                  AppTheme.primaryColor.withValues(alpha: 0.15),
                  AppTheme.secondaryColor.withValues(alpha: 0.1),
                ],
              ),
              borderRadius: const BorderRadius.vertical(
                top: Radius.circular(16),
              ),
            ),
            child: Center(
              child: icon.startsWith('http')
                  ? ClipRRect(
                      borderRadius: BorderRadius.circular(12),
                      child: Image.network(
                        icon,
                        width: 44,
                        height: 44,
                        fit: BoxFit.cover,
                        errorBuilder: (_, __, ___) => _defaultIcon(name),
                      ),
                    )
                  : _defaultIcon(name),
            ),
          ),
          Expanded(
            child: Padding(
              padding: const EdgeInsets.all(12),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    name,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: const TextStyle(
                      color: Colors.white,
                      fontSize: 14,
                      fontWeight: FontWeight.w600,
                    ),
                  ),
                  const SizedBox(height: 4),
                  Expanded(
                    child: Text(
                      desc,
                      maxLines: 2,
                      overflow: TextOverflow.ellipsis,
                      style: TextStyle(
                        color: Colors.grey[500],
                        fontSize: 11,
                        height: 1.4,
                      ),
                    ),
                  ),
                  const SizedBox(height: 6),
                  Row(
                    children: [
                      if (category.isNotEmpty) ...[
                        Container(
                          padding: const EdgeInsets.symmetric(
                            horizontal: 6,
                            vertical: 2,
                          ),
                          decoration: BoxDecoration(
                            color: const Color(0xFF334155),
                            borderRadius: BorderRadius.circular(4),
                          ),
                          child: Text(
                            category,
                            style: TextStyle(
                              color: Colors.grey[400],
                              fontSize: 9,
                            ),
                          ),
                        ),
                        const Spacer(),
                      ],
                      Icon(
                        Icons.download_rounded,
                        color: Colors.grey[600],
                        size: 12,
                      ),
                      const SizedBox(width: 3),
                      Text(
                        '$downloads',
                        style: TextStyle(color: Colors.grey[600], fontSize: 10),
                      ),
                    ],
                  ),
                ],
              ),
            ),
          ),
        ],
      ),
    );
  }

  Widget _defaultIcon(String name) {
    return Container(
      width: 44,
      height: 44,
      decoration: BoxDecoration(
        gradient: const LinearGradient(
          colors: [AppTheme.primaryColor, AppTheme.secondaryColor],
        ),
        borderRadius: BorderRadius.circular(12),
      ),
      child: Center(
        child: Text(
          name.isNotEmpty ? name[0].toUpperCase() : 'A',
          style: const TextStyle(
            color: Colors.white,
            fontSize: 20,
            fontWeight: FontWeight.bold,
          ),
        ),
      ),
    );
  }
}
