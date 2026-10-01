import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'router.dart';
import 'theme.dart';
import 'theme_controller.dart';

/// The application root.
///
/// [MaterialApp.router] rather than `MaterialApp` because navigation is handled
/// by go_router, which redirects between the login screen and the notes list as
/// the session changes.
///
/// Both themes are supplied and [MaterialApp.themeMode] picks between them, so
/// switching repaints the existing tree instead of rebuilding the app and losing
/// the navigation stack.
class NotesApp extends ConsumerWidget {
  const NotesApp({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final router = ref.watch(routerProvider);
    final themeMode = ref.watch(themeControllerProvider);

    return MaterialApp.router(
      title: 'Notes',
      debugShowCheckedModeBanner: false,
      theme: AppTheme.light(),
      darkTheme: AppTheme.dark(),
      themeMode: themeMode,
      routerConfig: router,
    );
  }
}
