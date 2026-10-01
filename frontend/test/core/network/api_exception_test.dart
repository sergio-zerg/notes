import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:notes_frontend/core/network/api_exception.dart';

/// A `Headers` map needs every value as a list, which is fiddly at a call site;
/// this keeps each test focused on the status code instead.
Map<String, List<String>> _headers(Map<String, String> flat) =>
    flat.map((key, value) => MapEntry(key, [value]));

/// Builds a DioException carrying an HTTP response, the shape a real failure
/// takes when the server answered.
DioException _badResponse(
  int status, {
  Object? body,
  Map<String, List<String>>? headers,
}) {
  return DioException(
    requestOptions: RequestOptions(path: '/notes'),
    type: DioExceptionType.badResponse,
    response: Response<dynamic>(
      requestOptions: RequestOptions(path: '/notes'),
      statusCode: status,
      data: body,
      headers: headers == null ? Headers() : Headers.fromMap(headers),
    ),
  );
}

void main() {
  group('mapDioException', () {
    test('a 401 becomes an unauthorized failure', () {
      final result = mapDioException(
        _badResponse(401, body: '{"error": "valid API key required"}'),
      );

      expect(result, isA<ApiExceptionUnauthorized>());
      // Not retryable: the same key will be rejected again.
      expect(result.isRetryable, isFalse);
    });

    // The backend writes domain rejections as text/plain via http.Error, so the
    // reason has to survive the mapping or the user sees a generic message.
    test('a 400 keeps the server\'s plain-text reason', () {
      final result = mapDioException(_badResponse(
        400,
        body: 'invalid input provided: title must be at least 3 characters',
      ));

      expect(result, isA<ApiExceptionValidation>());
      expect(result.message, contains('at least 3 characters'));
    });

    test('a 400 with an empty body still yields a message', () {
      final result = mapDioException(_badResponse(400, body: '   '));

      expect(result.message, 'The request was rejected.');
    });

    // An unexpected body must not become unbounded text in the UI.
    test('a 400 with a very long body is truncated', () {
      final result = mapDioException(_badResponse(400, body: 'x' * 5000));

      expect(result.message.length, lessThanOrEqualTo(301));
      expect(result.message, endsWith('…'));
    });

    test('a 404 becomes a not-found failure', () {
      expect(mapDioException(_badResponse(404, body: 'note not found')),
          isA<ApiExceptionNotFound>());
    });

    test('a 413 becomes a payload-too-large failure', () {
      expect(mapDioException(_badResponse(413, body: 'request body too large')),
          isA<ApiExceptionPayloadTooLarge>());
    });

    test('a 429 is retryable and reads Retry-After', () {
      final result = mapDioException(_badResponse(
        429,
        body: '{"error": "rate limit exceeded"}',
        headers: _headers({'retry-after': '2'}),
      ));

      expect(result, isA<ApiExceptionRateLimited>());
      expect((result as ApiExceptionRateLimited).retryAfterSeconds, 2);
      expect(result.isRetryable, isTrue);
    });

    test('a 429 without a usable Retry-After yields null', () {
      final result = mapDioException(_badResponse(
        429,
        headers: _headers({'retry-after': 'Wed, 21 Oct 2026 07:28:00 GMT'}),
      ));

      // A value this client cannot interpret is dropped rather than guessed at,
      // so the UI falls back to a plain wait.
      expect((result as ApiExceptionRateLimited).retryAfterSeconds, isNull);
    });

    // The server replaces a 500 body with generic text precisely so internal
    // detail never reaches a client, so the client must not display it either.
    test('a 500 never surfaces the body', () {
      final result = mapDioException(_badResponse(
        500,
        body: 'pq: password authentication failed for user "postgres"',
      ));

      expect(result, isA<ApiExceptionServer>());
      expect(result.message, 'The server had a problem.');
      expect(result.message, isNot(contains('postgres')));
    });

    test('a connection failure is retryable', () {
      final result = mapDioException(DioException(
        requestOptions: RequestOptions(path: '/notes'),
        type: DioExceptionType.connectionError,
      ));

      expect(result, isA<ApiExceptionNetwork>());
      expect(result.isRetryable, isTrue);
    });

    test('a timeout is treated as a network failure', () {
      expect(
        mapDioException(DioException(
          requestOptions: RequestOptions(path: '/notes'),
          type: DioExceptionType.connectionTimeout,
        )),
        isA<ApiExceptionNetwork>(),
      );
    });

    test('a validation failure is not retryable', () {
      expect(mapDioException(_badResponse(400)).isRetryable, isFalse);
    });
  });
}
