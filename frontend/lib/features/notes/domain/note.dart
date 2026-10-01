/// A note as the domain sees it.
///
/// Free of JSON and of HTTP: the wire format is the repository's concern, so a
/// change in field naming cannot reach the UI.
class Note {
  const Note({
    required this.id,
    required this.title,
    required this.content,
    required this.createdAt,
    required this.updatedAt,
  });

  final int id;
  final String title;
  final String content;
  final DateTime createdAt;
  final DateTime updatedAt;

  /// True when the note has been edited since it was created.
  bool get isEdited => updatedAt.isAfter(createdAt);

  /// A short excerpt for list rows.
  ///
  /// Collapses whitespace so a multi-line body does not render as several
  /// blank-looking lines, and bounds the length so a long note cannot dominate
  /// the row.
  String get preview {
    final flattened = content.replaceAll(RegExp(r'\s+'), ' ').trim();
    if (flattened.length <= 140) return flattened;
    return '${flattened.substring(0, 140)}…';
  }

  @override
  bool operator ==(Object other) =>
      identical(this, other) ||
      other is Note &&
          other.id == id &&
          other.title == title &&
          other.content == content &&
          other.createdAt == createdAt &&
          other.updatedAt == updatedAt;

  @override
  int get hashCode => Object.hash(id, title, content, createdAt, updatedAt);

  @override
  String toString() => 'Note(id: $id, title: $title)';
}
