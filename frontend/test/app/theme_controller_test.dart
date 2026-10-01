import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:notes_frontend/app/theme_controller.dart';
import 'package:notes_frontend/core/storage/theme_preference_store.dart';
import 'package:shared_preferences/shared_preferences.dart';

/// A store that records what was written, standing in for the real one so the
/// test needs no platform channel.
class FakeThemeStore implements ThemePreferenceStore {
  FakeThemeStore([this.stored]);

  ThemeMode? stored;
  ThemeMode? lastWritten;

  @override
  Future<ThemeMode?> read() async => stored;

  @override
  Future<void> write(ThemeMode mode) async {
    lastWritten = mode;
    stored = mode;
  }
}

/// Lets the controller's asynchronous restore run to completion.
///
/// The read goes through a future and then back into the container, so a single
/// microtask is not enough to observe the result — hence the real delay.
Future<void> _settle() => Future<void>.delayed(const Duration(milliseconds: 20));

void main() {
  late FakeThemeStore store;

  setUp(() => store = FakeThemeStore());

  ProviderContainer containerWith(ThemePreferenceStore repo) {
    final container = ProviderContainer(
      overrides: [themePreferenceStoreProvider.overrideWithValue(repo)],
    );
    addTearDown(container.dispose);
    return container;
  }

  group('initial mode', () {
    // Following the device is the answer most users would have chosen, and the
    // platform guidance asks for it by default.
    test('follows the system when nothing is stored', () async {
      final container = containerWith(store);

      expect(container.read(themeControllerProvider), ThemeMode.system);
      await _settle();
      expect(container.read(themeControllerProvider), ThemeMode.system);
    });

    test('restores a stored choice', () async {
      store.stored = ThemeMode.dark;
      final container = containerWith(store);

      // Read first: the provider is lazy, so it is only built on first access.
      // Waiting before touching it would let the timer elapse against a
      // controller that does not exist yet.
      expect(container.read(themeControllerProvider), ThemeMode.system);
      await _settle();

      expect(container.read(themeControllerProvider), ThemeMode.dark);
    });

    });

  group('ThemePreferenceStore', () {
    setUp(() => SharedPreferences.setMockInitialValues({}));

    test('round-trips every mode through storage', () async {
      final store = ThemePreferenceStore();

      for (final mode in ThemeMode.values) {
        await store.write(mode);
        expect(await store.read(), mode, reason: '$mode did not survive a round trip');
      }
    });

    test('reads null when nothing was ever stored', () async {
      expect(await ThemePreferenceStore().read(), isNull);
    });

    // A value written by a future version of the app must not crash this one,
    // and must not be guessed at: the system setting is the safer reading of
    // "unrecognised".
    test('ignores a value it does not recognise', () async {
      SharedPreferences.setMockInitialValues({'notes_theme_mode': 'sepia'});

      expect(await ThemePreferenceStore().read(), isNull);
    });

    test('a stored choice survives a new store instance', () async {
      await ThemePreferenceStore().write(ThemeMode.dark);

      // A fresh instance stands in for the next launch of the app.
      expect(await ThemePreferenceStore().read(), ThemeMode.dark);
    });
  });

  group('setMode', () {
    test('updates the state and persists it', () async {
      final container = containerWith(store);

      await container.read(themeControllerProvider.notifier).setMode(
        ThemeMode.dark,
      );

      expect(container.read(themeControllerProvider), ThemeMode.dark);
      expect(store.lastWritten, ThemeMode.dark);
    });

    // Theme and storage must not be able to disagree, or a restart would
    // silently revert the choice.
    test('persists every mode it accepts', () async {
      final container = containerWith(store);

      for (final mode in ThemeMode.values) {
        await container.read(themeControllerProvider.notifier).setMode(mode);
        expect(store.lastWritten, mode);
      }
    });
  });

  group('cycle', () {
    test('moves system → light → dark → system', () async {
      final container = containerWith(store);
      final controller = container.read(themeControllerProvider.notifier);

      expect(container.read(themeControllerProvider), ThemeMode.system);

      await controller.cycle();
      expect(container.read(themeControllerProvider), ThemeMode.light);

      await controller.cycle();
      expect(container.read(themeControllerProvider), ThemeMode.dark);

      await controller.cycle();
      expect(container.read(themeControllerProvider), ThemeMode.system);
    });

    test('cycles from light without waiting for a restore', () async {
      store.stored = ThemeMode.light;
      final container = containerWith(store);
      container.read(themeControllerProvider);
      await _settle();

      await container.read(themeControllerProvider.notifier).cycle();

      expect(container.read(themeControllerProvider), ThemeMode.dark);
    });
  });

  group('labels and icons', () {
    test('every mode has a distinct icon', () {
      final icons = ThemeMode.values.map(themeModeIcon).toSet();

      // A repeated icon would make the control ambiguous: three states and two
      // glyphs means the user cannot tell which is next.
      expect(icons, hasLength(ThemeMode.values.length));
    });

    test('every mode has a label', () {
      for (final mode in ThemeMode.values) {
        expect(themeModeLabel(mode), isNotEmpty);
      }
    });
  });
}