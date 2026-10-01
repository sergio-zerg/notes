import 'package:flutter_test/flutter_test.dart';
import 'package:notes_frontend/features/notes/domain/note_validation.dart';

void main() {
  group('validateTitle', () {
    test('accepts a normal title', () {
      expect(validateTitle('My Note Title'), isNull);
    });

    test('rejects an empty or whitespace-only title', () {
      expect(validateTitle(''), 'Title is required');
      expect(validateTitle('   '), 'Title is required');
      expect(validateTitle('\n\t'), 'Title is required');
    });

    test('rejects a title below the minimum length', () {
      expect(validateTitle('ab'), contains('at least 3'));
    });

    test('accepts a title at exactly the minimum length', () {
      expect(validateTitle('abc'), isNull);
    });

    test('rejects a title above the maximum length', () {
      expect(validateTitle('a' * 101), contains('at most 100'));
    });

    test('accepts a title at exactly the maximum length', () {
      expect(validateTitle('a' * 100), isNull);
    });

    // The server counts runes. Counting UTF-16 code units instead would reject
    // a legitimate 100-emoji title that the server accepts.
    test('measures length in runes, not UTF-16 code units', () {
      // Each of these is a surrogate pair, so String.length reports 200 while
      // the visible length is 100.
      final hundredEmoji = '😀' * 100;

      expect(hundredEmoji.length, 200);
      expect(validateTitle(hundredEmoji), isNull);
    });

    test('rejects an over-long emoji title the same way the server does', () {
      expect(validateTitle('😀' * 101), contains('at most 100'));
    });

    test('counts Cyrillic by character', () {
      expect(validateTitle('Привет'), isNull);
      expect(validateTitle('Пр'), contains('at least 3'));
    });

    // Surrounding whitespace is trimmed before measuring, so a title padded to
    // the limit is not rejected over characters the server will not store.
    test('ignores surrounding whitespace when measuring', () {
      expect(validateTitle('  ${'a' * 100}  '), isNull);
    });
  });

  group('validateContent', () {
    test('accepts any non-empty content', () {
      expect(validateContent('Some content'), isNull);
    });

    test('rejects empty or whitespace-only content', () {
      expect(validateContent(''), 'Content is required');
      expect(validateContent('  \n '), 'Content is required');
    });

    // A single character is enough; the server sets no lower bound beyond
    // presence, and a stricter client rule would refuse something valid.
    test('accepts a single character', () {
      expect(validateContent('a'), isNull);
    });
  });
}
