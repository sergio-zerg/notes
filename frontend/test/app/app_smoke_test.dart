import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:notes_frontend/app/app.dart';
import 'package:notes_frontend/core/storage/api_key_store.dart';
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
  Future<Note> get(int id) async =>
      notes.firstWhere((note) => note.id == id);

  @override
  Future<Note> create({required String title, required String content}) async {
    final note = Note(
      id: notes.length + 1,
      title: title,
      content: content,
      createdAt: DateTime.utc(2026, 9, 30, 10),
      updatedAt: DateTime.utc(2026, 9, 30, 10),
    );
    notes = [note, ...notes];
    return note;
  }

  @override
  Future<Note> update(
    int id, {
    required String title,
    required String content,
  }) async =>
      notes.firstWhere((note) => note.id == id);

  @override
  Future<void> delete(int id) async =>
      notes = notes.where((note) => note.id != id).toList();
}

/// A keystore stand-in, so the app boots without a platform channel.
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

void main() {
  /// Builds the real app with only the storage and network swapped.
  Future<void> pumpApp(
    WidgetTester tester, {
    FakeApiKeyStore? store,
    FakeNotesRepository? repository,
  }) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          apiKeyStoreProvider.overrideWithValue(store ?? FakeApiKeyStore()),
          notesRepositoryProvider
              .overrideWithValue(repository ?? FakeNotesRepository()),
        ],
        child: const NotesApp(),
      ),
    );
    // Enough pumps for the keystore read, the router redirect and the first
    // list load to settle.
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 100));
    await tester.pumpAndSettle();
  }

  testWidgets('an unauthenticated visitor sees the login screen',
      (tester) async {
    await pumpApp(tester);

    expect(find.text('Sign in with the API key for your server'), findsOneWidget);
    expect(find.widgetWithText(TextFormField, 'API key'), findsOneWidget);
  });

  testWidgets('a stored key lands on the notes list', (tester) async {
    await pumpApp(
      tester,
      store: FakeApiKeyStore('existing-key'),
      repository: FakeNotesRepository([
        Note(
          id: 1,
          title: 'Stored note',
          content: 'Body',
          createdAt: DateTime.utc(2026, 9, 30, 10),
          updatedAt: DateTime.utc(2026, 9, 30, 10),
        ),
      ]),
    );

    expect(find.text('Stored note'), findsOneWidget);
    expect(find.text('1 note'), findsOneWidget);
  });

  // The failure this guards against is a screen that throws during build and
  // leaves a blank page, with the reason visible only in a console nobody reads.
  testWidgets('the whole app builds without throwing', (tester) async {
    await pumpApp(tester, store: FakeApiKeyStore('key'));

    expect(tester.takeException(), isNull);
  });

  testWidgets('the list survives a rotation to a landscape viewport',
      (tester) async {
    tester.view.physicalSize = const Size(844, 390);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);

    await pumpApp(tester, store: FakeApiKeyStore('key'));

    expect(tester.takeException(), isNull);
  });
}
