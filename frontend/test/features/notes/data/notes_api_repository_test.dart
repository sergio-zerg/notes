import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http_mock_adapter/http_mock_adapter.dart';
import 'package:notes_frontend/core/network/api_client.dart';
import 'package:notes_frontend/core/network/api_exception.dart';
import 'package:notes_frontend/features/notes/data/notes_api_repository.dart';

/// A note as the server serialises it.
Map<String, dynamic> noteJson({
  required int id,
  required String title,
  required String content,
  String createdAt = '2026-09-30T10:00:00Z',
  String updatedAt = '2026-09-30T10:00:00Z',
}) =>
    {
      'id': id,
      'title': title,
      'content': content,
      'created_at': createdAt,
      'updated_at': updatedAt,
    };

void main() {
  late DioAdapter adapter;
  late NotesApiRepository repository;
  late Dio dio;

  setUp(() {
    dio = createApiClient(
      baseUrl: 'http://notes.test',
      apiKeyProvider: () async => 'test-key',
    );
    adapter = DioAdapter(dio: dio);
    repository = NotesApiRepository(dio);
  });

  group('list', () {
    test('parses the notes the server returned', () async {
      adapter.onGet(
        '/notes',
        (server) => server.reply(200, [
          noteJson(id: 1, title: 'First', content: 'Body one'),
        ]),
        queryParameters: {'limit': 50, 'offset': 0},
      );

      final notes = await repository.list();

      expect(notes, hasLength(1));
      expect(notes.single.title, 'First');
      expect(notes.single.content, 'Body one');
      // Timestamps are normalised to UTC so comparisons do not depend on the
      // device time zone.
      expect(notes.single.createdAt.isUtc, isTrue);
      expect(notes.single.createdAt, DateTime.utc(2026, 9, 30, 10));
    });

    // The key must be on every request: the server rejects the call without it,
    // and its absence would surface as a confusing 401 rather than a local bug.
    test('attaches the API key to the request', () async {
      adapter.onGet(
        '/notes',
        (server) => server.reply(200, []),
        headers: {'X-API-Key': 'test-key'},
      );

      // The adapter only matches when the header is present, so an unmatched
      // request throws rather than silently returning an empty list.
      await expectLater(repository.list(), completes);
    });

    test('sends the search term as q', () async {
      adapter.onGet(
        '/notes',
        (server) => server.reply(200, []),
        queryParameters: {'q': 'needle', 'limit': 50, 'offset': 0},
      );

      await expectLater(repository.list(query: 'needle'), completes);
    });

    // A blank search must not become `q=`, or the server would filter on a
    // whitespace string and return nothing.
    test('omits q when the search is blank', () async {
      adapter.onGet(
        '/notes',
        (server) => server.reply(200, []),
        queryParameters: {'limit': 50, 'offset': 0},
      );

      await expectLater(repository.list(query: '   '), completes);
    });

    test('sends limit and offset for paging', () async {
      adapter.onGet(
        '/notes',
        (server) => server.reply(200, []),
        queryParameters: {'limit': 50, 'offset': 100},
      );

      await expectLater(repository.list(limit: 50, offset: 100), completes);
    });

    // One malformed row must not empty the whole page.
    test('skips entries without a usable id', () async {
      adapter.onGet(
        '/notes',
        (server) => server.reply(200, [
          noteJson(id: 0, title: 'Bad', content: 'x'),
          noteJson(id: 2, title: 'Good', content: 'y'),
        ]),
        queryParameters: {'limit': 50, 'offset': 0},
      );

      final notes = await repository.list();

      expect(notes.map((note) => note.title), ['Good']);
    });

    test('an empty result yields an empty list', () async {
      adapter.onGet(
        '/notes',
        (server) => server.reply(200, []),
        queryParameters: {'limit': 50, 'offset': 0},
      );

      expect(await repository.list(), isEmpty);
    });

    test('translates a 401 into an unauthorized failure', () async {
      adapter.onGet(
        '/notes',
        (server) => server.reply(401, {'error': 'valid API key required'}),
      );

      await expectLater(
        repository.list(),
        throwsA(isA<ApiExceptionUnauthorized>()),
      );
    });
  });

  group('create', () {
    test('posts the fields and returns the stored note', () async {
      adapter.onPost(
        '/notes',
        (server) => server.reply(
          201,
          noteJson(id: 7, title: 'New', content: 'Body',
              updatedAt: '2026-09-30T11:00:00Z'),
        ),
        data: {'title': 'New', 'content': 'Body'},
      );

      final note = await repository.create(title: 'New', content: 'Body');

      expect(note.id, 7);
      expect(note.title, 'New');
      // The server assigns both timestamps, so the client must read them rather
      // than inventing its own.
      expect(note.updatedAt, DateTime.utc(2026, 9, 30, 11));
    });

    // The backend writes domain rejections as text/plain, and that wording is
    // the only thing that explains which rule was broken.
    test('surfaces the server\'s validation message', () async {
      adapter.onPost(
        '/notes',
        (server) => server.reply(
          400,
          'invalid input provided: title must be at least 3 characters',
          headers: {
            Headers.contentTypeHeader: ['text/plain; charset=utf-8'],
          },
        ),
        data: {'title': 'ab', 'content': 'Body'},
      );

      await expectLater(
        repository.create(title: 'ab', content: 'Body'),
        throwsA(
          isA<ApiExceptionValidation>()
              .having((e) => e.message, 'message', contains('at least 3')),
        ),
      );
    });

    test('translates a 429 and keeps Retry-After', () async {
      adapter.onPost(
        '/notes',
        (server) => server.reply(
          429,
          {'error': 'rate limit exceeded'},
          headers: {
            Headers.contentTypeHeader: [Headers.jsonContentType],
            'retry-after': ['3'],
          },
        ),
        data: {'title': 'New', 'content': 'Body'},
      );

      await expectLater(
        repository.create(title: 'New', content: 'Body'),
        throwsA(
          isA<ApiExceptionRateLimited>()
              .having((e) => e.retryAfterSeconds, 'retryAfterSeconds', 3),
        ),
      );
    });
  });

  group('get', () {
    test('requests the note by id', () async {
      adapter.onGet(
        '/notes/5',
        (server) => server.reply(200, noteJson(id: 5, title: 'One', content: 'Body')),
      );

      final note = await repository.get(5);

      expect(note.id, 5);
      expect(note.title, 'One');
    });

    test('a missing note becomes a not-found failure', () async {
      adapter.onGet(
        '/notes/999',
        (server) => server.reply(
          404,
          'note not found',
          headers: {
            Headers.contentTypeHeader: ['text/plain; charset=utf-8'],
          },
        ),
      );

      await expectLater(repository.get(999), throwsA(isA<ApiExceptionNotFound>()));
    });
  });

  group('update', () {
    test('puts the replacement fields', () async {
      adapter.onPut(
        '/notes/3',
        (server) => server.reply(
          200,
          noteJson(
            id: 3,
            title: 'Changed',
            content: 'New body',
            updatedAt: '2026-09-30T12:00:00Z',
          ),
        ),
        data: {'title': 'Changed', 'content': 'New body'},
      );

      final note =
          await repository.update(3, title: 'Changed', content: 'New body');

      expect(note.title, 'Changed');
      // The server preserves created_at and advances updated_at, which is what
      // drives the "edited" marker in the list.
      expect(note.isEdited, isTrue);
    });

    test('an unchanged note is not marked as edited', () async {
      adapter.onGet(
        '/notes/4',
        (server) => server.reply(200, noteJson(id: 4, title: 'Same', content: 'Body')),
      );

      expect((await repository.get(4)).isEdited, isFalse);
    });
  });

  group('delete', () {
    test('accepts a 204 with an empty body', () async {
      adapter.onDelete('/notes/4', (server) => server.reply(204, null));

      await expectLater(repository.delete(4), completes);
    });

    test('translates a 404', () async {
      adapter.onDelete(
        '/notes/4',
        (server) => server.reply(
          404,
          'note not found',
          headers: {
            Headers.contentTypeHeader: ['text/plain; charset=utf-8'],
          },
        ),
      );

      await expectLater(repository.delete(4), throwsA(isA<ApiExceptionNotFound>()));
    });
  });

  group('transport failures', () {
    test('a dropped connection becomes a retryable network failure', () async {
      adapter.onGet(
        '/notes',
        (server) => server.throws(
          0,
          DioException.connectionError(
            requestOptions: RequestOptions(path: '/notes'),
            reason: 'connection refused',
          ),
        ),
      );

      await expectLater(
        repository.list(),
        throwsA(
          isA<ApiException>().having((e) => e.isRetryable, 'isRetryable', isTrue),
        ),
      );
    });

    // A 500 body is replaced by generic text on the server so internal detail
    // never leaves it; the client must not display it either.
    test('a 500 never surfaces the body', () async {
      adapter.onGet(
        '/notes',
        (server) => server.reply(
          500,
          'pq: password authentication failed for user "postgres"',
          headers: {
            Headers.contentTypeHeader: ['text/plain; charset=utf-8'],
          },
        ),
      );

      await expectLater(
        repository.list(),
        throwsA(
          isA<ApiExceptionServer>().having(
            (e) => e.message,
            'message',
            isNot(contains('postgres')),
          ),
        ),
      );
    });
  });
}
