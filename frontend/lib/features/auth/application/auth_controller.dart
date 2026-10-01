import 'package:flutter/foundation.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/network/api_exception.dart';
import '../../../core/storage/api_key_store.dart';

/// Where the session currently stands.
///
/// Three states rather than a nullable key: "not checked yet" and "no key" look
/// the same in a nullable field, and collapsing them flashes the login screen
/// on every cold start before the keystore has answered.
enum AuthStatus { unknown, authenticated, unauthenticated }

/// The outcome of checking a key against the server.
///
/// Three outcomes rather than a bool, because "the server said no" and "the
/// server did not answer" call for different messages and different session
/// state. Collapsing them into a bool let an unreachable server report a
/// successful sign-in, and the user met the real failure on the notes list
/// instead of on the screen that could have explained it.
enum KeyVerdict {
  /// The server accepted the key.
  accepted,

  /// The server refused the key.
  rejected,

  /// The check could not be completed — a timeout, a DNS failure, a 500. The
  /// key's validity is unknown and must not be judged either way.
  undecided,
}

@immutable
class AuthState {
  const AuthState({
    this.status = AuthStatus.unknown,
    this.isSubmitting = false,
    this.error,
  });

  final AuthStatus status;
  final bool isSubmitting;

  /// Set when the last attempt failed, cleared when a new one starts.
  final String? error;

  AuthState copyWith({
    AuthStatus? status,
    bool? isSubmitting,
    String? error,
    bool clearError = false,
  }) {
    return AuthState(
      status: status ?? this.status,
      isSubmitting: isSubmitting ?? this.isSubmitting,
      error: clearError ? null : (error ?? this.error),
    );
  }
}

/// Owns the session: reads the stored key on start, verifies it on sign-in,
/// and forgets it on sign-out.
///
/// Verification is a real request rather than a local check, because the server
/// is the only party that knows whether a key is still configured. A key can be
/// revoked server-side while the app holds it, and only a request reveals that.
class AuthController extends Notifier<AuthState> {
  @override
  AuthState build() {
    // Restoring is asynchronous, so the initial state stays `unknown` and the
    // app shows a splash rather than guessing.
    _restore();
    return const AuthState();
  }

  /// The key cached for the duration of the session.
  ///
  /// Held in memory so the interceptor can attach it to every request without
  /// a keystore round trip on each one. It is never logged.
  String? _cachedKey;

  /// Returns the key for the current request, or null when there is none.
  Future<String?> currentKey() async {
    final cached = _cachedKey;
    if (cached != null) return cached;
    return _readStoredKey();
  }

  /// Reads the persisted key, caching it for subsequent requests.
  ///
  /// The value is trimmed: a stored key is compared against a header the server
  /// reads verbatim, so a stray space would make a credential that looks present
  /// but cannot authenticate.
  Future<String?> _readStoredKey() async {
    final stored = (await ref.read(apiKeyStoreProvider).read())?.trim();
    _cachedKey = (stored == null || stored.isEmpty) ? null : stored;
    return _cachedKey;
  }

  /// Decides the initial route once the keystore has answered.
  Future<void> _restore() async {
    final stored = await _readStoredKey();
    state = state.copyWith(
      status: stored == null
          ? AuthStatus.unauthenticated
          : AuthStatus.authenticated,
    );
  }

  /// Checks [apiKey] against the server and stores it when it works.
  ///
  /// Returns true on success so the router can navigate away without waiting
  /// for a state rebuild.
  Future<bool> signIn(
    String apiKey, {
    required Future<KeyVerdict> Function() verify,
  }) async {
    final trimmed = apiKey.trim();

    if (trimmed.isEmpty) {
      state = state.copyWith(error: 'Enter your API key');
      return false;
    }

    state = state.copyWith(isSubmitting: true, clearError: true);

    final previousKey = _cachedKey;
    // Attached to the very next request, so verification uses the key being
    // tested rather than the one already stored.
    _cachedKey = trimmed;

    try {
      final verdict = await verify();

      if (verdict != KeyVerdict.accepted) {
        if (verdict == KeyVerdict.rejected) {
          // A key the server has refused must not survive in the keystore: the
          // next launch would read it back, mark the session authenticated and
          // fail every request with it.
          _cachedKey = null;
          await ref.read(apiKeyStoreProvider).clear();

          state = state.copyWith(
            isSubmitting: false,
            status: AuthStatus.unauthenticated,
            error: 'That key was rejected by the server',
          );
          return false;
        }

        // Undecided: the server never answered, so the previous key is neither
        // vindicated nor condemned. Both memory and the keystore keep it, and
        // the session keeps its status — claiming the user is signed out would
        // throw away a credential that may well work.
        _cachedKey = previousKey;
        state = state.copyWith(
          isSubmitting: false,
          error: 'Could not reach the server to check the key',
        );
        return false;
      }

      await ref.read(apiKeyStoreProvider).write(trimmed);
      state = state.copyWith(
        isSubmitting: false,
        status: AuthStatus.authenticated,
        clearError: true,
      );
      return true;
    } catch (error) {
      _cachedKey = previousKey;
      state = state.copyWith(isSubmitting: false, error: _messageFor(error));
      return false;
    }
  }

  /// Drops the key from memory and from the keystore.
  Future<void> signOut() async {
    _cachedKey = null;
    await ref.read(apiKeyStoreProvider).clear();
    state = const AuthState(status: AuthStatus.unauthenticated);
  }

  String _messageFor(Object error) {
    return switch (error) {
      ApiExceptionUnauthorized() => 'That key was rejected by the server',
      ApiExceptionNetwork() =>
        'Cannot reach the server. Check that it is running.',
      ApiException() => error.message,
      _ => 'Something went wrong while signing in',
    };
  }
}

final apiKeyStoreProvider = Provider<ApiKeyStore>((ref) => ApiKeyStore());

final authControllerProvider = NotifierProvider<AuthController, AuthState>(
  AuthController.new,
);

/// The key to attach to outgoing requests.
///
/// Reading through the controller means a sign-out takes effect on the next
/// call without rebuilding the HTTP client.
final apiKeyProvider = Provider<Future<String?> Function()>((ref) {
  return () => ref.read(authControllerProvider.notifier).currentKey();
});
