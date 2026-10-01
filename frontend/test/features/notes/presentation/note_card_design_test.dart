import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:notes_frontend/app/theme.dart';
import 'package:notes_frontend/features/notes/domain/note.dart';
import 'package:notes_frontend/features/notes/presentation/widgets/note_card.dart';

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
  group('formatNoteTimestamp', () {
    test('renders a compact month, day and time', () {
      expect(
        formatNoteTimestamp(DateTime(2026, 9, 30, 14, 5)),
        'Sep 30, 14:05',
      );
    });

    // An unpadded hour would make the column of timestamps ragged, which reads
    // as misalignment even though the text is correct.
    test('zero-pads the hour and minute', () {
      expect(formatNoteTimestamp(DateTime(2026, 1, 2, 9, 7)), 'Jan 2, 09:07');
    });

    // An off-by-one in the month table would show a wrong name, so every index
    // is checked rather than a sample.
    test('maps every month to its own name', () {
      const names = [
        'Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', //
        'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec',
      ];

      for (var month = 1; month <= 12; month++) {
        expect(
          formatNoteTimestamp(DateTime(2026, month, 15, 12, 0)),
          startsWith('${names[month - 1]} '),
          reason: 'month $month should render as ${names[month - 1]}',
        );
      }
    });

    test('names January and December correctly', () {
      expect(formatNoteTimestamp(DateTime(2026, 1, 1, 0, 0)), 'Jan 1, 00:00');
      expect(formatNoteTimestamp(DateTime(2026, 12, 31, 23, 59)), 'Dec 31, 23:59');
    });
  });

  group('NoteCard', () {
    testWidgets('keeps its content readable in the dark theme', (tester) async {
      await tester.pumpWidget(
        MaterialApp(
          theme: AppTheme.dark(),
          home: Scaffold(
            body: NoteCard(
              note: noteWith(id: 1, title: 'Dark mode', content: 'Body'),
              onTap: () {},
              onDelete: () {},
            ),
          ),
        ),
      );

      expect(find.text('Dark mode'), findsOneWidget);
      expect(tester.takeException(), isNull);
    });

    // The metadata row is a Row, so a long timestamp could push the "Edited"
    // marker off the edge without any error being raised.
    testWidgets('keeps the edited marker on screen with a long title',
        (tester) async {
      await tester.pumpWidget(
        MaterialApp(
          home: Scaffold(
            body: NoteCard(
              note: noteWith(
                id: 1,
                title: 'A very long title that occupies the full width of the row',
                updatedAt: DateTime.utc(2026, 9, 30, 12),
              ),
              onTap: () {},
              onDelete: () {},
            ),
          ),
        ),
      );

      expect(find.text('Edited'), findsOneWidget);
      expect(tester.takeException(), isNull);
    });

    testWidgets('renders on a narrow screen without overflowing',
        (tester) async {
      tester.view.physicalSize = const Size(320, 640);
      tester.view.devicePixelRatio = 1.0;
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);

      await tester.pumpWidget(
        MaterialApp(
          home: Scaffold(
            body: NoteCard(
              note: noteWith(
                id: 1,
                title: 'Title long enough to need truncating on a small screen',
                content: 'Content that also needs truncating on a small screen',
              ),
              onTap: () {},
              onDelete: () {},
            ),
          ),
        ),
      );

      expect(tester.takeException(), isNull);
      expect(find.byIcon(Icons.delete_outline), findsOneWidget);
    });

    testWidgets('the delete target is large enough to tap', (tester) async {
      await tester.pumpWidget(
        MaterialApp(
          home: Scaffold(
            body: NoteCard(
              note: noteWith(id: 1),
              onTap: () {},
              onDelete: () {},
            ),
          ),
        ),
      );

      // The icon is 20pt; the tappable area around it is what has to be big
      // enough, and that is the button's box rather than the glyph's.
      final button = find.ancestor(
        of: find.byIcon(Icons.delete_outline),
        matching: find.byType(IconButton),
      );

      final size = tester.getSize(button);
      expect(size.width, greaterThanOrEqualTo(40));
      expect(size.height, greaterThanOrEqualTo(40));
    });
  });
}
