import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../data/notes_repository_provider.dart';
import '../domain/note.dart';

/// The note currently open in the editor.
///
/// Fetched separately from the list so opening a note does not depend on the
/// list having loaded, and so the editor has the full content even though the
/// list rows show only a preview.
///
/// autoDispose because a note is only interesting while its screen is on top;
/// keeping it alive would hold the content in memory after the screen is gone.
///
/// Writes go through [NotesListController] instead, so a saved note is already
/// in the list when the editor is popped and no refetch is needed.
final noteControllerProvider = FutureProvider.autoDispose.family<Note, int>((
  ref,
  id,
) {
  return ref.watch(notesRepositoryProvider).get(id);
});
