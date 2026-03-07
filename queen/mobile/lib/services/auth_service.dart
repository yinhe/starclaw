import 'package:flutter/foundation.dart';
import 'package:shared_preferences/shared_preferences.dart';

class AuthService extends ChangeNotifier {
  static final AuthService _instance = AuthService._internal();
  factory AuthService() => _instance;
  AuthService._internal();

  String? _token;
  String? _userId;
  String? _username;
  String? _email;
  bool _initialized = false;

  bool get isLoggedIn => _token != null;
  String? get token => _token;
  String? get userId => _userId;
  String? get username => _username;
  String? get email => _email;

  Future<void> init() async {
    if (_initialized) return;
    final prefs = await SharedPreferences.getInstance();
    _token = prefs.getString('token');
    _userId = prefs.getString('user_id');
    _username = prefs.getString('username');
    _email = prefs.getString('email');
    _initialized = true;
    notifyListeners();
  }

  Future<void> saveLogin({
    required String token,
    String? userId,
    String? username,
    String? email,
  }) async {
    _token = token;
    _userId = userId;
    _username = username;
    _email = email;
    final prefs = await SharedPreferences.getInstance();
    await prefs.setString('token', token);
    if (userId != null) await prefs.setString('user_id', userId);
    if (username != null) await prefs.setString('username', username);
    if (email != null) await prefs.setString('email', email);
    notifyListeners();
  }

  Future<void> logout() async {
    _token = null;
    _userId = null;
    _username = null;
    _email = null;
    final prefs = await SharedPreferences.getInstance();
    await prefs.remove('token');
    await prefs.remove('user_id');
    await prefs.remove('username');
    await prefs.remove('email');
    notifyListeners();
  }

  Future<String?> getToken() async {
    if (!_initialized) await init();
    return _token;
  }
}
