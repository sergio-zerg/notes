import '../domain/note.dart';

/// Wire format of a note, kept next to the [Note] it produces.
///
/// Timestamps arrive as RFC 3339 strings, which `DateTime.parse` handles. They
/// are normalised to UTC so comparisons — such as [Note.isEdited] — do not
/// depend on the device's time zone.
class NoteModel {
  const NoteModel({
    required this.id,
    required this.title,
    required this.content,
    required this.createdAt,
    required this.updatedAt,
  });

  factory NoteModel.fromJson(Map<String, dynamic> json) {
    return NoteModel(
      id: _readInt(json, 'id'),
      title: _readString(json, 'title'),
      content: _readString(json, 'content'),
      createdAt: _readTimestamp(json, 'created_at'),
      updatedAt: _readTimestamp(json, 'updated_at'),
    );
  }

  factory NoteModel.fromEntity(Note note) {
    return NoteModel(
      id: note.id,
      title: note.title,
      content: note.content,
      createdAt: note.createdAt,
      updatedAt: note.updatedAt,
    );
  }

  final int id;
  final String title;
  final String content;
  final DateTime createdAt;
  final DateTime updatedAt;

  Map<String, dynamic> toJson() => {
    'id': id,
    'title': title,
    'content': content,
    'created_at': createdAt.toUtc().toIso8601String(),
    'updated_at': updatedAt.toUtc().toIso8601String(),
  };

  Note toEntity() => Note(
    id: id,
    title: title,
    content: content,
    createdAt: createdAt,
    updatedAt: updatedAt,
  );
}

/// Reads a required integer.
///
/// The server documents `id` as int64. A value that will not fit in a Dart int
/// is impossible here — Dart ints are 64-bit on every target this app ships to —
/// but a malformed response must not crash the list, so a non-numeric id falls
/// back to 0 and the row is skipped by the caller.
int _readInt(Map<String, dynamic> json, String field) {
  final value = json[field];
  if (value is int) return value;
  if (value is num) return value.toInt();
  return int.tryParse('$value') ?? 0;
}

/// Reads a required string, tolerating a missing field as empty rather than
/// throwing: one malformed row should not empty the whole list.
String _readString(Map<String, dynamic> json, String field) {
  final value = json[field];
  return value is String ? value : '';
}

/// Parses a required RFC 3339 timestamp.
///
/// The server always sends one, but a malformed value falls back to the epoch
/// rather than throwing, which keeps a single bad row from failing the page.
DateTime _readTimestamp(Map<String, dynamic> json, String field) {
  final value = json[field];
  if (value is String) {
    final parsed = DateTime.tryParse(value);
    if (parsed != null) return parsed.toUtc();
  }
  return DateTime.fromMillisecondsSinceEpoch(0, isUtc: true);
}
