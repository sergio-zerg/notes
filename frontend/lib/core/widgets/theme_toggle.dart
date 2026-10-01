import 'package:flutter/material.dart';

import '../../app/theme_controller.dart';

/// The light/dark/system switch.
///
/// The icon shows the mode a tap would switch **to**, not the active one: the
/// control then answers "what happens if I press this" without needing a label
/// beside it, which matters in a header where there is no room for one.
class ThemeToggle extends StatelessWidget {
  const ThemeToggle({required this.mode, required this.onPressed, super.key});

  /// The mode currently in effect.
  final ThemeMode mode;

  final VoidCallback onPressed;

  /// The mode a press moves to.
  static ThemeMode nextMode(ThemeMode current) {
    const order = [ThemeMode.light, ThemeMode.dark, ThemeMode.system];
    return order[(order.indexOf(current) + 1) % order.length];
  }

  @override
  Widget build(BuildContext context) {
    final upcoming = nextMode(mode);

    return IconButton(
      onPressed: onPressed,
      icon: Icon(themeModeIcon(upcoming)),
      tooltip: 'Switch to ${themeModeLabel(upcoming).toLowerCase()}',
      style: IconButton.styleFrom(
        foregroundColor: Theme.of(context).colorScheme.onSurfaceVariant,
        // 44 rather than the default 48, so the header row keeps its rhythm on
        // a small phone without dropping below a comfortable tap target.
        minimumSize: const Size(44, 44),
      ),
    );
  }
}
