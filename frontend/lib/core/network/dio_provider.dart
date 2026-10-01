import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../features/auth/application/auth_controller.dart';
import '../config/app_config.dart';
import 'api_client.dart';

/// The configured HTTP client.
///
/// A single instance is shared because dio holds connection state and a
/// per-call client would pay for a new handshake every time. Tests override
/// this provider to point at a mock adapter or a local server.
final dioProvider = Provider<Dio>((ref) {
  final dio = createApiClient(
    baseUrl: AppConfig.baseUrl,
    apiKeyProvider: ref.watch(apiKeyProvider),
  );

  // Closing on dispose releases the sockets instead of leaving them to the
  // garbage collector.
  ref.onDispose(dio.close);

  return dio;
});
