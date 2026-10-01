import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../app/theme.dart';
import '../../../core/config/app_config.dart';
import '../../../core/network/api_exception.dart';
import '../application/notes_list_controller.dart';
import '../application/single_note_controller.dart';
import '../domain/note.dart';
import '../domain/note_validation.dart';

/// Creates a new note or edits an existing one.
///
/// One screen serves both: the fields, the validation and the save path are
/// identical, and a separate create screen would only duplicate them.
class NoteEditorScreen extends ConsumerStatefulWidget {
  const NoteEditorScreen({this.noteId, super.key});

  /// Null when composing a new note.
  final int? noteId;

  bool get isEditing => noteId != null;

  @override
  ConsumerState<NoteEditorScreen> createState() => _NoteEditorScreenState();
}

class _NoteEditorScreenState extends ConsumerState<NoteEditorScreen> {
  final _formKey = GlobalKey<FormState>();
  final _titleController = TextEditingController();
  final _contentController = TextEditingController();

  /// True once the fields have been filled from the server, so an autosave or a
  /// rebuild cannot write an empty note over the original.
  bool _isInitialized = false;

  @override
  void dispose() {
    _titleController.dispose();
    _contentController.dispose();
    super.dispose();
  }

  /// Fills the fields the first time the note arrives.
  ///
  /// Guarded by [_isInitialized] because rebuilds are common — the provider is
  /// watched, and Riverpod may deliver the same data twice — and refilling would
  /// discard whatever the user has typed since.
  ///
  /// Deferred to the end of the frame: assigning to a [TextEditingController]
  /// during build rebuilds the field that owns it, and Flutter throws when that
  /// happens part-way through laying out a tree.
  void _fill(Note note) {
    if (_isInitialized) return;
    _isInitialized = true;

    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!mounted) return;
      _titleController.text = note.title;
      _contentController.text = note.content;
    });
  }

  Future<void> _save() async {
    if (!(_formKey.currentState?.validate() ?? false)) return;

    FocusScope.of(context).unfocus();

    final controller = ref.read(notesListControllerProvider.notifier);
    final title = _titleController.text.trim();
    final content = _contentController.text.trim();

    final saved = widget.isEditing
        ? await controller.update(
            widget.noteId!,
            title: title,
            content: content,
          )
        : await controller.create(title: title, content: content);

    if (!mounted) return;

    if (saved) {
      Navigator.of(context).pop();
      return;
    }

    // Staying open keeps the typed text; the reason is shown on the field rather
    // than in a snackbar that a keystroke could cover.
    final message = ref.read(notesListControllerProvider).error;
    if (message != null) {
      ScaffoldMessenger.of(context)
        ..hideCurrentSnackBar()
        ..showSnackBar(SnackBar(content: Text(message)));
      ref.read(notesListControllerProvider.notifier).clearError();
    }
  }

  @override
  Widget build(BuildContext context) {
    final isSaving = ref.watch(
      notesListControllerProvider.select((state) => state.isSaving),
    );

    final title = widget.isEditing ? 'Edit note' : 'New note';

    return Scaffold(
      appBar: AppBar(
        title: Text(title),
        actions: [
          Padding(
            padding: const EdgeInsets.only(right: AppTokens.sm),
            child: FilledButton.tonal(
              // Disabled while saving, so a double tap cannot create two notes.
              onPressed: isSaving ? null : _save,
              style: FilledButton.styleFrom(
                minimumSize: const Size(0, 40),
                padding: const EdgeInsets.symmetric(horizontal: AppTokens.lg),
              ),
              child: isSaving
                  ? const SizedBox.square(
                      dimension: 16,
                      child: CircularProgressIndicator(strokeWidth: 2),
                    )
                  : const Text('Save'),
            ),
          ),
        ],
      ),
      body: _buildBody(),
    );
  }

  Widget _buildBody() {
    final isSaving = ref.watch(
      notesListControllerProvider.select((state) => state.isSaving),
    );

    if (widget.isEditing) {
      final noteAsync = ref.watch(noteControllerProvider(widget.noteId!));

      return noteAsync.when(
        loading: () => const Center(child: CircularProgressIndicator()),
        error: (error, _) => _LoadFailed(
          message: _messageFor(error),
          isSaving: isSaving,
          onRetry: () => ref.invalidate(noteControllerProvider(widget.noteId!)),
        ),
        data: (note) {
          _fill(note);
          return _buildForm();
        },
      );
    }

    return _buildForm();
  }

  Widget _buildForm() {
    final isSaving = ref.watch(
      notesListControllerProvider.select((state) => state.isSaving),
    );
    final theme = Theme.of(context);

    return Form(
      key: _formKey,
      child: ListView(
        padding: const EdgeInsets.all(AppTokens.lg),
        children: [
          TextFormField(
            controller: _titleController,
            autofocus: !widget.isEditing,
            textCapitalization: TextCapitalization.sentences,
            textInputAction: TextInputAction.next,
            style: theme.textTheme.titleMedium,
            decoration: const InputDecoration(
              labelText: 'Title',
              hintText: 'What is this note about?',
            ),
            // Validated against the server's own rule, including its rune-based
            // length, so the two cannot disagree.
            validator: (value) => validateTitle(value ?? ''),
            // The count updates as the user types, because the limit is easy to
            // approach without noticing.
            onChanged: (_) => setState(() {}),
          ),
          const SizedBox(height: AppTokens.sm),
          _TitleCounter(controller: _titleController),
          const SizedBox(height: AppTokens.lg),
          TextFormField(
            controller: _contentController,
            textCapitalization: TextCapitalization.sentences,
            minLines: 10,
            maxLines: null,
            keyboardType: TextInputType.multiline,
            style: theme.textTheme.bodyLarge?.copyWith(height: 1.5),
            decoration: const InputDecoration(
              labelText: 'Content',
              alignLabelWithHint: true,
              hintText: 'Start writing…',
            ),
            validator: (value) => validateContent(value ?? ''),
          ),
          const SizedBox(height: AppTokens.xxl),
          FilledButton.icon(
            onPressed: isSaving ? null : _save,
            icon: Icon(
              widget.isEditing ? Icons.check_rounded : Icons.add_rounded,
            ),
            label: Text(widget.isEditing ? 'Save changes' : 'Create note'),
          ),
        ],
      ),
    );
  }

  /// Turns a failure into text a reader can act on.
  ///
  /// The repository maps HTTP failures onto [ApiException], whose message is
  /// written for a user; interpolating the error object instead printed its Dart
  /// type, e.g. "ApiExceptionNotFound: This note no longer exists." The generic
  /// branch keeps an unexpected error from reaching the screen as prose.
  String _messageFor(Object error) {
    return switch (error) {
      ApiException() => error.message,
      _ => 'Could not load this note.',
    };
  }
}

/// Live character count, so the title limit is visible rather than discovered on
/// save.
///
/// Rendered as a chip rather than loose text: it sits under the field it
/// measures, and a pill makes it read as a readout rather than a caption.
class _TitleCounter extends StatelessWidget {
  const _TitleCounter({required this.controller});

  final TextEditingController controller;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);

    // Counted in runes, like the server: a title of emoji occupies far more
    // UTF-16 code units than visible characters.
    final length = controller.text.runes.length;
    final overLimit = length > AppConfig.maxTitleLength;

    final color = overLimit
        ? theme.colorScheme.error
        : theme.colorScheme.onSurfaceVariant;

    return Align(
      alignment: Alignment.centerRight,
      child: Container(
        padding: const EdgeInsets.symmetric(
          horizontal: AppTokens.sm + 2,
          vertical: 3,
        ),
        decoration: BoxDecoration(
          color: overLimit
              ? theme.colorScheme.errorContainer
              : theme.colorScheme.surfaceContainerHighest,
          borderRadius: BorderRadius.circular(AppTokens.radiusFull),
        ),
        child: Text(
          '$length / ${AppConfig.maxTitleLength}',
          style: theme.textTheme.labelSmall?.copyWith(
            color: overLimit ? theme.colorScheme.onErrorContainer : color,
            fontFeatures: const [FontFeature.tabularFigures()],
          ),
        ),
      ),
    );
  }
}

/// Shown when the note being edited could not be loaded.
class _LoadFailed extends StatelessWidget {
  const _LoadFailed({
    required this.message,
    required this.isSaving,
    required this.onRetry,
  });

  final String message;
  final bool isSaving;
  final VoidCallback onRetry;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);

    return Center(
      child: Padding(
        padding: const EdgeInsets.all(AppTokens.xxl),
        child: ConstrainedBox(
          constraints: const BoxConstraints(maxWidth: 360),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Icon(
                Icons.error_outline_rounded,
                size: 40,
                color: theme.colorScheme.onSurfaceVariant,
              ),
              const SizedBox(height: AppTokens.lg),
              Text(
                message,
                style: theme.textTheme.bodyLarge,
                textAlign: TextAlign.center,
              ),
              const SizedBox(height: AppTokens.xl),
              FilledButton.tonalIcon(
                onPressed: isSaving ? null : onRetry,
                icon: const Icon(Icons.refresh_rounded, size: 20),
                label: const Text('Try again'),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
