import 'package:flutter/foundation.dart';

/// Resolves where the API lives and how much of the key to keep in memory.
///
/// The three targets cannot agree on a single address: a browser on the same
/// machine reaches `localhost`, the Android emulator reaches the host through a
/// special alias, and a physical device needs the machine's LAN address. Guessing
/// wrong produces a confusing network error, so the platform cases are explicit
/// and every one of them can be overridden from the command line.
abstract final class AppConfig {
  /// Where the API is, with a per-platform default.
  ///
  /// Override for a physical device or a non-local backend:
  /// `flutter run --dart-define=API_BASE_URL=http://192.168.1.10:8080`
  static String get baseUrl {
    const override = String.fromEnvironment('API_BASE_URL');
    if (override.isNotEmpty) {
      return override;
    }

    if (kIsWeb) {
      // The browser runs on the host, so the same address the API binds to is
      // the one it can reach.
      return 'http://localhost:8080';
    }

    switch (defaultTargetPlatform) {
      case TargetPlatform.android:
        // Inside the emulator the host is reachable only through 10.0.2.2;
        // localhost refers to the emulated device itself.
        return 'http://10.0.2.2:8080';
      case TargetPlatform.iOS:
        // The simulator shares the host's network stack.
        return 'http://localhost:8080';
      case TargetPlatform.macOS:
      case TargetPlatform.windows:
      case TargetPlatform.linux:
      case TargetPlatform.fuchsia:
        return 'http://localhost:8080';
    }
  }

  /// How many notes one page holds.
  ///
  /// The server defaults to 50 and caps at 200. The cap is respected here so a
  /// paging bug cannot provoke a server-side clamp that silently changes the
  /// page size mid-scroll.
  static const int pageSize = 50;

  /// Largest page the server will serve.
  static const int maxPageSize = 200;

  /// Minimum and maximum title length, counted in runes to match the server.
  ///
  /// The server measures in runes so Cyrillic titles are judged fairly; Dart's
  /// `String.length` counts UTF-16 code units, which would let a title of 100
  /// emoji through validation and have it rejected on save.
  static const int minTitleLength = 3;
  static const int maxTitleLength = 100;
}
