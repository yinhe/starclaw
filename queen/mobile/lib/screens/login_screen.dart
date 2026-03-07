import 'package:flutter/material.dart';
import '../services/api_service.dart';
import '../services/auth_service.dart';
import '../theme.dart';

class LoginScreen extends StatefulWidget {
  const LoginScreen({super.key});

  @override
  State<LoginScreen> createState() => _LoginScreenState();
}

class _LoginScreenState extends State<LoginScreen>
    with SingleTickerProviderStateMixin {
  final _emailCtrl = TextEditingController();
  final _phoneCtrl = TextEditingController();
  final _passwordCtrl = TextEditingController();
  final _usernameCtrl = TextEditingController();
  late TabController _tabCtrl;
  bool _loading = false;
  bool _isRegister = false;
  String? _error;

  @override
  void initState() {
    super.initState();
    _tabCtrl = TabController(length: 2, vsync: this);
    _tabCtrl.addListener(() {
      if (!_tabCtrl.indexIsChanging) {
        setState(() {
          _error = null;
        });
      }
    });
  }

  bool get _isPhoneMode => _tabCtrl.index == 1;

  Future<void> _submit() async {
    final password = _passwordCtrl.text.trim();
    if (password.isEmpty) return;

    if (_isPhoneMode) {
      final phone = _phoneCtrl.text.trim();
      if (phone.isEmpty) return;
      await _submitPhone(phone, password);
    } else {
      final email = _emailCtrl.text.trim();
      if (email.isEmpty) return;
      await _submitEmail(email, password);
    }
  }

  Future<void> _submitEmail(String email, String password) async {
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      if (_isRegister) {
        final username = _usernameCtrl.text.trim();
        if (username.isEmpty) {
          setState(() {
            _error = '请输入用户名';
            _loading = false;
          });
          return;
        }
        await ApiService().register(email, username, password);
      }
      final res = await ApiService().login(email, password);
      final data = res.data;
      await AuthService().saveLogin(
        token: data['token'],
        userId: data['user']?['id'] ?? data['user_id'],
        username: data['user']?['username'] ?? data['username'],
        email: data['user']?['email'] ?? email,
      );
    } catch (e) {
      setState(() {
        _error = _isRegister ? '注册失败，邮箱可能已被注册' : '登录失败，请检查邮箱和密码';
      });
    } finally {
      if (mounted)
        setState(() {
          _loading = false;
        });
    }
  }

  Future<void> _submitPhone(String phone, String password) async {
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      if (_isRegister) {
        final username = _usernameCtrl.text.trim();
        await ApiService().phoneRegister(phone, password, username: username);
      }
      final res = await ApiService().phoneLogin(phone, password);
      final data = res.data;
      await AuthService().saveLogin(
        token: data['token'],
        userId: data['user']?['id'] ?? data['user_id'],
        username: data['user']?['username'] ?? data['username'],
        email: data['user']?['email'],
      );
    } catch (e) {
      setState(() {
        _error = _isRegister ? '注册失败，手机号可能已被注册' : '登录失败，请检查手机号和密码';
      });
    } finally {
      if (mounted)
        setState(() {
          _loading = false;
        });
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      body: SafeArea(
        child: Center(
          child: SingleChildScrollView(
            padding: const EdgeInsets.all(32),
            child: Column(
              mainAxisAlignment: MainAxisAlignment.center,
              children: [
                // Logo
                Container(
                  width: 80,
                  height: 80,
                  decoration: BoxDecoration(
                    gradient: const LinearGradient(
                      colors: [AppTheme.primaryColor, AppTheme.secondaryColor],
                    ),
                    borderRadius: BorderRadius.circular(20),
                  ),
                  child: const Icon(
                    Icons.auto_awesome,
                    color: Colors.white,
                    size: 40,
                  ),
                ),
                const SizedBox(height: 24),
                Text(
                  'StarClaw',
                  style: Theme.of(context).textTheme.headlineMedium?.copyWith(
                    fontWeight: FontWeight.bold,
                    color: Colors.white,
                  ),
                ),
                const SizedBox(height: 8),
                Text(
                  'AI 创作平台',
                  style: TextStyle(color: Colors.grey[400], fontSize: 14),
                ),
                const SizedBox(height: 32),

                // Email / Phone tab
                Container(
                  decoration: BoxDecoration(
                    color: const Color(0xFF1E293B),
                    borderRadius: BorderRadius.circular(12),
                  ),
                  child: TabBar(
                    controller: _tabCtrl,
                    indicatorSize: TabBarIndicatorSize.tab,
                    indicator: BoxDecoration(
                      color: AppTheme.primaryColor,
                      borderRadius: BorderRadius.circular(12),
                    ),
                    labelColor: Colors.white,
                    unselectedLabelColor: Colors.grey[400],
                    dividerHeight: 0,
                    tabs: const [
                      Tab(text: '邮箱'),
                      Tab(text: '手机号'),
                    ],
                  ),
                ),
                const SizedBox(height: 24),

                if (_isRegister) ...[
                  TextField(
                    controller: _usernameCtrl,
                    decoration: const InputDecoration(
                      hintText: '用户名（手机注册可选）',
                      prefixIcon: Icon(Icons.person_outline),
                    ),
                  ),
                  const SizedBox(height: 16),
                ],

                // Email or Phone input
                AnimatedBuilder(
                  animation: _tabCtrl,
                  builder: (context, _) => _isPhoneMode
                      ? TextField(
                          controller: _phoneCtrl,
                          keyboardType: TextInputType.phone,
                          decoration: const InputDecoration(
                            hintText: '手机号码',
                            prefixIcon: Icon(Icons.phone_android),
                          ),
                        )
                      : TextField(
                          controller: _emailCtrl,
                          keyboardType: TextInputType.emailAddress,
                          decoration: const InputDecoration(
                            hintText: '邮箱',
                            prefixIcon: Icon(Icons.email_outlined),
                          ),
                        ),
                ),
                const SizedBox(height: 16),
                TextField(
                  controller: _passwordCtrl,
                  obscureText: true,
                  decoration: const InputDecoration(
                    hintText: '密码',
                    prefixIcon: Icon(Icons.lock_outline),
                  ),
                  onSubmitted: (_) => _submit(),
                ),
                if (_error != null) ...[
                  const SizedBox(height: 12),
                  Text(
                    _error!,
                    style: const TextStyle(
                      color: AppTheme.errorColor,
                      fontSize: 13,
                    ),
                  ),
                ],
                const SizedBox(height: 24),
                SizedBox(
                  width: double.infinity,
                  child: ElevatedButton(
                    onPressed: _loading ? null : _submit,
                    child: _loading
                        ? const SizedBox(
                            width: 20,
                            height: 20,
                            child: CircularProgressIndicator(
                              strokeWidth: 2,
                              color: Colors.white,
                            ),
                          )
                        : Text(_isRegister ? '注册' : '登录'),
                  ),
                ),
                const SizedBox(height: 16),
                TextButton(
                  onPressed: () => setState(() {
                    _isRegister = !_isRegister;
                    _error = null;
                  }),
                  child: Text(_isRegister ? '已有账号？去登录' : '没有账号？去注册'),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }

  @override
  void dispose() {
    _tabCtrl.dispose();
    _emailCtrl.dispose();
    _phoneCtrl.dispose();
    _passwordCtrl.dispose();
    _usernameCtrl.dispose();
    super.dispose();
  }
}
