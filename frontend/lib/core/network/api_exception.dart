import 'package:dio/dio.dart';

/// A failure expressed in terms the UI can act on, rather than in terms of the
/// transport that produced it.
///
/// The backend is a separate deployable, so the client cannot assume that every
/// problem is a bug on its side. Mapping the status codes it actually documents
/// onto named cases means a screen can branch on "the key is wrong" without
/// parsing prose, and it keeps dio out of the presentation layer.
sealed class ApiException implements Exception {
  const ApiException(this.message);

  /// Text safe to show to a user. Never contains the API key or a raw stack.
  final String message;

  /// Whether retrying the same request could plausibly succeed later.
  bool get isRetryable => switch (this) {
    ApiExceptionNetwork() => true,
    ApiExceptionServer() => true,
    ApiExceptionRateLimited() => true,
    _ => false,
  };

  @override
  String toString() => '$runtimeType: $message';
}

/// No usable API key, or the server rejected the one supplied.
///
/// Surfaced from the login screen as a prompt to re-enter the key rather than as
/// an error banner over a list that can never load.
final class ApiExceptionUnauthorized extends ApiException {
  const ApiExceptionUnauthorized()
    : super('Invalid API key. Check the key configured for the server.');
}

/// The server refused the payload. Carries the backend's own wording, which
/// explains the specific rule that was broken.
final class ApiExceptionValidation extends ApiException {
  const ApiExceptionValidation(super.message);
}

/// The note does not exist, or was deleted by someone else.
final class ApiExceptionNotFound extends ApiException {
  const ApiExceptionNotFound([super.message = 'This note no longer exists.']);
}

/// The rate limit budget is spent.
///
/// [retryAfterSeconds] comes from the `Retry-After` header, which the CORS
/// middleware exposes to browser clients; without that header a Flutter web
/// build could not read it and would have to guess.
final class ApiExceptionRateLimited extends ApiException {
  const ApiExceptionRateLimited({required this.retryAfterSeconds})
    : super('Too many requests. Slowing down.');

  final int? retryAfterSeconds;
}

/// The server failed, or answered with something this client cannot parse.
final class ApiExceptionServer extends ApiException {
  const ApiExceptionServer([super.message = 'The server had a problem.']);
}

/// The request never reached the server.
final class ApiExceptionNetwork extends ApiException {
  const ApiExceptionNetwork()
    : super('Cannot reach the server. Check that it is running and reachable.');
}

/// The payload exceeded the server's 1 MiB request limit.
final class ApiExceptionPayloadTooLarge extends ApiException {
  const ApiExceptionPayloadTooLarge()
    : super('This note is too large to save.');
}

/// Maps a dio failure onto a case the UI can branch on.
///
/// The backend answers with two different error shapes — `text/plain` for
/// domain rejections (`400`, `404`, `413`) and `{"error": "..."}` for
/// authentication and rate limiting — so both are read here and nothing above
/// this function has to care.
ApiException mapDioException(DioException error) {
  switch (error.type) {
    case DioExceptionType.connectionTimeout:
    case DioExceptionType.sendTimeout:
    case DioExceptionType.receiveTimeout:
    case DioExceptionType.transformTimeout:
    case DioExceptionType.connectionError:
    case DioExceptionType.badCertificate:
    case DioExceptionType.unknown:
      // Every case here means the exchange did not produce a usable HTTP
      // response. A cancelled request is included: it carries no response either,
      // and it is not a server fault.
      return const ApiExceptionNetwork();
    case DioExceptionType.cancel:
      return const ApiExceptionNetwork();
    case DioExceptionType.badResponse:
      return _mapStatus(error.response);
  }
}

ApiException _mapStatus(Response<dynamic>? response) {
  final status = response?.statusCode;
  final body = response?.data;

  switch (status) {
    case 400:
      // http.Error writes a plain-text reason such as
      // "invalid input provided: title must be at least 3 characters".
      return ApiExceptionValidation(_plainText(body));
    case 401:
      return const ApiExceptionUnauthorized();
    case 404:
      return ApiExceptionNotFound(_plainText(body));
    case 413:
      return const ApiExceptionPayloadTooLarge();
    case 429:
      return ApiExceptionRateLimited(retryAfterSeconds: _retryAfter(response));
    default:
      // The body of a 500 is deliberately generic on the server side, so it is
      // never surfaced: internal detail stays in the server log.
      return const ApiExceptionServer();
  }
}

/// Reads the backend's `text/plain` error body, discarding it if it is empty or
/// absurdly long so an unexpected response cannot become unbounded UI text.
String _plainText(dynamic body) {
  if (body is! String || body.trim().isEmpty) {
    return 'The request was rejected.';
  }
  final trimmed = body.trim();
  return trimmed.length <= 300 ? trimmed : '${trimmed.substring(0, 300)}…';
}

/// Parses `Retry-After`, which is documented as a non-negative integer count of
/// seconds. Anything else yields null, which the UI treats as "wait and retry".
int? _retryAfter(Response<dynamic>? response) {
  final header = response?.headers.value('retry-after');
  if (header == null) return null;
  return int.tryParse(header.trim());
}
