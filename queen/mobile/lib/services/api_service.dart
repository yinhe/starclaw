import 'package:dio/dio.dart';
import 'package:flutter/foundation.dart';
import 'auth_service.dart';

class ApiService {
  static final ApiService _instance = ApiService._internal();
  factory ApiService() => _instance;

  late final Dio dio;
  // Change this to your server URL
  static const String baseUrl = 'https://api.starclaw.me/v1';

  ApiService._internal() {
    dio = Dio(
      BaseOptions(
        baseUrl: baseUrl,
        connectTimeout: const Duration(seconds: 15),
        receiveTimeout: const Duration(seconds: 30),
        headers: {'Content-Type': 'application/json'},
      ),
    );

    dio.interceptors.add(
      InterceptorsWrapper(
        onRequest: (options, handler) async {
          final token = await AuthService().getToken();
          if (token != null) {
            options.headers['Authorization'] = 'Bearer $token';
          }
          return handler.next(options);
        },
        onError: (error, handler) async {
          if (error.response?.statusCode == 401) {
            await AuthService().logout();
          }
          return handler.next(error);
        },
      ),
    );

    if (kDebugMode) {
      dio.interceptors.add(
        LogInterceptor(requestBody: true, responseBody: true),
      );
    }
  }

  // ── Auth ──
  Future<Response> login(String email, String password) =>
      dio.post('/auth/login', data: {'email': email, 'password': password});

  Future<Response> register(String email, String username, String password) =>
      dio.post(
        '/auth/register',
        data: {'email': email, 'username': username, 'password': password},
      );

  Future<Response> phoneLogin(String phone, String password) => dio.post(
    '/auth/phone/login',
    data: {'phone': phone, 'password': password},
  );

  Future<Response> phoneRegister(
    String phone,
    String password, {
    String? username,
  }) => dio.post(
    '/auth/phone/register',
    data: {
      'phone': phone,
      'password': password,
      if (username != null && username.isNotEmpty) 'username': username,
    },
  );

  // ── Agents ──
  Future<Response> listAgents() => dio.get('/agents');
  Future<Response> ensureSuperAgent() => dio.post('/agents/super-agent');

  // ── Conversations ──
  Future<Response> listConversations() => dio.get('/conversations');
  Future<Response> getMessages(String conversationId) =>
      dio.get('/conversations/$conversationId/messages');
  Future<Response> deleteConversation(String id) =>
      dio.delete('/conversations/$id');
  Future<Response> renameConversation(String id, String title) =>
      dio.put('/conversations/$id', data: {'title': title});

  // ── Chat (non-streaming) ──
  Future<Response> sendChat({
    required String agentId,
    String? conversationId,
    required String message,
  }) => dio.post(
    '/chat/completions',
    data: {
      'agent_id': agentId,
      'conversation_id': conversationId ?? '',
      'message': message,
      'stream': false,
    },
  );

  // ── Chat (streaming via SSE) ──
  static String getChatStreamUrl() => '$baseUrl/chat/completions';

  // ── Videos ──
  Future<Response> listVideos() => dio.get('/videos');
  Future<Response> deleteVideo(String id) => dio.delete('/videos/$id');
  Future<Response> retryVideo(String id) => dio.post('/videos/$id/retry');
  Future<Response> regenerateVideo(String id) =>
      dio.post('/videos/$id/regenerate');
  Future<Response> remergeVideo(String id) => dio.post('/videos/$id/remerge');
  Future<Response> dubVideo(
    String id,
    String text,
    String voice, {
    String subtitleStyle = 'auto',
  }) => dio.post(
    '/videos/$id/dub',
    data: {'text': text, 'voice': voice, 'subtitle_style': subtitleStyle},
  );
  Future<Response> addMusicToVideo(
    String id,
    String musicId, {
    String? lyricsSrt,
  }) => dio.post(
    '/videos/$id/add-music',
    data: {'music_id': musicId, 'lyrics_srt': lyricsSrt ?? ''},
  );
  Future<Response> listVoices() => dio.get('/videos/voices');

  // ── Music ──
  Future<Response> listMusic() => dio.get('/music');
  Future<Response> deleteMusic(String id) => dio.delete('/music/$id');

  // ── Images ──
  Future<Response> listImages() => dio.get('/images');
  Future<Response> deleteImage(String id) => dio.delete('/images/$id');

  // ── Settings ──
  Future<Response> getProfile() => dio.get('/settings/profile');
  Future<Response> updateProfile({
    String? username,
    String? email,
    String? phone,
  }) => dio.put(
    '/settings/profile',
    data: {
      if (username != null && username.isNotEmpty) 'username': username,
      if (email != null && email.isNotEmpty) 'email': email,
      if (phone != null && phone.isNotEmpty) 'phone': phone,
    },
  );
  Future<Response> changePassword(String oldPassword, String newPassword) =>
      dio.put(
        '/settings/password',
        data: {'old_password': oldPassword, 'new_password': newPassword},
      );

  // ── Dashboard ──
  Future<Response> getStats() => dio.get('/dashboard/stats');

  // ── Models ──
  Future<Response> listModels() => dio.get('/models');

  // ── Notifications ──
  Future<Response> getUnreadCount() => dio.get('/notifications/unread-count');
  Future<Response> listNotifications({bool unread = false}) => dio.get(
    '/notifications',
    queryParameters: unread ? {'unread': 'true'} : {},
  );
  Future<Response> markNotificationsRead({List<String>? ids}) =>
      dio.post('/notifications/read', data: {'ids': ids});

  // ── Multimodal ──
  Future<Response> uploadImage(String filePath) {
    final formData = FormData.fromMap({
      'file': MultipartFile.fromFileSync(filePath),
    });
    return dio.post(
      '/multimodal/upload-image',
      data: formData,
      options: Options(headers: {'Content-Type': 'multipart/form-data'}),
    );
  }

  // Helper: resolve full URL for media
  String resolveUrl(String url) {
    if (url.startsWith('http')) return url;
    return 'https://api.starclaw.me$url';
  }
}
