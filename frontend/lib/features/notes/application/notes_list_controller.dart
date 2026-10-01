import 'package:flutter/foundation.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/config/app_config.dart';
import '../../../core/network/api_exception.dart';
import '../domain/note.dart';
import '../domain/notes_repository.dart';
import '../data/notes_repository_provider.dart';

/// Search text, held apart from the results so typing does not rebuild the list.
final searchQueryProvider = NotifierProvider<SearchQueryController, String>(
  SearchQueryController.new,
);

class SearchQueryController extends Notifier<String> {
  @override
  String build() => '';

  void update(String value) => state = value;

  void clear() => state = '';
}

/// The notes list, its loading state and its errors.
@immutable
class NotesListState {
  const NotesListState({
    this.notes = const [],
    this.isLoading = true,
    this.isLoadingMore = false,
    this.isSaving = false,
    this.nextOffset = 0,
    this.hasMore = true,
    this.error,
  });

  final List<Note> notes;

  /// True during the first load, when the list has nothing to show yet.
  final bool isLoading;

  /// True while a further page is being fetched, as distinct from a refresh.
  final bool isLoadingMore;

  /// True while a create, update or delete is in flight.
  final bool isSaving;

  /// The offset the next page request should carry.
  ///
  /// This is the server-side position of the first note not yet loaded, and it
  /// is not the same as [notes].length once a note has been created or deleted:
  /// those change the server's ordering underneath the client's page window, so
  /// deriving the offset from the visible length skips or repeats a row.
  final int nextOffset;

  /// Whether another page could exist.
  ///
  /// The list endpoint has no total count, so paging continues while a full page
  /// came back: a short page is the only end-of-list signal available. It is
  /// recorded when the page arrives rather than computed from the list length,
  /// for the same reason [nextOffset] is not.
  final bool hasMore;

  /// Set when the last operation failed; null when the last one succeeded.
  final String? error;

  /// Whether the list is genuinely empty, as opposed to still loading.
  ///
  /// A screen must distinguish these: "no notes" offers to create one, while a
  /// spinner should not be replaced by that message for one frame.
  bool get isEmpty => !isLoading && notes.isEmpty && error == null;

  NotesListState copyWith({
    List<Note>? notes,
    bool? isLoading,
    bool? isLoadingMore,
    bool? isSaving,
    int? nextOffset,
    bool? hasMore,
    String? error,
    bool clearError = false,
  }) {
    return NotesListState(
      notes: notes ?? this.notes,
      isLoading: isLoading ?? this.isLoading,
      isLoadingMore: isLoadingMore ?? this.isLoadingMore,
      isSaving: isSaving ?? this.isSaving,
      nextOffset: nextOffset ?? this.nextOffset,
      hasMore: hasMore ?? this.hasMore,
      error: clearError ? null : (error ?? this.error),
    );
  }
}

/// Drives the notes list.
///
/// A single controller owns the list rather than one per screen, so a note
/// created in the editor is visible when the list is popped back into view
/// without a refetch.
class NotesListController extends Notifier<NotesListState> {
  @override
  NotesListState build() => const NotesListState();

  NotesRepository get _repository => ref.read(notesRepositoryProvider);

  /// Loads the first page, replacing whatever is on screen.
  Future<void> load() async {
    state = state.copyWith(isLoading: true, clearError: true);

    try {
      final notes = await _repository.list(
        query: ref.read(searchQueryProvider),
        limit: AppConfig.pageSize,
      );
      state = state.copyWith(
        notes: notes,
        isLoading: false,
        nextOffset: notes.length,
        hasMore: notes.length >= AppConfig.pageSize,
        clearError: true,
      );
    } on ApiException catch (error) {
      state = state.copyWith(isLoading: false, error: error.message);
    }
  }

  /// Appends the next page, if there is one.
  Future<void> loadMore() async {
    // Guarded rather than merely checked at the call site: a scroll
    // notification can fire several times for one gesture.
    if (state.isLoading || state.isLoadingMore || !state.hasMore) return;

    state = state.copyWith(isLoadingMore: true);

    // Read once: a write that lands while the request is in flight would
    // otherwise shift the window between deciding on it and using it.
    final offset = state.nextOffset;

    try {
      final page = await _repository.list(
        query: ref.read(searchQueryProvider),
        limit: AppConfig.pageSize,
        offset: offset,
      );
      state = state.copyWith(
        notes: _appendPage(state.notes, page),
        isLoadingMore: false,
        // The cursor advances by what the server returned, not by what was
        // added: a note created since the window was opened comes back in this
        // page and must not be counted twice.
        nextOffset: offset + page.length,
        hasMore: page.length >= AppConfig.pageSize,
        clearError: true,
      );
    } on ApiException catch (error) {
      state = state.copyWith(isLoadingMore: false, error: error.message);
    }
  }

  /// Applies a search term and reloads from the first page.
  Future<void> search(String query) async {
    ref.read(searchQueryProvider.notifier).update(query);
    await load();
  }

  /// Creates a note and shows it straight away.
  ///
  /// The note is placed where the server's ordering puts it rather than at the
  /// top of the screen. The list is paged by offset over an ascending id, so a
  /// locally reordered list no longer describes the window the next request
  /// reads, and paging then repeats or skips a row.
  Future<bool> create({required String title, required String content}) async {
    return _mutate(() async {
      final note = await _repository.create(title: title, content: content);
      state = state.copyWith(notes: _insertSorted(state.notes, note));
    });
  }

  /// Saves changes to an existing note, replacing it in place.
  ///
  /// A note that is not in the list — edited from a deep link, or created in
  /// another tab — is added rather than dropped: the write succeeded, and a
  /// successful write that leaves the screen disagreeing with the server is
  /// worse than one extra row.
  Future<bool> update(int id, {required String title, required String content}) async {
    return _mutate(() async {
      final note = await _repository.update(id, title: title, content: content);

      final known = state.notes.any((existing) => existing.id == id);
      state = state.copyWith(
        notes: known ? _replace(state.notes, note) : _insertSorted(state.notes, note),
      );
    });
  }

  /// Deletes a note and drops it from the list.
  ///
  /// Removing a row shifts every later server offset down by one, so the cursor
  /// moves with it. Without that the next page would skip a note.
  Future<bool> delete(int id) async {
    return _mutate(() async {
      await _repository.delete(id);

      final index = state.notes.indexWhere((note) => note.id == id);
      final shiftsWindow = index >= 0 && index < state.nextOffset;

      state = state.copyWith(
        notes: state.notes.where((note) => note.id != id).toList(),
        nextOffset: shiftsWindow ? state.nextOffset - 1 : state.nextOffset,
      );
    });
  }

  /// Runs a write, keeping [NotesListState.isSaving] and its error in step.
  ///
  /// Returns whether the write succeeded, so the editor knows whether to close.
  Future<bool> _mutate(Future<void> Function() action) async {
    if (state.isSaving) return false;

    state = state.copyWith(isSaving: true, clearError: true);

    try {
      await action();
      state = state.copyWith(isSaving: false, clearError: true);
      return true;
    } on ApiException catch (error) {
      state = state.copyWith(isSaving: false, error: error.message);
      return false;
    }
  }

  /// Drops the current error, once it has been shown.
  void clearError() => state = state.copyWith(clearError: true);
}

/// The server orders by ascending id, so the list keeps that order. Everything
/// that changes a note's identity has to agree with it, or the offsets the next
/// page asks for no longer line up with the rows already on screen.
int _compareById(Note a, Note b) => a.id.compareTo(b.id);

/// Adds [note], or replaces the note already carrying its id.
List<Note> _insertSorted(List<Note> notes, Note note) {
  final merged = _replace(notes, note);

  final index = merged.indexWhere((existing) => _compareById(existing, note) > 0);
  return index == -1
      ? [...merged, note]
      : [...merged.sublist(0, index), note, ...merged.sublist(index)];
}

List<Note> _replace(List<Note> notes, Note note) {
  var found = false;

  final replaced = notes.map((existing) {
    if (existing.id != note.id) return existing;
    found = true;
    return note;
  }).toList();

  return found ? replaced : notes;
}

/// Appends a page, skipping notes already on screen.
///
/// A note created while an earlier page was open is returned again by the next
/// request, because it took the slot at the cursor. Dropping the repeat here is
/// what keeps that from showing up twice.
List<Note> _appendPage(List<Note> notes, List<Note> page) {
  if (page.isEmpty) return notes;

  final seen = notes.map((note) => note.id).toSet();
  return [
    ...notes,
    for (final note in page)
      if (seen.add(note.id)) note,
  ];
}

final notesListControllerProvider = NotifierProvider<NotesListController, NotesListState>(
  NotesListController.new,
);
