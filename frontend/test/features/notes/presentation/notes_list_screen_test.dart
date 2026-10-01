import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:mocktail/mocktail.dart';
import 'package:notes_frontend/core/network/api_exception.dart';
import 'package:notes_frontend/core/storage/api_key_store.dart';
import 'package:notes_frontend/features/auth/application/auth_controller.dart';
import 'package:notes_frontend/features/notes/data/notes_repository_provider.dart';
import 'package:notes_frontend/features/notes/domain/note.dart';
import 'package:notes_frontend/features/notes/domain/notes_repository.dart';
import 'package:notes_frontend/features/notes/presentation/note_editor_screen.dart';
import 'package:notes_frontend/features/notes/presentation/notes_list_screen.dart';

class MockNotesRepository extends Mock implements NotesRepository {}

/// A keystore stand-in that reports a signed-in session without touching a
/// platform channel.
class FakeApiKeyStore implements ApiKeyStore {
  FakeApiKeyStore([this.value = 'test-key']);

  String? value;

  @override
  Future<String?> read() async => value;

  @override
  Future<void> write(String apiKey) async => value = apiKey;

  @override
  Future<void> clear() async => value = null;
}

Note noteWith({
  required int id,
  String title = 'Title',
  String content = 'Content',
}) =>
    Note(
      id: id,
      title: title,
      content: content,
      createdAt: DateTime.utc(2026, 9, 30, 10),
      updatedAt: DateTime.utc(2026, 9, 30, 10),
    );

void main() {
  late MockNotesRepository repository;

  setUpAll(() => registerFallbackValue(noteWith(id: 0)));

  setUp(() {
    repository = MockNotesRepository();
  });

  /// The routes the app declares, without the auth redirect.
  ///
  /// The list navigates with `context.push`, so a screen opened outside a router
  /// has no way to reach the editor at all. Building the same route table here
  /// is what lets these tests exercise the real navigation path instead of
  /// quietly testing a second, hand-built one.
  GoRouter buildRouter() {
    return GoRouter(
      initialLocation: '/notes',
      routes: [
        GoRoute(
          path: '/notes',
          builder: (_, _) => const NotesListScreen(),
          routes: [
            GoRoute(
              path: 'new',
              builder: (_, _) => const NoteEditorScreen(),
            ),
            GoRoute(
              path: ':id',
              builder: (_, state) => NoteEditorScreen(
                noteId: int.tryParse(state.pathParameters['id'] ?? ''),
              ),
            ),
          ],
        ),
      ],
    );
  }

  Future<void> pumpList(
    WidgetTester tester, {
    List<Note>? notes,
    Object? error,
  }) async {
    when(() => repository.list(
          query: any(named: 'query'),
          limit: any(named: 'limit'),
          offset: any(named: 'offset'),
        )).thenAnswer((_) async {
      if (error != null) throw error;
      return notes ?? [];
    });

    final router = buildRouter();
    addTearDown(router.dispose);

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          notesRepositoryProvider.overrideWithValue(repository),
          apiKeyStoreProvider.overrideWithValue(FakeApiKeyStore()),
        ],
        child: MaterialApp.router(routerConfig: router),
      ),
    );
    await tester.pumpAndSettle();
  }

  testWidgets('shows a spinner while the first load is running', (tester) async {
    var completer = Completer<List<Note>>();
    when(() => repository.list(
          query: any(named: 'query'),
          limit: any(named: 'limit'),
          offset: any(named: 'offset'),
        )).thenAnswer((_) => completer.future);

    await tester.pumpWidget(
      ProviderScope(
        overrides: [notesRepositoryProvider.overrideWithValue(repository)],
        child: const MaterialApp(home: NotesListScreen()),
      ),
    );
    await tester.pump();

    expect(find.byType(CircularProgressIndicator), findsWidgets);

    completer.complete([noteWith(id: 1)]);
    await tester.pumpAndSettle();
  });

  testWidgets('lists the notes it loaded', (tester) async {
    await pumpList(tester, notes: [
      noteWith(id: 1, title: 'First note'),
      noteWith(id: 2, title: 'Second note'),
    ]);

    expect(find.text('First note'), findsOneWidget);
    expect(find.text('Second note'), findsOneWidget);
  });

  // "No notes" and "could not load" must not look alike: one invites creating a
  // note, the other needs a retry.
  testWidgets('invites a first note when the list is empty', (tester) async {
    await pumpList(tester, notes: []);

    expect(find.text('No notes yet'), findsOneWidget);
    expect(find.text('Try again'), findsNothing);
  });

  testWidgets('offers a retry when the load failed', (tester) async {
    await pumpList(tester, error: const ApiExceptionNetwork());

    expect(find.text('Try again'), findsOneWidget);
    expect(find.text('No notes yet'), findsNothing);
  });

  testWidgets('surfaces the failure message', (tester) async {
    await pumpList(tester, error: const ApiExceptionNetwork());

    expect(
      find.text('Cannot reach the server. Check that it is running and reachable.'),
      findsOneWidget,
    );
  });

  testWidgets('a retry re-requests the list', (tester) async {
    var attempts = 0;
    when(() => repository.list(
          query: any(named: 'query'),
          limit: any(named: 'limit'),
          offset: any(named: 'offset'),
        )).thenAnswer((_) async {
      attempts++;
      if (attempts == 1) throw const ApiExceptionNetwork();
      return [noteWith(id: 1, title: 'Recovered')];
    });

    await tester.pumpWidget(
      ProviderScope(
        overrides: [notesRepositoryProvider.overrideWithValue(repository)],
        child: const MaterialApp(home: NotesListScreen()),
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.text('Try again'));
    await tester.pumpAndSettle();

    expect(find.text('Recovered'), findsOneWidget);
  });

  testWidgets('searching asks the server for the term', (tester) async {
    await pumpList(tester);

    await tester.enterText(find.widgetWithText(TextField, 'Search notes'), 'needle');
    // The debounce must elapse before a request is sent.
    await tester.pump(const Duration(milliseconds: 500));
    await tester.pumpAndSettle();

    verify(() => repository.list(
          query: 'needle',
          limit: any(named: 'limit'),
          offset: 0,
        )).called(1);
  });

  // Typing five letters must not spend five of the server's rate-limit budget.
  testWidgets('debounces search so typing does not spam the server',
      (tester) async {
    await pumpList(tester);

    await tester.enterText(find.widgetWithText(TextField, 'Search notes'), 'a');
    await tester.pump(const Duration(milliseconds: 100));
    await tester.enterText(find.widgetWithText(TextField, 'Search notes'), 'ab');
    await tester.pump(const Duration(milliseconds: 100));
    await tester.enterText(find.widgetWithText(TextField, 'Search notes'), 'abc');
    await tester.pump(const Duration(milliseconds: 500));
    await tester.pumpAndSettle();

    verify(() => repository.list(
          query: 'abc',
          limit: any(named: 'limit'),
          offset: 0,
        )).called(1);
    verifyNever(() => repository.list(
          query: 'a',
          limit: any(named: 'limit'),
          offset: 0,
        ));
  });

  testWidgets('asks before deleting, and deletes on confirmation',
      (tester) async {
    when(() => repository.delete(any())).thenAnswer((_) async {});
    await pumpList(tester, notes: [noteWith(id: 1, title: 'Doomed')]);

    await tester.tap(find.byIcon(Icons.delete_outline));
    await tester.pumpAndSettle();

    expect(find.text('Delete this note?'), findsOneWidget);

    await tester.tap(find.text('Delete'));
    await tester.pumpAndSettle();

    verify(() => repository.delete(1)).called(1);
    expect(find.text('Doomed'), findsNothing);
  });

  // A confirmation that deletes nothing would train the user to dismiss it.
  testWidgets('cancelling the confirmation deletes nothing', (tester) async {
    await pumpList(tester, notes: [noteWith(id: 1, title: 'Kept')]);

    await tester.tap(find.byIcon(Icons.delete_outline));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Cancel'));
    await tester.pumpAndSettle();

    verifyNever(() => repository.delete(any()));
    expect(find.text('Kept'), findsOneWidget);
  });

  testWidgets('opens an empty editor from the action button', (tester) async {
    await pumpList(tester);

    await tester.tap(find.text('New note'));
    await tester.pumpAndSettle();

    // A null id means composing, not editing, so the title says so.
    expect(find.text('New note'), findsOneWidget);
    expect(find.widgetWithText(TextFormField, 'Title'), findsOneWidget);
  });

  testWidgets('opens an existing note for editing', (tester) async {
    when(() => repository.get(any())).thenAnswer(
      (_) async => noteWith(id: 1, title: 'Editable', content: 'Body'),
    );
    await pumpList(tester, notes: [noteWith(id: 1, title: 'Editable')]);

    await tester.tap(find.text('Editable'));
    await tester.pumpAndSettle();

    expect(find.text('Edit note'), findsOneWidget);
    expect(find.text('Editable'), findsWidgets);
  });

  testWidgets('signs out and forgets the key', (tester) async {
    final store = FakeApiKeyStore();
    when(() => repository.list(
          query: any(named: 'query'),
          limit: any(named: 'limit'),
          offset: any(named: 'offset'),
        )).thenAnswer((_) async => []);

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          notesRepositoryProvider.overrideWithValue(repository),
          apiKeyStoreProvider.overrideWithValue(store),
        ],
        child: const MaterialApp(home: NotesListScreen()),
      ),
    );
    await tester.pumpAndSettle();

    // Found by tooltip rather than by icon: the glyph is a cosmetic detail, and
    // the test should break only if the action stops being reachable.
    await tester.tap(find.byTooltip('Sign out'));
    await tester.pumpAndSettle();

    expect(store.value, isNull);
  });
}
