import 'package:dio/dio.dart';

import '../../../core/network/api_client.dart';
import '../domain/note.dart';
import '../domain/notes_repository.dart';
import 'note_model.dart';

/// [NotesRepository] backed by the REST API.
class NotesApiRepository implements NotesRepository {
  NotesApiRepository(this._dio);

  final Dio _dio;

  @override
  Future<List<Note>> list({
    String query = '',
    int limit = defaultPageSize,
    int offset = 0,
  }) async {
    final response = await guardRequest<List<dynamic>>(
      () => _dio.get<List<dynamic>>(
        '/notes',
        // Omitting the empty values keeps the request URL the same as a plain
        // list, and lets the server apply its own defaults rather than having
        // the client second-guess them.
        queryParameters: {
          if (query.trim().isNotEmpty) 'q': query.trim(),
          'limit': limit,
          'offset': offset,
        },
      ),
      parse: (data) => data ?? const [],
    );

    return response
        .whereType<Map<String, dynamic>>()
        .map(NoteModel.fromJson)
        .where((model) => model.id > 0)
        .map((model) => model.toEntity())
        .toList(growable: false);
  }

  @override
  Future<Note> get(int id) async {
    final model = await guardRequest<NoteModel>(
      () => _dio.get<Map<String, dynamic>>('/notes/$id'),
      parse: (data) => NoteModel.fromJson(_asJson(data)),
    );
    return model.toEntity();
  }

  @override
  Future<Note> create({required String title, required String content}) async {
    final model = await guardRequest<NoteModel>(
      () => _dio.post<Map<String, dynamic>>(
        '/notes',
        data: {'title': title, 'content': content},
      ),
      parse: (data) => NoteModel.fromJson(_asJson(data)),
    );
    return model.toEntity();
  }

  @override
  Future<Note> update(
    int id, {
    required String title,
    required String content,
  }) async {
    final model = await guardRequest<NoteModel>(
      () => _dio.put<Map<String, dynamic>>(
        '/notes/$id',
        data: {'title': title, 'content': content},
      ),
      parse: (data) => NoteModel.fromJson(_asJson(data)),
    );
    return model.toEntity();
  }

  @override
  Future<void> delete(int id) async {
    // 204 No Content: the body is empty by definition, so there is nothing to
    // read and nothing to check.
    await guardVoidRequest(() => _dio.delete<void>('/notes/$id'));
  }
}

/// Asserts that a response body is a JSON object before it is read as one.
///
/// A `200` carrying a bare array or a string would otherwise throw a cast
/// error from deep inside the model, far from the request that produced it.
Map<String, dynamic> _asJson(dynamic data) {
  if (data is Map<String, dynamic>) {
    return data;
  }
  throw const FormatException('Expected a JSON object from the notes API');
}
