import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../../app/theme.dart';
import '../../../app/theme_controller.dart';
import '../../../core/utils/debouncer.dart';
import '../../../core/widgets/api_endpoint_hint.dart';
import '../../../core/widgets/theme_toggle.dart';
import '../../auth/application/auth_controller.dart';
import '../application/notes_list_controller.dart';
import '../domain/note.dart';
import 'widgets/note_card.dart';

/// The list of notes, with search and paging.
class NotesListScreen extends ConsumerStatefulWidget {
  const NotesListScreen({super.key});

  @override
  ConsumerState<NotesListScreen> createState() => _NotesListScreenState();
}

class _NotesListScreenState extends ConsumerState<NotesListScreen> {
  final _scrollController = ScrollController();
  final _searchController = TextEditingController();
  final _searchFocus = FocusNode();

  /// Debounces search so a request is not sent per keystroke.
  ///
  /// The server rate limits per client at 20 requests per second, and a
  /// five-letter search would otherwise spend five of them immediately.
  final _searchDebouncer = Debouncer(const Duration(milliseconds: 400));

  @override
  void initState() {
    super.initState();
    _scrollController.addListener(_onScroll);

    // Started here rather than from the controller's build, because a provider
    // that fetches during construction cannot be read while it initialises and
    // leaves the request untestable.
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!mounted) return;
      ref.read(notesListControllerProvider.notifier).load();
    });
  }

  @override
  void dispose() {
    // Without this the pending search would fire against a disposed element.
    _searchDebouncer.dispose();
    _scrollController.dispose();
    _searchController.dispose();
    _searchFocus.dispose();
    super.dispose();
  }

  /// Fetches the next page when the list is scrolled near its end.
  ///
  /// Guarded rather than a jump straight to the controller: scrolling past the
  /// end fires many events, and each one would otherwise start a request.
  void _onScroll() {
    if (!_scrollController.hasClients) return;

    final position = _scrollController.position;
    if (position.pixels >= position.maxScrollExtent - 300) {
      ref.read(notesListControllerProvider.notifier).loadMore();
    }
  }

  void _onSearchChanged(String value) {
    _searchDebouncer.run(() {
      if (!mounted) return;
      // Jumping to the top: the result set changed, so the old offset no longer
      // points anywhere meaningful.
      if (_scrollController.hasClients) {
        _scrollController.jumpTo(0);
      }
      ref.read(notesListControllerProvider.notifier).search(value);
    });
  }

  void _clearSearch() {
    _searchController.clear();
    _searchFocus.unfocus();
    // Cancelled rather than waited out: clearing is an explicit request to see
    // everything now, not the end of a search being typed.
    _searchDebouncer.cancel();
    ref.read(notesListControllerProvider.notifier).search('');
  }

  /// Opens the editor, creating a note when [note] is null.
  ///
  /// Pushed through the router rather than through [Navigator.push]: the editor
  /// is a declared route, and building a second one by hand put a screen on the
  /// stack that the router does not know exists. A deep link to `/notes/5` and
  /// an in-app tap at note 5 then produced two different stacks, so the back
  /// button behaved differently depending on how the user arrived.
  void _openEditor([Note? note]) {
    context.push(note == null ? '/notes/new' : '/notes/${note.id}');
  }

  Future<void> _confirmDelete(Note note) async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: const Text('Delete this note?'),
        content: Text('"${note.title}" will be removed permanently.'),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(dialogContext).pop(false),
            child: const Text('Cancel'),
          ),
          FilledButton(
            style: FilledButton.styleFrom(
              backgroundColor: Theme.of(dialogContext).colorScheme.error,
              foregroundColor: Theme.of(dialogContext).colorScheme.onError,
              minimumSize: const Size(0, 44),
            ),
            onPressed: () => Navigator.of(dialogContext).pop(true),
            child: const Text('Delete'),
          ),
        ],
      ),
    );

    if (confirmed != true || !mounted) return;

    await ref.read(notesListControllerProvider.notifier).delete(note.id);
  }

  /// Shows the controller's current error once, then clears it.
  ///
  /// Reading and clearing here rather than watching for changes keeps the
  /// snackbar to one per failure, instead of reappearing on every rebuild.
  void _showError() {
    final message = ref.read(notesListControllerProvider).error;
    if (message == null) return;

    ScaffoldMessenger.of(context)
      ..hideCurrentSnackBar()
      ..showSnackBar(SnackBar(content: Text(message)));
    ref.read(notesListControllerProvider.notifier).clearError();
  }

  @override
  Widget build(BuildContext context) {
    final state = ref.watch(notesListControllerProvider);
    final query = ref.watch(searchQueryProvider);

    // A failure that leaves notes on screen is reported over the content the
    // user is still working with. A failure with nothing loaded becomes a
    // full-screen state instead, because there is no content to sit over.
    ref.listen(notesListControllerProvider, (previous, next) {
      final failed = next.error != null && next.error != previous?.error;
      if (failed && next.notes.isNotEmpty) {
        _showError();
      }
    });

    return Scaffold(
      body: SafeArea(
        bottom: false,
        child: Column(
          children: [
            _Header(
              count: state.notes.isEmpty ? null : state.notes.length,
              onSignOut: () =>
                  ref.read(authControllerProvider.notifier).signOut(),
            ),
            Padding(
              padding: const EdgeInsets.fromLTRB(
                AppTokens.lg,
                AppTokens.sm,
                AppTokens.lg,
                AppTokens.md,
              ),
              child: _SearchField(
                controller: _searchController,
                focusNode: _searchFocus,
                hasQuery: query.isNotEmpty,
                onChanged: _onSearchChanged,
                onClear: _clearSearch,
              ),
            ),
            Expanded(child: _buildBody(state, query)),
          ],
        ),
      ),
      floatingActionButton: FloatingActionButton.extended(
        onPressed: state.isSaving ? null : () => _openEditor(),
        icon: const Icon(Icons.add_rounded),
        label: const Text('New note'),
      ),
    );
  }

  Widget _buildBody(NotesListState state, String query) {
    if (state.isLoading) {
      return const Center(child: CircularProgressIndicator());
    }

    if (state.notes.isEmpty && state.error != null) {
      return _LoadFailed(
        message: state.error!,
        onRetry: () => ref.read(notesListControllerProvider.notifier).load(),
      );
    }

    if (state.isEmpty) {
      return _EmptyState(hasQuery: query.isNotEmpty);
    }

    return RefreshIndicator(
      onRefresh: () => ref.read(notesListControllerProvider.notifier).load(),
      child: ListView.separated(
        controller: _scrollController,
        // Always scrollable so pull-to-refresh works even when the content is
        // shorter than the viewport.
        physics: const AlwaysScrollableScrollPhysics(),
        // Cleared of the FAB's height, so the last card is not hidden behind it.
        padding: const EdgeInsets.fromLTRB(
          AppTokens.lg,
          0,
          AppTokens.lg,
          AppTokens.xxxl * 2,
        ),
        itemCount: state.notes.length + (state.isLoadingMore ? 1 : 0),
        separatorBuilder: (_, _) => const SizedBox(height: AppTokens.md),
        itemBuilder: (context, index) {
          if (index >= state.notes.length) {
            return const Padding(
              padding: EdgeInsets.all(AppTokens.xl),
              child: Center(
                child: SizedBox.square(
                  dimension: 24,
                  child: CircularProgressIndicator(strokeWidth: 2),
                ),
              ),
            );
          }

          final note = state.notes[index];
          return NoteCard(
            note: note,
            onTap: () => _openEditor(note),
            onDelete: () => _confirmDelete(note),
          );
        },
      ),
    );
  }
}

/// The title bar, with the note count and the header actions.
class _Header extends ConsumerWidget {
  const _Header({required this.count, required this.onSignOut});

  /// Null while the count is unknown, which is only during the first load.
  final int? count;
  final VoidCallback onSignOut;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final theme = Theme.of(context);

    return Padding(
      padding: const EdgeInsets.fromLTRB(
        AppTokens.lg,
        AppTokens.md,
        AppTokens.sm,
        AppTokens.xs,
      ),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.center,
        children: [
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text('Notes', style: theme.textTheme.headlineSmall),
                if (count != null) ...[
                  const SizedBox(height: 2),
                  Text(
                    _countLabel(count!),
                    style: theme.textTheme.bodySmall?.copyWith(
                      color: theme.colorScheme.onSurfaceVariant,
                    ),
                  ),
                ],
              ],
            ),
          ),
          // Theme before sign-out: switching appearance is frequent and
          // harmless, whereas signing out is rare and destructive, so the
          // low-consequence action sits where it is easiest to reach.
          ThemeToggle(
            mode: ref.watch(themeControllerProvider),
            onPressed: () => ref.read(themeControllerProvider.notifier).cycle(),
          ),
          IconButton(
            tooltip: 'Sign out',
            icon: const Icon(Icons.logout_rounded),
            onPressed: onSignOut,
          ),
        ],
      ),
    );
  }

  /// A count that reads naturally in both numbers.
  static String _countLabel(int count) =>
      count == 1 ? '1 note' : '$count notes';
}

/// The search field.
///
/// The clear button is part of the field rather than the app bar so it sits next
/// to the text it clears, which is where the eye already is.
class _SearchField extends StatelessWidget {
  const _SearchField({
    required this.controller,
    required this.focusNode,
    required this.hasQuery,
    required this.onChanged,
    required this.onClear,
  });

  final TextEditingController controller;
  final FocusNode focusNode;
  final bool hasQuery;
  final ValueChanged<String> onChanged;
  final VoidCallback onClear;

  @override
  Widget build(BuildContext context) {
    return TextField(
      controller: controller,
      focusNode: focusNode,
      onChanged: onChanged,
      textInputAction: TextInputAction.search,
      decoration: InputDecoration(
        hintText: 'Search notes',
        prefixIcon: const Icon(Icons.search_rounded, size: 22),
        suffixIcon: hasQuery
            ? IconButton(
                icon: const Icon(Icons.close_rounded, size: 20),
                tooltip: 'Clear search',
                onPressed: onClear,
              )
            : null,
        contentPadding: const EdgeInsets.symmetric(vertical: AppTokens.md),
      ),
    );
  }
}

/// Shown when there is genuinely nothing to list.
class _EmptyState extends StatelessWidget {
  const _EmptyState({required this.hasQuery});

  final bool hasQuery;

  @override
  Widget build(BuildContext context) {
    return _CenteredMessage(
      icon: hasQuery ? Icons.search_off_rounded : Icons.auto_stories_outlined,
      title: hasQuery ? 'Nothing matches that search' : 'No notes yet',
      body: hasQuery
          ? 'Try a different word, or clear the search.'
          : 'Create your first note to get started.',
    );
  }
}

/// Shown when the first load failed, offering a retry.
class _LoadFailed extends StatelessWidget {
  const _LoadFailed({required this.message, required this.onRetry});

  final String message;
  final VoidCallback onRetry;

  @override
  Widget build(BuildContext context) {
    return _CenteredMessage(
      icon: Icons.cloud_off_rounded,
      title: message,
      footnote: const ApiEndpointHint(),
      action: FilledButton.tonalIcon(
        onPressed: onRetry,
        icon: const Icon(Icons.refresh_rounded, size: 20),
        label: const Text('Try again'),
      ),
    );
  }
}

/// A vertically centred icon, message and optional action.
///
/// Shared by both empty states so "nothing here" and "something went wrong"
/// occupy the same shape and the difference between them is the wording and the
/// action.
class _CenteredMessage extends StatelessWidget {
  const _CenteredMessage({
    required this.icon,
    required this.title,
    this.body,
    this.footnote,
    this.action,
  });

  final IconData icon;
  final String title;
  final String? body;
  final Widget? footnote;
  final Widget? action;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);

    return Center(
      child: SingleChildScrollView(
        padding: const EdgeInsets.all(AppTokens.xxl),
        child: ConstrainedBox(
          constraints: const BoxConstraints(maxWidth: 360),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Container(
                width: 88,
                height: 88,
                decoration: BoxDecoration(
                  color: theme.colorScheme.surfaceContainerHighest.withValues(
                    alpha: 0.6,
                  ),
                  shape: BoxShape.circle,
                ),
                child: Icon(
                  icon,
                  size: 40,
                  color: theme.colorScheme.onSurfaceVariant,
                ),
              ),
              const SizedBox(height: AppTokens.xl),
              Text(
                title,
                style: theme.textTheme.titleMedium,
                textAlign: TextAlign.center,
              ),
              if (body != null) ...[
                const SizedBox(height: AppTokens.sm),
                Text(
                  body!,
                  style: theme.textTheme.bodyMedium?.copyWith(
                    color: theme.colorScheme.onSurfaceVariant,
                    height: 1.5,
                  ),
                  textAlign: TextAlign.center,
                ),
              ],
              ?footnote,
              if (action != null) ...[
                const SizedBox(height: AppTokens.xl),
                action!,
              ],
            ],
          ),
        ),
      ),
    );
  }
}
