import 'package:flutter/material.dart';
import 'package:shared_preferences/shared_preferences.dart';

/// Persists the user's theme choice between launches.
///
/// `SharedPreferences` rather than the secure storage the API key uses: a theme
/// is not a secret, and putting it behind the keystore would mean unlocking the
/// device's secure hardware to read a colour.
class ThemePreferenceStore {
  // The parameter is named `instance` rather than `preferences` because a
  // private field cannot take an initializing formal.
  ThemePreferenceStore({SharedPreferences? instance})
    : _preferences = instance;

  SharedPreferences? _preferences;

  static const _key = 'notes_theme_mode';

  /// Cached after the first read, because every toggle would otherwise hit
  /// storage twice.
  SharedPreferences? get _cached => _preferences;

  /// Returns the stored choice, or null when the user has never chosen one.
  Future<ThemeMode?> read() async {
    try {
      final preferences = await SharedPreferences.getInstance();
      _preferences = preferences;

      final stored = preferences.getString(_key);
      return stored == null ? null : _parse(stored);
    } catch (_) {
      // A preference store that will not open must not stop the app: the
      // caller falls back to following the system setting.
      return null;
    }
  }

  /// Saves [mode], replacing any previous choice.
  Future<void> write(ThemeMode mode) async {
    try {
      final preferences = _cached ?? await SharedPreferences.getInstance();
      _preferences = preferences;
      await preferences.setString(_key, mode.name);
    } catch (_) {
      // Losing the preference across launches is an acceptable outcome; failing
      // to switch the theme now would not be.
    }
  }

  /// Maps the stored name back to a mode.
  ///
  /// Unknown values yield null rather than a guess: a value written by a future
  /// version must not crash this one, and falling back to the system setting is
  /// the safer reading of "unrecognised".
  static ThemeMode? _parse(String stored) {
    for (final mode in ThemeMode.values) {
      if (mode.name == stored) return mode;
    }
    return null;
  }
}
