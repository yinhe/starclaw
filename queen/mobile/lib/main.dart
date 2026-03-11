import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'services/auth_service.dart';
import 'theme.dart';
import 'screens/login_screen.dart';
import 'screens/home_screen.dart';
import 'screens/bounty_screen.dart';
import 'screens/forum_screen.dart';
import 'screens/billing_screen.dart';
import 'screens/profile_screen.dart';

void main() async {
  WidgetsFlutterBinding.ensureInitialized();
  await AuthService().init();
  SystemChrome.setSystemUIOverlayStyle(
    const SystemUiOverlayStyle(
      statusBarColor: Colors.transparent,
      statusBarIconBrightness: Brightness.light,
      systemNavigationBarColor: Color(0xFF1E293B),
    ),
  );
  runApp(const StarClawApp());
}

class StarClawApp extends StatelessWidget {
  const StarClawApp({super.key});

  @override
  Widget build(BuildContext context) {
    return ListenableBuilder(
      listenable: AuthService(),
      builder: (context, _) {
        return MaterialApp(
          title: 'StarClaw',
          debugShowCheckedModeBanner: false,
          theme: AppTheme.darkTheme,
          home: AuthService().isLoggedIn
              ? const MainShell()
              : const LoginScreen(),
        );
      },
    );
  }
}

class MainShell extends StatefulWidget {
  const MainShell({super.key});

  @override
  State<MainShell> createState() => _MainShellState();
}

class _MainShellState extends State<MainShell> {
  int _currentIndex = 0;

  final List<Widget> _screens = const [
    HomeScreen(),
    BountyScreen(),
    ForumScreen(),
    BillingScreen(),
    ProfileScreen(),
  ];

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      body: IndexedStack(index: _currentIndex, children: _screens),
      bottomNavigationBar: BottomNavigationBar(
        currentIndex: _currentIndex,
        onTap: (index) => setState(() => _currentIndex = index),
        items: const [
          BottomNavigationBarItem(icon: Icon(Icons.home_rounded), label: '首页'),
          BottomNavigationBarItem(
            icon: Icon(Icons.emoji_events_rounded),
            label: '赏金',
          ),
          BottomNavigationBarItem(icon: Icon(Icons.forum_rounded), label: '社区'),
          BottomNavigationBarItem(
            icon: Icon(Icons.account_balance_wallet_rounded),
            label: '钱包',
          ),
          BottomNavigationBarItem(
            icon: Icon(Icons.person_rounded),
            label: '我的',
          ),
        ],
      ),
    );
  }
}
