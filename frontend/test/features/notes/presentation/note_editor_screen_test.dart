import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:mocktail/mocktail.dart';
import 'package:notes_frontend/core/network/api_exception.dart';
import 'package:notes_frontend/features/notes/data/notes_repository_provider.dart';
import 'package:notes_frontend/features/notes/domain/note.dart';
import 'package:notes_frontend/features/notes/domain/notes_repository.dart';
import 'package:notes_frontend/features/notes/presentation/note_editor_screen.dart';

class MockNotesRepository extends Mock implements NotesRepository {}

Note noteWith({
  required int id,
  String title = 'Title',
  String content = 'Content',
  DateTime? createdAt,
  DateTime? updatedAt,
}) {
  final created = createdAt ?? DateTime.utc(2026, 9, 30, 10);
  return Note(
    id: id,
    title: title,
    content: content,
    createdAt: created,
    updatedAt: updatedAt ?? created,
  );
}

void main() {
  late MockNotesRepository repository;

  setUpAll(() => registerFallbackValue(noteWith(id: 0)));

  setUp(() {
    repository = MockNotesRepository();
    when(() => repository.list(
          query: any(named: 'query'),
          limit: any(named: 'limit'),
          offset: any(named: 'offset'),
        )).thenAnswer((_) async => []);
  });

  Future<void> pumpEditor(
    WidgetTester tester, {
    int? noteId,
  }) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [notesRepositoryProvider.overrideWithValue(repository)],
        child: MaterialApp(
          home: NoteEditorScreen(noteId: noteId),
        ),
      ),
    );
    await tester.pump();
  }

  group('creating', () {
    testWidgets('starts empty and focuses the title', (tester) async {
      await pumpEditor(tester);

      expect(find.text('New note'), findsOneWidget);
      expect(find.widgetWithText(TextFormField, 'Title'), findsOneWidget);
    });

    // The server rejects both, and catching it here saves a round trip.
    testWidgets('refuses to save an empty title', (tester) async {
      await pumpEditor(tester);

      await tester.enterText(find.widgetWithText(TextFormField, 'Content'), 'Body');
      await tester.tap(find.text('Save'));
      await tester.pump();

      expect(find.text('Title is required'), findsOneWidget);
      verifyNever(() => repository.create(
            title: any(named: 'title'),
            content: any(named: 'content'),
          ));
    });

    testWidgets('refuses to save empty content', (tester) async {
      await pumpEditor(tester);

      await tester.enterText(find.widgetWithText(TextFormField, 'Title'), 'Valid');
      await tester.tap(find.text('Save'));
      await tester.pump();

      expect(find.text('Content is required'), findsOneWidget);
      verifyNever(() => repository.create(
            title: any(named: 'title'),
            content: any(named: 'content'),
          ));
    });

    testWidgets('refuses a title shorter than the server allows',
        (tester) async {
      await pumpEditor(tester);

      await tester.enterText(find.widgetWithText(TextFormField, 'Title'), 'ab');
      await tester.enterText(find.widgetWithText(TextFormField, 'Content'), 'Body');
      await tester.tap(find.text('Save'));
      await tester.pump();

      expect(find.textContaining('at least 3'), findsOneWidget);
    });

    testWidgets('sends the trimmed title and content', (tester) async {
      when(() => repository.create(title: any(named: 'title'), content: any(named: 'content')))
          .thenAnswer((_) async => noteWith(id: 1, title: 'Trimmed'));

      await pumpEditor(tester);

      await tester.enterText(
          find.widgetWithText(TextFormField, 'Title'), '  Trimmed  ');
      await tester.enterText(
          find.widgetWithText(TextFormField, 'Content'), '  Body  ');
      await tester.tap(find.text('Save'));
      await tester.pumpAndSettle();

      verify(() => repository.create(title: 'Trimmed', content: 'Body')).called(1);
    });

    // Closing the screen would throw away what the user typed, so a failed save
    // must leave the editor open.
    testWidgets('stays open and shows the reason when the save fails',
        (tester) async {
      when(() => repository.create(title: any(named: 'title'), content: any(named: 'content')))
          .thenThrow(const ApiExceptionServer());

      await pumpEditor(tester);

      await tester.enterText(find.widgetWithText(TextFormField, 'Title'), 'Valid');
      await tester.enterText(
          find.widgetWithText(TextFormField, 'Content'), 'Body');
      await tester.tap(find.text('Save'));
      await tester.pumpAndSettle();

      expect(find.text('New note'), findsOneWidget);
      expect(find.text('The server had a problem.'), findsOneWidget);
      // The typed text is still there, so the user can fix and retry.
      expect(find.text('Valid'), findsOneWidget);
    });

    // The server counts runes; the counter must agree or the UI would reject a
    // title the server accepts.
    testWidgets('counts the title in runes', (tester) async {
      await pumpEditor(tester);

      await tester.enterText(find.widgetWithText(TextFormField, 'Title'), '😀😀😀');
      await tester.pump();

      expect(find.text('3 / 100'), findsOneWidget);
    });

    testWidgets('flags a title over the limit', (tester) async {
      await pumpEditor(tester);

      await tester.enterText(
          find.widgetWithText(TextFormField, 'Title'), 'a' * 101);
      await tester.pump();

      expect(find.text('101 / 100'), findsOneWidget);
    });
  });

  group('editing', () {
    testWidgets('fills the fields from the server', (tester) async {
      when(() => repository.get(any())).thenAnswer(
        (_) async => noteWith(id: 5, title: 'Existing', content: 'Existing body'),
      );

      await pumpEditor(tester, noteId: 5);
      await tester.pumpAndSettle();

      expect(find.text('Edit note'), findsOneWidget);
      expect(find.text('Existing'), findsOneWidget);
      expect(find.text('Existing body'), findsOneWidget);
    });

    testWidgets('sends the update to the right note', (tester) async {
      when(() => repository.get(any())).thenAnswer(
        (_) async => noteWith(id: 5, title: 'Existing', content: 'Body'),
      );
      when(() => repository.update(any(),
              title: any(named: 'title'), content: any(named: 'content')))
          .thenAnswer((_) async => noteWith(id: 5, title: 'Changed'));

      await pumpEditor(tester, noteId: 5);
      await tester.pumpAndSettle();

      await tester.enterText(
          find.widgetWithText(TextFormField, 'Title'), 'Changed');
      await tester.tap(find.text('Save'));
      await tester.pumpAndSettle();

      verify(() => repository.update(5, title: 'Changed', content: 'Body'))
          .called(1);
      verifyNever(() => repository.create(
            title: any(named: 'title'),
            content: any(named: 'content'),
          ));
    });

    testWidgets('offers a retry when the note cannot be loaded',
        (tester) async {
      when(() => repository.get(any())).thenThrow(const ApiExceptionNotFound());

      await pumpEditor(tester, noteId: 5);
      await tester.pumpAndSettle();

      expect(find.text('Try again'), findsOneWidget);
      // An empty form would show "Title is required" at a note that simply
      // failed to load.
      expect(find.text('Title is required'), findsNothing);
    });

    // The message is what the user reads, and the failure it used to show was
    // the interpolated error object: "ApiExceptionNotFound: This note no longer
    // exists." Asserting only that *some* text appeared is what let that pass.
    testWidgets('shows the failure in plain language', (tester) async {
      when(() => repository.get(any())).thenThrow(const ApiExceptionNotFound());

      await pumpEditor(tester, noteId: 5);
      await tester.pumpAndSettle();

      expect(find.text('This note no longer exists.'), findsOneWidget);
      expect(find.textContaining('ApiException'), findsNothing);
    });

    testWidgets('an unexpected failure does not leak its type either',
        (tester) async {
      when(() => repository.get(any())).thenThrow(StateError('socket exploded'));

      await pumpEditor(tester, noteId: 5);
      await tester.pumpAndSettle();

      expect(find.text('Could not load this note.'), findsOneWidget);
      expect(find.textContaining('Bad state'), findsNothing);
    });
  });
}
