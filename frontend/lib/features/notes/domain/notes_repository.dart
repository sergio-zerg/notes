import 'note.dart';

/// The page size a caller gets when it names none.
///
/// Matches [AppConfig.pageSize], which is what the controller asks for. The
/// constant is repeated rather than imported so the domain layer keeps no
/// dependency on the app's configuration; the two drifting apart would only
/// change how many rows one call happens to fetch.
const int defaultPageSize = 50;

/// The contract the presentation layer depends on.
///
/// Declaring it in the domain layer, next to [Note], is what keeps dio and JSON
/// out of the controllers: the UI asks for notes and gets notes, and swapping
/// REST for gRPC would touch only the data layer.
abstract interface class NotesRepository {
  /// Returns one page of notes in the server's order, oldest first.
  ///
  /// [query] filters by a case-insensitive substring of title and content, and
  /// an empty string means no filter. [offset] pages through the result, which
  /// is stable because the server orders by ascending id.
  ///
  /// The order matters to callers: [offset] is a position in that ordering, so a
  /// list assembled from these pages describes a window the next request reads
  /// back. Reordering it locally makes the next offset wrong.
  Future<List<Note>> list({
    String query = '',
    int limit = defaultPageSize,
    int offset = 0,
  });

  /// Fetches a single note.
  Future<Note> get(int id);

  /// Creates a note and returns it as the server stored it, including the
  /// timestamps it assigned.
  Future<Note> create({required String title, required String content});

  /// Replaces the title and content of an existing note.
  Future<Note> update(int id, {required String title, required String content});

  /// Deletes a note. Completes normally when it is already gone, because the
  /// caller's intent — that it should not exist — is satisfied either way.
  Future<void> delete(int id);
}
