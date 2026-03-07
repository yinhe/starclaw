import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'services/auth_service.dart';
import 'theme.dart';
import 'screens/login_screen.dart';
import 'screens/chat_list_screen.dart';
import 'screens/videos_screen.dart';
import 'screens/music_screen.dart';
import 'screens/images_screen.dart';
import 'screens/settings_screen.dart';

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
              ? const HomeScreen()
              : const LoginScreen(),
        );
      },
    );
  }
}

class HomeScreen extends StatefulWidget {
  const HomeScreen({super.key});

  @override
  State<HomeScreen> createState() => _HomeScreenState();
}

class _HomeScreenState extends State<HomeScreen> {
  int _currentIndex = 0;

  final List<Widget> _screens = const [
    ChatListScreen(),
    VideosScreen(),
    MusicScreen(),
    ImagesScreen(),
    SettingsScreen(),
  ];

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      body: IndexedStack(index: _currentIndex, children: _screens),
      bottomNavigationBar: BottomNavigationBar(
        currentIndex: _currentIndex,
        onTap: (index) => setState(() => _currentIndex = index),
        items: const [
          BottomNavigationBarItem(icon: Icon(Icons.chat), label: '对话'),
          BottomNavigationBarItem(icon: Icon(Icons.videocam), label: '视频'),
          BottomNavigationBarItem(icon: Icon(Icons.music_note), label: '音乐'),
          BottomNavigationBarItem(icon: Icon(Icons.image), label: '图片'),
          BottomNavigationBarItem(icon: Icon(Icons.settings), label: '设置'),
        ],
      ),
    );
  }
}
