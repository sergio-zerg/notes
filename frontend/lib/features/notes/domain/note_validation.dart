import '../../../core/config/app_config.dart';

/// The server's validation rules, mirrored for use before a request is sent.
///
/// The server stays the authority — it validates in its domain layer and will
/// reject anything invalid regardless of what the client checks. Checking here
/// is about the experience: the user learns that a title is too short without a
/// round trip, and the save button does not fail after appearing to work.
///
/// Both limits live in [AppConfig] and are expressed in runes, because the server
/// counts runes too. Dart's `String.length` counts UTF-16 code units, so an
/// emoji-heavy title would pass a naive client check and then be rejected on
/// save — the client must not be more permissive than the server.

/// Validates a note title, returning a message or null when it is acceptable.
String? validateTitle(String value) {
  final trimmed = value.trim();

  if (trimmed.isEmpty) {
    return 'Title is required';
  }

  final length = trimmed.runes.length;
  if (length < AppConfig.minTitleLength) {
    return 'Title must be at least ${AppConfig.minTitleLength} characters';
  }
  if (length > AppConfig.maxTitleLength) {
    return 'Title must be at most ${AppConfig.maxTitleLength} characters';
  }

  return null;
}

/// Validates note content, which the server requires to be non-empty.
String? validateContent(String value) {
  if (value.trim().isEmpty) {
    return 'Content is required';
  }
  return null;
}
