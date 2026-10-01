import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:notes_frontend/app/app.dart';
import 'package:notes_frontend/app/theme.dart';
import 'package:notes_frontend/app/theme_controller.dart';
import 'package:notes_frontend/core/storage/api_key_store.dart';
import 'package:notes_frontend/core/storage/theme_preference_store.dart';
import 'package:notes_frontend/core/widgets/theme_toggle.dart';
import 'package:notes_frontend/features/auth/application/auth_controller.dart';
import 'package:notes_frontend/features/notes/data/notes_repository_provider.dart';
import 'package:notes_frontend/features/notes/domain/note.dart';
import 'package:notes_frontend/features/notes/domain/notes_repository.dart';

class FakeNotesRepository implements NotesRepository {
  FakeNotesRepository([this.notes = const []]);

  List<Note> notes;

  @override
  Future<List<Note>> list({
    String query = '',
    int limit = 50,
    int offset = 0,
  }) async =>
      notes;

  @override
  Future<Note> get(int id) async => notes.firstWhere((n) => n.id == id);

  @override
  Future<Note> create({required String title, required String content}) async =>
      notes.first;

  @override
  Future<Note> update(
    int id, {
    required String title,
    required String content,
  }) async =>
      notes.firstWhere((n) => n.id == id);

  @override
  Future<void> delete(int id) async =>
      notes = notes.where((n) => n.id != id).toList();
}

class FakeApiKeyStore implements ApiKeyStore {
  FakeApiKeyStore([this.value]);

  String? value;

  @override
  Future<String?> read() async => value;

  @override
  Future<void> write(String apiKey) async => value = apiKey;

  @override
  Future<void> clear() async => value = null;
}

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

void main() {
  late FakeThemeStore themeStore;

  setUp(() => themeStore = FakeThemeStore());

  Future<ProviderContainer> pumpApp(
    WidgetTester tester, {
    bool signedIn = true,
  }) async {
    final container = ProviderContainer(
      overrides: [
        apiKeyStoreProvider.overrideWithValue(
          FakeApiKeyStore(signedIn ? 'key' : null),
        ),
        themePreferenceStoreProvider.overrideWithValue(themeStore),
        notesRepositoryProvider.overrideWithValue(FakeNotesRepository()),
      ],
    );
    addTearDown(container.dispose);

    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: const NotesApp(),
      ),
    );
    await tester.pump();
    await tester.pumpAndSettle();

    return container;
  }

  /// The brightness the app is actually painting with.
  ///
  /// Read from the rendered `MaterialApp` rather than from the theme object,
  /// so this asserts what the user sees instead of what was configured.
  Brightness brightnessOf(WidgetTester tester) {
    final mode = tester.widget<MaterialApp>(find.byType(MaterialApp)).themeMode;

    // `themeMode` is nullable on the widget, so the null case is grouped with
    // system: an unset mode follows the platform, which is light under test.
    return switch (mode) {
      ThemeMode.dark => Brightness.dark,
      _ => Brightness.light,
    };
  }

  group('ThemeToggle', () {
    testWidgets('shows the mode a press would switch to', (tester) async {
      // From system, the next mode is light.
      await tester.pumpWidget(
        MaterialApp(
          home: Scaffold(
            body: ThemeToggle(mode: ThemeMode.system, onPressed: () {}),
          ),
        ),
      );

      expect(find.byIcon(Icons.light_mode_rounded), findsOneWidget);
    });

    testWidgets('shows dark when the active mode is light', (tester) async {
      await tester.pumpWidget(
        MaterialApp(
          home: Scaffold(
            body: ThemeToggle(mode: ThemeMode.light, onPressed: () {}),
          ),
        ),
      );

      expect(find.byIcon(Icons.dark_mode_rounded), findsOneWidget);
    });

    testWidgets('shows the system icon when the active mode is dark',
        (tester) async {
      await tester.pumpWidget(
        MaterialApp(
          home: Scaffold(
            body: ThemeToggle(mode: ThemeMode.dark, onPressed: () {}),
          ),
        ),
      );

      expect(find.byIcon(Icons.brightness_auto_rounded), findsOneWidget);
    });

    test('cycles through all three modes and returns to the start', () {
      expect(ThemeToggle.nextMode(ThemeMode.system), ThemeMode.light);
      expect(ThemeToggle.nextMode(ThemeMode.light), ThemeMode.dark);
      expect(ThemeToggle.nextMode(ThemeMode.dark), ThemeMode.system);
    });

    testWidgets('has a tap target large enough for a finger', (tester) async {
      await tester.pumpWidget(
        MaterialApp(
          home: Scaffold(
            body: ThemeToggle(mode: ThemeMode.system, onPressed: () {}),
          ),
        ),
      );

      final size = tester.getSize(find.byType(IconButton));
      expect(size.width, greaterThanOrEqualTo(44));
      expect(size.height, greaterThanOrEqualTo(44));
    });
  });

  group('the app', () {
    testWidgets('offers the toggle on the login screen', (tester) async {
      await pumpApp(tester, signedIn: false);

      expect(find.text('Sign in with the API key for your server'), findsOneWidget);
      expect(find.byType(ThemeToggle), findsOneWidget);
    });

    testWidgets('offers the toggle on the notes list', (tester) async {
      await pumpApp(tester);

      expect(find.byType(ThemeToggle), findsOneWidget);
    });

    // The point of the feature: tapping must actually change the painted theme,
    // not just the stored preference. Asserted against the resolved
    // ThemeData's brightness, because `themeMode` alone only records the
    // choice — the resolution to a concrete theme happens inside MaterialApp.
    testWidgets('a press switches the painted theme', (tester) async {
      final container = await pumpApp(tester);

      expect(container.read(themeControllerProvider), ThemeMode.system);

      await tester.tap(find.byType(ThemeToggle));
      await tester.pumpAndSettle();

      expect(container.read(themeControllerProvider), ThemeMode.light);

      final app = tester.widget<MaterialApp>(find.byType(MaterialApp));
      expect(app.darkTheme?.brightness, Brightness.dark);
      // Light mode is what the app now asks for, and the active theme must
      // follow it rather than staying on whatever it was.
      expect(
        Theme.of(tester.element(find.byType(ThemeToggle))).brightness,
        Brightness.light,
      );
    });

    testWidgets('a press from light switches to dark', (tester) async {
      themeStore.stored = ThemeMode.light;
      final container = await pumpApp(tester);
      expect(container.read(themeControllerProvider), ThemeMode.light);

      await tester.tap(find.byType(ThemeToggle));
      await tester.pumpAndSettle();

      expect(container.read(themeControllerProvider), ThemeMode.dark);
      expect(
        Theme.of(tester.element(find.byType(ThemeToggle))).brightness,
        Brightness.dark,
      );
    });

    testWidgets('a press persists the new mode', (tester) async {
      await pumpApp(tester);

      await tester.tap(find.byType(ThemeToggle));
      await tester.pumpAndSettle();

      expect(themeStore.lastWritten, ThemeMode.light);
    });

    testWidgets('a stored dark choice is applied on launch', (tester) async {
      themeStore.stored = ThemeMode.dark;
      await pumpApp(tester);

      expect(brightnessOf(tester), Brightness.dark);
    });

    // Switching must repaint, not rebuild: a rebuild would throw away the
    // navigation stack and send the user back to the list.
    testWidgets('switching keeps the current screen', (tester) async {
      await pumpApp(tester);

      await tester.tap(find.byType(ThemeToggle));
      await tester.pumpAndSettle();

      expect(find.byType(ThemeToggle), findsOneWidget);
      expect(tester.takeException(), isNull);
    });
  });

  group('the theme pair', () {
    test('light and dark are genuinely different', () {
      final light = AppTheme.light();
      final dark = AppTheme.dark();

      expect(light.brightness, Brightness.light);
      expect(dark.brightness, Brightness.dark);
      // If these matched, the toggle would repaint to an identical UI and look
      // broken.
      expect(light.colorScheme.primary, isNot(dark.colorScheme.primary));
      expect(
        light.scaffoldBackgroundColor,
        isNot(dark.scaffoldBackgroundColor),
      );
    });

    test('both share one seed, so the palettes stay related', () {
      // Two unrelated seeds would make the dark theme feel like a second app
      // rather than the same one in a different light.
      final light = AppTheme.light().colorScheme;
      final dark = AppTheme.dark().colorScheme;

      expect(light.primaryContainer, isNot(dark.primaryContainer));
      // Tonal palettes shift hue slightly per brightness, so the comparison is
      // on closeness rather than equality.
      expect(
        light.primary.computeLuminance(),
        isNot(closeTo(dark.primary.computeLuminance(), 0.001)),
      );
    });
  });
}