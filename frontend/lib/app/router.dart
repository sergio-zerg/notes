import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../features/auth/application/auth_controller.dart';
import '../features/auth/presentation/login_screen.dart';
import '../features/notes/presentation/note_editor_screen.dart';
import '../features/notes/presentation/notes_list_screen.dart';

/// The routes, in one place.
///
/// The session decides which one is reachable: an unauthenticated visitor is
/// redirected to the login screen, and signing out replaces the stack so the
/// back button cannot walk into a screen that needs a key.
final routerProvider = Provider<GoRouter>((ref) {
  // go_router re-runs the redirect whenever the listenable fires, so the
  // session is read inside the callback rather than captured here. Watching it
  // here as well would rebuild the router on every state change and throw away
  // the navigation stack.
  final router = GoRouter(
    initialLocation: '/notes',
    refreshListenable: _AuthRefresh(ref),
    redirect: (context, state) {
      final isAuthenticated =
          ref.read(authControllerProvider).status == AuthStatus.authenticated;

      final goingToLogin = state.matchedLocation == '/login';

      if (!isAuthenticated) {
        // Still reading the keystore: redirecting now would flash the login
        // screen on every cold start.
        if (ref.read(authControllerProvider).status == AuthStatus.unknown) {
          return null;
        }
        return goingToLogin ? null : '/login';
      }

      return goingToLogin ? '/notes' : null;
    },
    routes: [
      GoRoute(path: '/login', builder: (context, state) => const LoginScreen()),
      GoRoute(
        path: '/notes',
        builder: (context, state) => const NotesListScreen(),
        routes: [
          GoRoute(
            path: 'new',
            builder: (context, state) => const NoteEditorScreen(),
          ),
          GoRoute(
            path: ':id',
            builder: (context, state) => NoteEditorScreen(
              noteId: int.tryParse(state.pathParameters['id'] ?? ''),
            ),
          ),
        ],
      ),
    ],
  );

  ref.onDispose(router.dispose);

  return router;
});

/// Bridges Riverpod state changes into go_router's `refreshListenable`.
class _AuthRefresh extends ChangeNotifier {
  _AuthRefresh(this._ref) {
    _ref.listen(authControllerProvider, (_, _) => notifyListeners());
  }

  final Ref _ref;
}
