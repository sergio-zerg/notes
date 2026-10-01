import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:notes_frontend/core/storage/api_key_store.dart';
import 'package:notes_frontend/features/auth/application/auth_controller.dart';

/// An in-memory stand-in for the platform keystore.
///
/// Using a real one would need a platform channel, which a unit test has no way
/// to answer; what matters here is that the controller reads, writes and deletes
/// through the store and never keeps the key anywhere else.
class FakeApiKeyStore implements ApiKeyStore {
  String? value;

  @override
  Future<String?> read() async => value;

  @override
  Future<void> write(String apiKey) async => value = apiKey;

  @override
  Future<void> clear() async => value = null;
}

void main() {
  // Typed as the fake rather than the interface so the tests can inspect and
  // seed the stored value directly; the override below still narrows it to the
  // interface the provider expects.
  late FakeApiKeyStore store;

  setUp(() => store = FakeApiKeyStore());

  ProviderContainer containerWith(ApiKeyStore repo) {
    final container = ProviderContainer(
      overrides: [apiKeyStoreProvider.overrideWithValue(repo)],
    );
    addTearDown(container.dispose);
    return container;
  }

  group('on start', () {
    test('is unauthenticated when nothing is stored', () async {
      final container = containerWith(store);

      // The keystore answers asynchronously, so the state is unknown until it
      // has. Reading it before then must not be mistaken for "signed out".
      expect(container.read(authControllerProvider).status, AuthStatus.unknown);

      await container.read(authControllerProvider.notifier).currentKey();
      await Future<void>.delayed(Duration.zero);

      expect(container.read(authControllerProvider).status, AuthStatus.unauthenticated);
    });

    test('is authenticated when a key is stored', () async {
      store.value = 'stored-key';
      final container = containerWith(store);

      await container.read(authControllerProvider.notifier).currentKey();
      await Future<void>.delayed(Duration.zero);

      expect(container.read(authControllerProvider).status, AuthStatus.authenticated);
    });

    test('an empty stored value counts as unauthenticated', () async {
      store.value = '   ';
      final container = containerWith(store);

      await container.read(authControllerProvider.notifier).currentKey();
      await Future<void>.delayed(Duration.zero);

      expect(container.read(authControllerProvider).status, AuthStatus.unauthenticated);
    });
  });

  group('sign in', () {
    test('stores a key the server accepts', () async {
      final container = containerWith(store);

      final accepted = await container.read(authControllerProvider.notifier).signIn(
            '  good-key  ',
            verify: () async => KeyVerdict.accepted,
          );

      expect(accepted, isTrue);
      // Trimmed before storing: a stray space would be sent as part of the
      // header and the server would reject it later.
      expect(store.value, 'good-key');
      expect(container.read(authControllerProvider).status, AuthStatus.authenticated);
    });

    test('rejects an empty key without calling the server', () async {
      var verified = false;
      final container = containerWith(store);

      final accepted = await container.read(authControllerProvider.notifier).signIn(
            '   ',
            verify: () async {
              verified = true;
              return KeyVerdict.accepted;
            },
          );

      expect(accepted, isFalse);
      expect(verified, isFalse);
      expect(container.read(authControllerProvider).error, 'Enter your API key');
    });

    // A rejected key must not be persisted, or every later launch would start
    // from a credential that cannot work.
    test('does not store a key the server rejects', () async {
      final container = containerWith(store);

      final accepted = await container.read(authControllerProvider.notifier).signIn(
            'bad-key',
            verify: () async => KeyVerdict.rejected,
          );

      expect(accepted, isFalse);
      expect(store.value, isNull);
      expect(container.read(authControllerProvider).status, AuthStatus.unauthenticated);
      expect(container.read(authControllerProvider).error, contains('rejected'));
    });

    test('reports an unreachable server without blaming the key', () async {
      final container = containerWith(store);

      final accepted = await container.read(authControllerProvider.notifier).signIn(
            'key',
            verify: () async => throw Exception('unreachable'),
          );

      expect(accepted, isFalse);
      expect(store.value, isNull);
      expect(container.read(authControllerProvider).error, isNotNull);
    });

    // Regression: the check used to report success for anything that was not a
    // 401, so a timeout or a 500 signed the user in to an app that could load
    // nothing. The key was then persisted, and the failure surfaced later on
    // the notes list instead of here, where it could be explained.
    test('an undecided verdict neither signs in nor signs out', () async {
      store.value = 'working-key';
      final container = containerWith(store);
      await container.read(authControllerProvider.notifier).currentKey();
      await Future<void>.delayed(Duration.zero);
      expect(container.read(authControllerProvider).status, AuthStatus.authenticated);

      final accepted = await container.read(authControllerProvider.notifier).signIn(
            'maybe-key',
            verify: () async => KeyVerdict.undecided,
          );

      expect(accepted, isFalse);
      // The previous key survives: the check proved nothing about it.
      expect(store.value, 'working-key');
      expect(container.read(authControllerProvider).status, AuthStatus.authenticated);
      expect(
        container.read(authControllerProvider).error,
        contains('Could not reach the server'),
      );
    });

    test('a rejected key moves the session to signed out', () async {
      store.value = 'stale-key';
      final container = containerWith(store);
      await container.read(authControllerProvider.notifier).currentKey();
      await Future<void>.delayed(Duration.zero);

      await container.read(authControllerProvider.notifier).signIn(
            'stale-key',
            verify: () async => KeyVerdict.rejected,
          );

      expect(container.read(authControllerProvider).status, AuthStatus.unauthenticated);
      expect(store.value, isNull);
    });
  });

  group('sign out', () {
    test('clears the stored key and returns to the login state', () async {
      store.value = 'key';
      final container = containerWith(store);

      await container.read(authControllerProvider.notifier).signIn(
            'key',
            verify: () async => KeyVerdict.accepted,
          );
      await container.read(authControllerProvider.notifier).signOut();

      expect(store.value, isNull);
      expect(container.read(authControllerProvider).status, AuthStatus.unauthenticated);
    });

    // The cached key must go too: leaving it in memory would keep attaching it
    // to requests after the user asked to sign out.
    test('stops attaching the key to requests', () async {
      final container = containerWith(store);

      await container.read(authControllerProvider.notifier).signIn(
            'secret-key',
            verify: () async => KeyVerdict.accepted,
          );
      await container.read(authControllerProvider.notifier).signOut();

      expect(await container.read(authControllerProvider.notifier).currentKey(), isNull);
    });
  });

  test('currentKey returns the stored key for outgoing requests', () async {
    store.value = 'existing';
    final container = containerWith(store);

    expect(await container.read(authControllerProvider.notifier).currentKey(), 'existing');
  });

  test('a freshly signed-in key is what requests carry', () async {
    final container = containerWith(store);

    await container.read(authControllerProvider.notifier).signIn(
          'new-key',
          verify: () async => KeyVerdict.accepted,
        );

    // Read through the same function the HTTP interceptor uses, so this is the
    // value that would actually be attached to a request.
    final provider = container.read(apiKeyProvider);

    expect(await provider(), 'new-key');
  });
}


