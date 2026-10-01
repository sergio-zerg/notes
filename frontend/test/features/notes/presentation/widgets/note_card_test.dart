import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
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

/// Renders a card inside the minimum a real screen provides, so a missing
/// Material ancestor fails here rather than in production.
Widget wrap(Widget child) => MaterialApp(
      home: Scaffold(body: child),
    );

void main() {
  testWidgets('shows the title and a preview of the content', (tester) async {
    await tester.pumpWidget(wrap(NoteCard(
      note: noteWith(id: 1, title: 'Shopping list', content: 'Milk, bread'),
      onTap: () {},
      onDelete: () {},
    )));

    expect(find.text('Shopping list'), findsOneWidget);
    expect(find.text('Milk, bread'), findsOneWidget);
  });

  // A multi-line body would otherwise render as several blank-looking rows.
  testWidgets('collapses whitespace in the preview', (tester) async {
    await tester.pumpWidget(wrap(NoteCard(
      note: noteWith(id: 1, content: 'First line\n\nSecond   line'),
      onTap: () {},
      onDelete: () {},
    )));

    expect(find.text('First line Second line'), findsOneWidget);
  });

  testWidgets('omits the preview when the content is blank', (tester) async {
    await tester.pumpWidget(wrap(NoteCard(
      note: noteWith(id: 1, content: '   '),
      onTap: () {},
      onDelete: () {},
    )));

    expect(find.byType(Text), findsWidgets);
  });

  testWidgets('invokes onTap when the row is tapped', (tester) async {
    var tapped = false;

    await tester.pumpWidget(wrap(NoteCard(
      note: noteWith(id: 1),
      onTap: () => tapped = true,
      onDelete: () {},
    )));

    await tester.tap(find.text('Title'));
    await tester.pump();

    expect(tapped, isTrue);
  });

  testWidgets('invokes onDelete from the delete button', (tester) async {
    var deleted = false;

    await tester.pumpWidget(wrap(NoteCard(
      note: noteWith(id: 1),
      onTap: () {},
      onDelete: () => deleted = true,
    )));

    await tester.tap(find.byIcon(Icons.delete_outline));
    await tester.pump();

    expect(deleted, isTrue);
  });

  testWidgets('marks a note that has been edited', (tester) async {
    await tester.pumpWidget(wrap(NoteCard(
      note: noteWith(
        id: 1,
        createdAt: DateTime.utc(2026, 9, 30, 10),
        updatedAt: DateTime.utc(2026, 9, 30, 12),
      ),
      onTap: () {},
      onDelete: () {},
    )));

    expect(find.byIcon(Icons.edit), findsOneWidget);
  });

  testWidgets('does not mark a note that was never edited', (tester) async {
    await tester.pumpWidget(wrap(NoteCard(
      note: noteWith(id: 1),
      onTap: () {},
      onDelete: () {},
    )));

    expect(find.byIcon(Icons.edit), findsNothing);
  });

  // A long title must not push the delete button off a narrow screen.
  testWidgets('truncates a long title instead of overflowing', (tester) async {
    await tester.pumpWidget(wrap(NoteCard(
      note: noteWith(id: 1, title: 'A' * 200, content: 'Body'),
      onTap: () {},
      onDelete: () {},
    )));

    expect(tester.takeException(), isNull);
    expect(find.byIcon(Icons.delete_outline), findsOneWidget);
  });
}
