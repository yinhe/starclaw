import 'package:dio/dio.dart';
import 'package:flutter/foundation.dart';
import 'auth_service.dart';

class ApiService {
  static final ApiService _instance = ApiService._internal();
  factory ApiService() => _instance;

  late final Dio dio;
  // Queen API base URL (change for production)
  static const String baseUrl = 'https://queen.starclaw.me/api';

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

  Future<Response> loginPhone(String phone, String password) =>
      dio.post('/auth/login', data: {'phone': phone, 'password': password});

  Future<Response> register({
    String? email,
    String? phone,
    required String nickname,
    required String password,
  }) => dio.post(
    '/auth/register',
    data: {
      if (email != null) 'email': email,
      if (phone != null) 'phone': phone,
      'nickname': nickname,
      'password': password,
    },
  );

  // ── User Profile ──
  Future<Response> getProfile() => dio.get('/user/profile');
  Future<Response> updateProfile({String? nickname, String? bio}) => dio.put(
    '/user/profile',
    data: {
      if (nickname != null) 'nickname': nickname,
      if (bio != null) 'bio': bio,
    },
  );
  Future<Response> changePassword(String oldPwd, String newPwd) => dio.put(
    '/user/password',
    data: {'old_password': oldPwd, 'new_password': newPwd},
  );

  // ── Billing ──
  Future<Response> getBalance() => dio.get('/pay/balance');
  Future<Response> getPackages() => dio.get('/pay/packages');
  Future<Response> getPayMethods() => dio.get('/pay/methods');
  Future<Response> getTransactions({int page = 1}) =>
      dio.get('/pay/transactions', queryParameters: {'page': page});
  Future<Response> getOrders({int page = 1}) =>
      dio.get('/pay/orders', queryParameters: {'page': page});
  Future<Response> createOrder(String packageId, String payMethod) => dio.post(
    '/pay/create',
    data: {'package_id': packageId, 'pay_method': payMethod},
  );
  Future<Response> queryOrderStatus(String orderNo) =>
      dio.get('/pay/order/$orderNo/status');

  // ── Bounty ──
  Future<Response> listBounties({
    String? category,
    String? status,
    int? page,
  }) => dio.get(
    '/bounty',
    queryParameters: {
      if (category != null) 'category': category,
      if (status != null) 'status': status,
      if (page != null) 'page': page,
    },
  );
  Future<Response> getBounty(String id) => dio.get('/bounty/$id');
  Future<Response> getBountyStats() => dio.get('/bounty/stats');
  Future<Response> getBountyCategories() => dio.get('/bounty/categories');
  Future<Response> claimBounty(String id, String userId, String userName) =>
      dio.post(
        '/bounty/$id/claim',
        data: {'user_id': userId, 'user_name': userName},
      );
  Future<Response> deliverBounty(String id, String notes) =>
      dio.post('/bounty/$id/deliver', data: {'delivery_notes': notes});
  Future<Response> cancelBounty(String id) => dio.post('/bounty/$id/cancel');
  Future<Response> disputeBounty(String id, String reason) =>
      dio.post('/bounty/$id/dispute', data: {'reason': reason});

  // ── Forum ──
  Future<Response> listForumPosts({String? categoryId, int? page}) => dio.get(
    '/forum/posts',
    queryParameters: {
      if (categoryId != null) 'category_id': categoryId,
      if (page != null) 'page': page,
    },
  );
  Future<Response> getForumPost(String id) => dio.get('/forum/posts/$id');
  Future<Response> getForumCategories() => dio.get('/forum/categories');
  Future<Response> createForumPost({
    required String authorId,
    required String authorName,
    required String title,
    required String content,
    String? categoryId,
    String? tags,
  }) => dio.post(
    '/forum/posts',
    data: {
      'author_id': authorId,
      'author_name': authorName,
      'title': title,
      'content': content,
      if (categoryId != null) 'category_id': categoryId,
      if (tags != null) 'tags': tags,
    },
  );
  Future<Response> createForumReply(
    String postId, {
    required String authorId,
    required String authorName,
    required String content,
  }) => dio.post(
    '/forum/posts/$postId/replies',
    data: {
      'author_id': authorId,
      'author_name': authorName,
      'content': content,
    },
  );
  Future<Response> likeForumPost(String postId, String userId) =>
      dio.post('/forum/posts/$postId/like', data: {'user_id': userId});

  // ── Node Binding ──
  Future<Response> listNodes() => dio.get('/user/nodes');
  Future<Response> bindNode({
    required String nodeId,
    required String localUserId,
    String? nodeName,
    String? nodeAddr,
  }) => dio.post(
    '/user/nodes',
    data: {
      'node_id': nodeId,
      'local_user_id': localUserId,
      if (nodeName != null) 'node_name': nodeName,
      if (nodeAddr != null) 'node_addr': nodeAddr,
    },
  );
  Future<Response> unbindNode(String nodeId) =>
      dio.delete('/user/nodes/$nodeId');

  // ── Reports ──
  Future<Response> getReportReasons() => dio.get('/reports/reasons');
  Future<Response> submitReport({
    required String targetType,
    required String targetId,
    required String reason,
    String? targetTitle,
    String? authorId,
    String? detail,
  }) => dio.post(
    '/reports',
    data: {
      'target_type': targetType,
      'target_id': targetId,
      'reason': reason,
      if (targetTitle != null) 'target_title': targetTitle,
      if (authorId != null) 'author_id': authorId,
      if (detail != null) 'detail': detail,
    },
  );

  // Helper: resolve full URL
  String resolveUrl(String url) {
    if (url.startsWith('http')) return url;
    return 'https://queen.starclaw.me$url';
  }
}
