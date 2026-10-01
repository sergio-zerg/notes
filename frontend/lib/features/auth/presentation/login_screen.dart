import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../app/theme.dart';
import '../../../app/theme_controller.dart';
import '../../../core/network/dio_provider.dart';
import '../../../core/widgets/theme_toggle.dart';
import '../application/auth_controller.dart';

/// Asks for the API key and verifies it against the server before storing it.
///
/// Verification is a request rather than a local check: the server is the only
/// party that knows whether a key is still configured, and a key revoked
/// server-side would otherwise be accepted here and fail every later call.
class LoginScreen extends ConsumerStatefulWidget {
  const LoginScreen({super.key});

  @override
  ConsumerState<LoginScreen> createState() => _LoginScreenState();
}

class _LoginScreenState extends ConsumerState<LoginScreen> {
  final _formKey = GlobalKey<FormState>();
  final _controller = TextEditingController();
  final _focusNode = FocusNode();

  @override
  void dispose() {
    _controller.dispose();
    _focusNode.dispose();
    super.dispose();
  }

  Future<void> _submit() async {
    if (!(_formKey.currentState?.validate() ?? false)) return;

    // Focus is dropped first so the keyboard does not cover the outcome, which
    // matters on a phone where this form fills much of the screen.
    _focusNode.unfocus();

    await ref.read(authControllerProvider.notifier).signIn(
          _controller.text,
          verify: _verifyKey,
        );
  }

  /// Confirms the key by listing notes, the cheapest authenticated call there is.
  ///
  /// An empty list is a success: the key works, there is simply nothing to show.
  Future<KeyVerdict> _verifyKey() async {
    try {
      await ref.read(dioProvider).get<List<dynamic>>(
            '/notes',
            queryParameters: const {'limit': 1},
          );
      return KeyVerdict.accepted;
    } on DioException catch (error) {
      // Only 401 proves the key is wrong. A timeout, a refused connection or a
      // 500 says nothing about the key: treating those as success signed the
      // user in to an app that could not load anything, and treating them as a
      // rejection would blame their key for the server's outage.
      return switch (error.response?.statusCode) {
        401 || 403 => KeyVerdict.rejected,
        _ => KeyVerdict.undecided,
      };
    }
  }

  @override
  Widget build(BuildContext context) {
    final authState = ref.watch(authControllerProvider);
    final theme = Theme.of(context);

    return Scaffold(
      body: DecoratedBox(
        // A soft gradient behind the card so the form lifts off the background
        // without a drop shadow, which flattens badly on a dark surface.
        decoration: BoxDecoration(
          gradient: LinearGradient(
            begin: Alignment.topLeft,
            end: Alignment.bottomRight,
            colors: AppTheme.isLight(theme)
                ? [
                    theme.colorScheme.primaryContainer.withValues(alpha: 0.55),
                    theme.colorScheme.surface,
                  ]
                : [
                    theme.colorScheme.surface,
                    theme.colorScheme.surfaceContainerHighest,
                  ],
          ),
        ),
        child: SafeArea(
          child: Stack(
            children: [
              // Pinned to the corner and out of the flow, so the control adds
              // nothing to the height of a form that has to fit on a phone.
              Positioned(
                top: AppTokens.xs,
                right: AppTokens.xs,
                child: ThemeToggle(
                  mode: ref.watch(themeControllerProvider),
                  onPressed:
                      () => ref.read(themeControllerProvider.notifier).cycle(),
                ),
              ),
              Center(
                child: SingleChildScrollView(
                  padding: const EdgeInsets.all(AppTokens.xl),
                  child: ConstrainedBox(
                    constraints:
                        const BoxConstraints(maxWidth: AppTokens.maxFormWidth),
                    child: Column(
                      mainAxisSize: MainAxisSize.min,
                      crossAxisAlignment: CrossAxisAlignment.stretch,
                      children: [
                        const _BrandMark(),
                        const SizedBox(height: AppTokens.xl),
                        Text(
                          'Notes',
                          style: theme.textTheme.headlineMedium,
                          textAlign: TextAlign.center,
                        ),
                        const SizedBox(height: AppTokens.sm),
                        Text(
                          'Sign in with the API key for your server',
                          style: theme.textTheme.bodyMedium?.copyWith(
                            color: theme.colorScheme.onSurfaceVariant,
                          ),
                          textAlign: TextAlign.center,
                        ),
                        const SizedBox(height: AppTokens.xxl),
                        _Card(
                          child: Form(
                            key: _formKey,
                            child: Column(
                              crossAxisAlignment: CrossAxisAlignment.stretch,
                              children: [
                                TextFormField(
                                  controller: _controller,
                                  focusNode: _focusNode,
                                  // Obscured, with autocorrect and suggestions
                                  // off: the key is a shared secret, and a keyboard
                                  // dictionary must not capture it.
                                  obscureText: true,
                                  autocorrect: false,
                                  enableSuggestions: false,
                                  autofillHints: const [AutofillHints.password],
                                  textInputAction: TextInputAction.go,
                                  onFieldSubmitted: (_) => _submit(),
                                  decoration: const InputDecoration(
                                    labelText: 'API key',
                                    prefixIcon: Icon(Icons.key_outlined),
                                  ),
                                  validator: (value) =>
                                      (value == null || value.trim().isEmpty)
                                          ? 'Enter your API key'
                                          : null,
                                ),
                                AnimatedSize(
                                  duration: AppTokens.medium,
                                  alignment: Alignment.topCenter,
                                  child: authState.error == null
                                      ? const SizedBox(width: double.infinity)
                                      : Padding(
                                          padding: const EdgeInsets.only(
                                            top: AppTokens.md,
                                          ),
                                          child: _ErrorBanner(
                                            message: authState.error!,
                                          ),
                                        ),
                                ),
                                const SizedBox(height: AppTokens.xl),
                                FilledButton(
                                  onPressed: authState.isSubmitting ? null : _submit,
                                  child: authState.isSubmitting
                                      ? const SizedBox.square(
                                          dimension: 20,
                                          child: CircularProgressIndicator(
                                            strokeWidth: 2,
                                          ),
                                        )
                                      : const Text('Sign in'),
                                ),
                              ],
                            ),
                          ),
                        ),
                        const SizedBox(height: AppTokens.xl),
                        Text(
                          'The key is kept in your device keystore and sent as '
                          'X-API-Key on every request.',
                          style: theme.textTheme.bodySmall?.copyWith(
                            color: theme.colorScheme.onSurfaceVariant,
                          ),
                          textAlign: TextAlign.center,
                        ),
                      ],
                    ),
                  ),
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

/// The logo mark: a rounded square holding a glyph from the brand gradient.
class _BrandMark extends StatelessWidget {
  const _BrandMark();

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Container(
        width: 72,
        height: 72,
        decoration: BoxDecoration(
          gradient: LinearGradient(
            begin: Alignment.topLeft,
            end: Alignment.bottomRight,
            colors: AppTheme.brandGradient,
          ),
          borderRadius: BorderRadius.circular(AppTokens.radiusXl),
        ),
        child: const Icon(
          Icons.auto_stories_rounded,
          color: Colors.white,
          size: 34,
        ),
      ),
    );
  }
}

/// The form panel.
class _Card extends StatelessWidget {
  const _Card({required this.child});

  final Widget child;

  @override
  Widget build(BuildContext context) {
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(AppTokens.xl),
        child: child,
      ),
    );
  }
}

/// A failure that resizes its own slot.
///
/// Animated so an error appearing does not shove the button down abruptly, and
/// absent rather than empty when there is nothing wrong.
class _ErrorBanner extends StatelessWidget {
  const _ErrorBanner({required this.message});

  final String message;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).colorScheme;

    return Container(
      padding: const EdgeInsets.all(AppTokens.md),
      decoration: BoxDecoration(
        color: colors.errorContainer,
        borderRadius: BorderRadius.circular(AppTokens.radiusMd),
      ),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Icon(Icons.error_outline, size: 18, color: colors.onErrorContainer),
          const SizedBox(width: AppTokens.sm + 2),
          Expanded(
            child: Text(
              message,
              style: Theme.of(context).textTheme.bodySmall?.copyWith(
                    color: colors.onErrorContainer,
                    height: 1.4,
                  ),
            ),
          ),
        ],
      ),
    );
  }
}
