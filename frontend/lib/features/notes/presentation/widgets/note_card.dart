import 'package:flutter/material.dart';

import '../../../../app/theme.dart';
import '../../domain/note.dart';

/// One note in the list.
///
/// The card is a surface with a hairline edge rather than a shadow, and the
/// title carries the strongest weight on the row so a long list scans by title
/// first and detail second.
class NoteCard extends StatelessWidget {
  const NoteCard({
    required this.note,
    required this.onTap,
    required this.onDelete,
    super.key,
  });

  final Note note;
  final VoidCallback onTap;
  final VoidCallback onDelete;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final colors = theme.colorScheme;
    final preview = note.preview;

    return Card(
      clipBehavior: Clip.antiAlias,
      child: InkWell(
        onTap: onTap,
        child: Padding(
          padding: const EdgeInsets.fromLTRB(
            AppTokens.lg,
            AppTokens.lg,
            AppTokens.sm,
            AppTokens.md,
          ),
          child: Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      note.title,
                      style: theme.textTheme.titleMedium?.copyWith(
                        height: 1.25,
                      ),
                      maxLines: 2,
                      overflow: TextOverflow.ellipsis,
                    ),
                    if (preview.isNotEmpty) ...[
                      const SizedBox(height: AppTokens.sm - 2),
                      Text(
                        preview,
                        style: theme.textTheme.bodyMedium?.copyWith(
                          color: colors.onSurfaceVariant,
                          height: 1.4,
                        ),
                        maxLines: 2,
                        overflow: TextOverflow.ellipsis,
                      ),
                    ],
                    const SizedBox(height: AppTokens.md),
                    _Metadata(note: note),
                  ],
                ),
              ),
              _DeleteAction(onPressed: onDelete),
            ],
          ),
        ),
      ),
    );
  }
}

/// The delete control.
///
/// Tinted with the error role so the one destructive action on the row is the
/// only thing in that colour, rather than a neutral glyph that reads as
/// "more options".
class _DeleteAction extends StatelessWidget {
  const _DeleteAction({required this.onPressed});

  final VoidCallback onPressed;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).colorScheme;

    return IconButton(
      onPressed: onPressed,
      icon: const Icon(Icons.delete_outline, size: 20),
      tooltip: 'Delete note',
      style: IconButton.styleFrom(
        foregroundColor: colors.error,
        minimumSize: const Size(40, 40),
        shape: RoundedRectangleBorder(
          borderRadius: BorderRadius.circular(AppTokens.radiusSm),
        ),
      ),
    );
  }
}

/// When the note was last touched, and whether it was ever edited.
class _Metadata extends StatelessWidget {
  const _Metadata({required this.note});

  final Note note;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final style = theme.textTheme.labelMedium?.copyWith(
      color: theme.colorScheme.onSurfaceVariant,
    );

    return Row(
      children: [
        Flexible(
          child: Text(
            formatNoteTimestamp(note.updatedAt.toLocal()),
            style: style,
            overflow: TextOverflow.ellipsis,
          ),
        ),
        if (note.isEdited) ...[
          const SizedBox(width: AppTokens.sm),
          Icon(Icons.edit, size: 12, color: theme.colorScheme.onSurfaceVariant),
          const SizedBox(width: AppTokens.xs),
          Text('Edited', style: style),
        ],
      ],
    );
  }
}

/// Formats the timestamp the way the list shows it.
///
/// Written out rather than pulled from `intl` because the list needs one fixed
/// compact form on every locale: a locale-aware `DateFormat` would reorder the
/// parts and, for some locales, widen the metadata row enough to wrap it.
String formatNoteTimestamp(DateTime value) {
  const months = [
    'Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', //
    'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec',
  ];
  final hour = value.hour.toString().padLeft(2, '0');
  final minute = value.minute.toString().padLeft(2, '0');
  return '${months[value.month - 1]} ${value.day}, $hour:$minute';
}
