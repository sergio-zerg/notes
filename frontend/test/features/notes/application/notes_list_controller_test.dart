import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:mocktail/mocktail.dart';
import 'package:notes_frontend/core/network/api_exception.dart';
import 'package:notes_frontend/features/notes/application/notes_list_controller.dart';
import 'package:notes_frontend/features/notes/data/notes_repository_provider.dart';
import 'package:notes_frontend/features/notes/domain/note.dart';
import 'package:notes_frontend/features/notes/domain/notes_repository.dart';

class MockNotesRepository extends Mock implements NotesRepository {}

/// Builds a note with only the fields a test cares about.
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

  setUpAll(() {
    // registerFallbackValue covers the positional argument the repository takes
    // on delete, which mocktail cannot infer from the call site.
    registerFallbackValue(noteWith(id: 0));
  });

  setUp(() {
    repository = MockNotesRepository();
  });

  /// Builds a container whose repository provider returns the mock, and lets a
  /// test read the controller through it.
  ProviderContainer containerWith(NotesRepository repo) {
    final container = ProviderContainer(
      overrides: [notesRepositoryProvider.overrideWithValue(repo)],
    );
    addTearDown(container.dispose);
    return container;
  }

  test('loads the first page when the controller is built', () async {
    when(() => repository.list(query: any(named: 'query'), limit: any(named: 'limit'), offset: any(named: 'offset')))
        .thenAnswer((_) async => [noteWith(id: 1), noteWith(id: 2)]);

    final container = containerWith(repository);
    await container.read(notesListControllerProvider.notifier).load();

    final state = container.read(notesListControllerProvider);
    expect(state.notes.map((note) => note.id), [1, 2]);
    expect(state.isLoading, isFalse);
    expect(state.error, isNull);
  });

  test('reports an error and stops loading when the first load fails', () async {
    when(() => repository.list(query: any(named: 'query'), limit: any(named: 'limit'), offset: any(named: 'offset')))
        .thenThrow(const ApiExceptionNetwork());

    final container = containerWith(repository);
    await container.read(notesListControllerProvider.notifier).load();

    final state = container.read(notesListControllerProvider);
    expect(state.isLoading, isFalse);
    expect(state.error, isNotNull);
    // A failed load must not masquerade as "no notes": the screen offers a
    // retry rather than inviting the user to create a note.
    expect(state.isEmpty, isFalse);
  });

  test('an empty result is genuinely empty', () async {
    when(() => repository.list(query: any(named: 'query'), limit: any(named: 'limit'), offset: any(named: 'offset')))
        .thenAnswer((_) async => []);

    final container = containerWith(repository);
    await container.read(notesListControllerProvider.notifier).load();

    expect(container.read(notesListControllerProvider).isEmpty, isTrue);
  });

  group('paging', () {
    test('appends the next page instead of replacing the list', () async {
      var call = 0;
      when(() => repository.list(query: any(named: 'query'), limit: any(named: 'limit'), offset: any(named: 'offset')))
          .thenAnswer((_) async {
        call++;
        // A full page is what signals "there may be more", since the endpoint
        // reports no total count.
        if (call == 1) return List.generate(50, (i) => noteWith(id: i + 1));
        return [noteWith(id: 51)];
      });

      final container = containerWith(repository);
      final controller = container.read(notesListControllerProvider.notifier);

      await controller.load();
      await controller.loadMore();

      final state = container.read(notesListControllerProvider);
      expect(state.notes, hasLength(51));
      expect(state.notes.first.id, 1);
      expect(state.notes.last.id, 51);
    });

    // The second page must ask for the offset the first one ended at, or it
    // would return the same rows again.
    test('requests the next offset', () async {
      final offsets = <int>[];
      when(() => repository.list(query: any(named: 'query'), limit: any(named: 'limit'), offset: any(named: 'offset')))
          .thenAnswer((invocation) async {
        final offset = invocation.namedArguments[#offset] as int;
        offsets.add(offset);
        if (offset == 0) return List.generate(50, (i) => noteWith(id: i + 1));
        return [];
      });

      final container = containerWith(repository);
      final controller = container.read(notesListControllerProvider.notifier);

      await controller.load();
      await controller.loadMore();

      expect(offsets, [0, 50]);
    });

    // The endpoint reports no total count, so a short page is the only
    // end-of-list signal there is.
    test('stops paging after a short page', () async {
      var calls = 0;
      when(() => repository.list(query: any(named: 'query'), limit: any(named: 'limit'), offset: any(named: 'offset')))
          .thenAnswer((_) async {
        calls++;
        // Three rows is a partial page, so there is nothing more to fetch.
        return [noteWith(id: 1), noteWith(id: 2), noteWith(id: 3)];
      });

      final container = containerWith(repository);
      final controller = container.read(notesListControllerProvider.notifier);

      await controller.load();
      expect(container.read(notesListControllerProvider).hasMore, isFalse);

      await controller.loadMore();
      expect(calls, 1);
    });

    // A scroll gesture emits many events; each must not start a request.
    test('ignores a second loadMore while one is in flight', () async {
      when(() => repository.list(query: any(named: 'query'), limit: any(named: 'limit'), offset: any(named: 'offset')))
          .thenAnswer((_) async => List.generate(50, (i) => noteWith(id: i + 1)));

      final container = containerWith(repository);
      final controller = container.read(notesListControllerProvider.notifier);

      await controller.load();
      await Future.wait([controller.loadMore(), controller.loadMore()]);

      // One initial load plus exactly one page fetch: the second loadMore must
      // see isLoadingMore and return without asking the server again.
      verify(() => repository.list(
            query: any(named: 'query'),
            limit: any(named: 'limit'),
            offset: any(named: 'offset'),
          )).called(2);
    });
  });

  group('search', () {
    test('stores the term and reloads from the first page', () async {
      when(() => repository.list(query: any(named: 'query'), limit: any(named: 'limit'), offset: any(named: 'offset')))
          .thenAnswer((_) async => [noteWith(id: 1)]);

      final container = containerWith(repository);
      await container.read(notesListControllerProvider.notifier).search('needle');

      expect(container.read(searchQueryProvider), 'needle');
      verify(() => repository.list(
            query: 'needle',
            limit: any(named: 'limit'),
            offset: 0,
          )).called(1);
    });
  });

  group('create', () {
    // The list mirrors the server's ascending-id order rather than showing the
    // new note at the top. Paging is by offset over that same order, so a list
    // reordered locally no longer describes the window the next request reads:
    // the note comes back a second time in the following page.
    test('places the created note where the server would', () async {
      when(() => repository.list(query: any(named: 'query'), limit: any(named: 'limit'), offset: any(named: 'offset')))
          .thenAnswer((_) async => [noteWith(id: 1), noteWith(id: 2)]);
      when(() => repository.create(title: any(named: 'title'), content: any(named: 'content')))
          .thenAnswer((_) async => noteWith(id: 9, title: 'Fresh'));

      final container = containerWith(repository);
      final controller = container.read(notesListControllerProvider.notifier);

      await controller.load();
      final saved = await controller.create(title: 'Fresh', content: 'Body');

      expect(saved, isTrue);
      expect(container.read(notesListControllerProvider).notes.map((n) => n.id), [1, 2, 9]);
    });

    test('a note created before an open window is not shown twice', () async {
      // A note created while page one is on screen takes the slot at the cursor,
      // so the next page returns it again. Deduplicating on append is what keeps
      // it to a single row.
      when(() => repository.list(query: any(named: 'query'), limit: any(named: 'limit'), offset: any(named: 'offset')))
          .thenAnswer((invocation) async {
        final offset = invocation.namedArguments[#offset] as int;
        if (offset == 0) return List.generate(50, (i) => noteWith(id: i + 1));
        return [noteWith(id: offset + 1)];
      });
      when(() => repository.create(title: any(named: 'title'), content: any(named: 'content')))
          .thenAnswer((_) async => noteWith(id: 51, title: 'Fresh'));

      final container = containerWith(repository);
      final controller = container.read(notesListControllerProvider.notifier);

      await controller.load();
      await controller.create(title: 'Fresh', content: 'Body');
      expect(container.read(notesListControllerProvider).notes.last.id, 51);

      await controller.loadMore();

      final state = container.read(notesListControllerProvider);
      expect(state.notes, hasLength(51));
      expect(state.notes.map((n) => n.id).toSet(), hasLength(51));
    });

    test('reports failure and leaves the list untouched', () async {
      when(() => repository.list(query: any(named: 'query'), limit: any(named: 'limit'), offset: any(named: 'offset')))
          .thenAnswer((_) async => [noteWith(id: 1)]);
      when(() => repository.create(title: any(named: 'title'), content: any(named: 'content')))
          .thenThrow(const ApiExceptionValidation('title must be at least 3 characters'));

      final container = containerWith(repository);
      final controller = container.read(notesListControllerProvider.notifier);

      await controller.load();
      final saved = await controller.create(title: 'ab', content: 'Body');

      final state = container.read(notesListControllerProvider);
      expect(saved, isFalse);
      expect(state.error, contains('at least 3'));
      expect(state.notes.map((n) => n.id), [1]);
      expect(state.isSaving, isFalse);
    });
  });

  group('update', () {
    test('replaces the note in place', () async {
      when(() => repository.list(query: any(named: 'query'), limit: any(named: 'limit'), offset: any(named: 'offset')))
          .thenAnswer((_) async => [
                noteWith(id: 1, title: 'Old'),
                noteWith(id: 2, title: 'Other'),
              ]);
      when(() => repository.update(any(), title: any(named: 'title'), content: any(named: 'content')))
          .thenAnswer((_) async => noteWith(id: 1, title: 'New'));

      final container = containerWith(repository);
      final controller = container.read(notesListControllerProvider.notifier);

      await controller.load();
      final saved = await controller.update(1, title: 'New', content: 'Body');

      expect(saved, isTrue);
      expect(
        container.read(notesListControllerProvider).notes.map((n) => n.title),
        ['New', 'Other'],
      );
    });

    test('a failed update keeps the old note in the list', () async {
      when(() => repository.list(query: any(named: 'query'), limit: any(named: 'limit'), offset: any(named: 'offset')))
          .thenAnswer((_) async => [noteWith(id: 1, title: 'Old')]);
      when(() => repository.update(any(), title: any(named: 'title'), content: any(named: 'content')))
          .thenThrow(const ApiExceptionServer());

      final container = containerWith(repository);
      final controller = container.read(notesListControllerProvider.notifier);

      await controller.load();
      final saved = await controller.update(1, title: 'New', content: 'Body');

      expect(saved, isFalse);
      expect(container.read(notesListControllerProvider).notes.single.title, 'Old');
    });

    // Regression: the replacement rebuilt the list from ids it already had, so
    // a note that was not in the list was dropped on a successful save. The
    // write landed on the server and the screen kept disagreeing with it.
    test('adds a note that was not in the list', () async {
      when(() => repository.list(query: any(named: 'query'), limit: any(named: 'limit'), offset: any(named: 'offset')))
          .thenAnswer((_) async => [noteWith(id: 1), noteWith(id: 5)]);
      when(() => repository.update(any(), title: any(named: 'title'), content: any(named: 'content')))
          .thenAnswer((_) async => noteWith(id: 3, title: 'Edited elsewhere'));

      final container = containerWith(repository);
      final controller = container.read(notesListControllerProvider.notifier);

      await controller.load();
      final saved = await controller.update(3, title: 'Edited elsewhere', content: 'Body');

      expect(saved, isTrue);
      // Inserted in id order, so it still lines up with the server's offsets.
      expect(container.read(notesListControllerProvider).notes.map((n) => n.id), [1, 3, 5]);
    });
  });

  group('delete', () {
    test('removes the note from the list', () async {
      when(() => repository.list(query: any(named: 'query'), limit: any(named: 'limit'), offset: any(named: 'offset')))
          .thenAnswer((_) async => [noteWith(id: 1), noteWith(id: 2)]);
      when(() => repository.delete(any())).thenAnswer((_) async {});

      final container = containerWith(repository);
      final controller = container.read(notesListControllerProvider.notifier);

      await controller.load();
      final deleted = await controller.delete(1);

      expect(deleted, isTrue);
      expect(container.read(notesListControllerProvider).notes.map((n) => n.id), [2]);
    });

    test('a failed delete keeps the note and reports the error', () async {
      when(() => repository.list(query: any(named: 'query'), limit: any(named: 'limit'), offset: any(named: 'offset')))
          .thenAnswer((_) async => [noteWith(id: 1)]);
      when(() => repository.delete(any()))
          .thenThrow(const ApiExceptionNetwork());

      final container = containerWith(repository);
      final controller = container.read(notesListControllerProvider.notifier);

      await controller.load();
      final deleted = await controller.delete(1);

      expect(deleted, isFalse);
      expect(container.read(notesListControllerProvider).notes.single.id, 1);
    });

    // Removing a row shifts every later server offset down by one. Keeping the
    // old cursor asked for one row too far and skipped the note after the one
    // deleted.
    test('moves the page cursor back when a loaded note goes', () async {
      final offsets = <int>[];
      when(() => repository.list(query: any(named: 'query'), limit: any(named: 'limit'), offset: any(named: 'offset')))
          .thenAnswer((invocation) async {
        final offset = invocation.namedArguments[#offset] as int;
        offsets.add(offset);
        if (offset < 50) return List.generate(50, (i) => noteWith(id: i + 1));
        return [noteWith(id: offset + 1)];
      });
      when(() => repository.delete(any())).thenAnswer((_) async {});

      final container = containerWith(repository);
      final controller = container.read(notesListControllerProvider.notifier);

      await controller.load();
      expect(container.read(notesListControllerProvider).nextOffset, 50);

      await controller.delete(10);
      expect(container.read(notesListControllerProvider).nextOffset, 49);

      await controller.loadMore();
      expect(offsets, [0, 49]);
    });

    // A note outside the loaded window is at or after the cursor, so nothing
    // the client can see has shifted.
    test('leaves the cursor alone for a note outside the window', () async {
      when(() => repository.list(query: any(named: 'query'), limit: any(named: 'limit'), offset: any(named: 'offset')))
          .thenAnswer((_) async => List.generate(50, (i) => noteWith(id: i + 1)));
      when(() => repository.delete(any())).thenAnswer((_) async {});

      final container = containerWith(repository);
      final controller = container.read(notesListControllerProvider.notifier);

      await controller.load();
      await controller.delete(999);

      expect(container.read(notesListControllerProvider).nextOffset, 50);
    });
  });

  test('clearError resets the message once it has been shown', () async {
    when(() => repository.list(query: any(named: 'query'), limit: any(named: 'limit'), offset: any(named: 'offset')))
        .thenThrow(const ApiExceptionNetwork());

    final container = containerWith(repository);
    final controller = container.read(notesListControllerProvider.notifier);

    await controller.load();
    expect(container.read(notesListControllerProvider).error, isNotNull);

    controller.clearError();
    expect(container.read(notesListControllerProvider).error, isNull);
  });

  test('an ApiException carries a message the screen can show', () async {
    when(() => repository.list(query: any(named: 'query'), limit: any(named: 'limit'), offset: any(named: 'offset')))
        .thenThrow(const ApiExceptionRateLimited(retryAfterSeconds: 2));

    final container = containerWith(repository);
    await container.read(notesListControllerProvider.notifier).load();

    final error = container.read(notesListControllerProvider).error;
    expect(error, isNotNull);
    expect(error, isNotEmpty);
    // The repository contract is that it throws ApiException, never a transport
    // type — which is what keeps dio out of the presentation layer.
    expect(error, isNot(contains('DioException')));
  });
}
