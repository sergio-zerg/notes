import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/network/dio_provider.dart';
import '../domain/notes_repository.dart';
import 'notes_api_repository.dart';

/// The repository the app uses.
///
/// Overriding this one provider in a test swaps the whole data layer, which is
/// why the controllers depend on the interface rather than on the
/// implementation.
final notesRepositoryProvider = Provider<NotesRepository>((ref) {
  return NotesApiRepository(ref.watch(dioProvider));
});
