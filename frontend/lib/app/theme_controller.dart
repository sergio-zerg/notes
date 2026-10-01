import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/storage/theme_preference_store.dart';

/// The user's theme choice.
///
/// [ThemeMode.system] is the default, so a first-run user gets whatever their
/// device is already set to — which is the answer most people would have
/// chosen, and the one platform guidance asks for. An explicit choice then
/// overrides the system until it is changed again.
class ThemeController extends Notifier<ThemeMode> {
  @override
  ThemeMode build() {
    // Reading is asynchronous, so the first frame uses the system setting and
    // repaints if a stored choice differs. Waiting for the read before painting
    // would mean a blank frame or a splash on every cold start.
    _restore();
    return ThemeMode.system;
  }

  Future<void> _restore() async {
    final stored = await ref.read(themePreferenceStoreProvider).read();
    if (stored != null && stored != state) {
      state = stored;
    }
  }

  /// Sets the mode explicitly and remembers it.
  Future<void> setMode(ThemeMode mode) async {
    state = mode;
    await ref.read(themePreferenceStoreProvider).write(mode);
  }

  /// Cycles light → dark → system.
  ///
  /// Returning to `system` is deliberate rather than a two-state toggle: a user
  /// who picked a theme by accident needs a way back to following the device,
  /// and a single toggle would trap them in one.
  Future<void> cycle() async {
    const order = [ThemeMode.light, ThemeMode.dark, ThemeMode.system];
    final next = order[(order.indexOf(state) + 1) % order.length];
    await setMode(next);
  }
}

final themePreferenceStoreProvider = Provider<ThemePreferenceStore>(
  (ref) => ThemePreferenceStore(),
);

final themeControllerProvider = NotifierProvider<ThemeController, ThemeMode>(
  ThemeController.new,
);

/// The icon representing a mode.
IconData themeModeIcon(ThemeMode mode) => switch (mode) {
  ThemeMode.light => Icons.light_mode_rounded,
  ThemeMode.dark => Icons.dark_mode_rounded,
  ThemeMode.system => Icons.brightness_auto_rounded,
};

/// The name of a mode, for a tooltip or an accessibility label.
String themeModeLabel(ThemeMode mode) => switch (mode) {
  ThemeMode.light => 'Light theme',
  ThemeMode.dark => 'Dark theme',
  ThemeMode.system => 'System theme',
};
