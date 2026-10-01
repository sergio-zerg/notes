import 'package:dio/dio.dart';
import 'package:flutter/foundation.dart';

import 'api_exception.dart';

/// Reads the API key from wherever it lives at the moment a request goes out.
///
/// The key is supplied by the user on the login screen and held in secure
/// storage, so the interceptor cannot capture it once at construction time —
/// it has to ask on every call. That also means a key change takes effect
/// immediately, without rebuilding the client.
typedef ApiKeyProvider = Future<String?> Function();

/// Builds the configured dio instance.
///
/// Everything platform-specific about the base URL is resolved here, so the
/// rest of the app depends on one abstract client and can be tested by
/// substituting it.
Dio createApiClient({
  required String baseUrl,
  required ApiKeyProvider apiKeyProvider,
  Duration? timeout,
}) {
  final dio = Dio(
    BaseOptions(
      baseUrl: baseUrl,
      connectTimeout: timeout ?? const Duration(seconds: 10),
      receiveTimeout: timeout ?? const Duration(seconds: 10),
      sendTimeout: timeout ?? const Duration(seconds: 10),
      // Errors are translated by the interceptor below, so a non-2xx response
      // must reach it as a DioException rather than as data.
      contentType: Headers.jsonContentType,
      responseType: ResponseType.json,
    ),
  );

  dio.interceptors.add(
    InterceptorsWrapper(
      onRequest: (options, handler) async {
        // Added per request rather than stored on the instance: the key must
        // never sit in a field that a log line or a crash dump can reach.
        final key = await apiKeyProvider();
        if (key != null && key.isNotEmpty) {
          options.headers['X-API-Key'] = key;
        }
        handler.next(options);
      },
      onError: (error, handler) {
        // In debug builds the raw failure is printed, which is what makes a
        // misconfigured base URL visible while developing. Release builds stay
        // silent so a production log never carries request detail.
        if (kDebugMode) {
          debugPrint(
            'API ${error.requestOptions.method} '
            '${error.requestOptions.path} failed: ${error.type}',
          );
        }
        handler.next(error);
      },
    ),
  );

  return dio;
}

/// Unwraps a dio call into either its body or a translated [ApiException].
///
/// Every repository call goes through this, so a screen never sees a
/// DioException and no `try`/`catch` is needed to keep dio out of the UI.
///
/// The wire type and the domain type are separate parameters: dio returns
/// whatever JSON shape the server sent, and [parse] turns it into the model the
/// repository returns. Declaring the request as `Future<Response<dynamic>>`
/// keeps dio from trying to cast to a type that never appears on the wire.
Future<T> guardRequest<T>(
  Future<Response<dynamic>> Function() request, {
  required T Function(dynamic data) parse,
}) async {
  try {
    final response = await request();
    return parse(response.data);
  } on DioException catch (error) {
    throw mapDioException(error);
  }
}

/// The same translation for a call whose response body is not used.
///
/// DELETE answers 204, so there is nothing to parse. This exists because
/// [guardRequest] must produce a value, and returning a placeholder from a
/// `void` method would only push the fiction to the caller.
Future<void> guardVoidRequest(
  Future<Response<dynamic>> Function() request,
) async {
  try {
    await request();
  } on DioException catch (error) {
    throw mapDioException(error);
  }
}
