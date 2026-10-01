import 'package:flutter/material.dart';

import '../../../app/theme.dart';
import '../config/app_config.dart';

/// The API base URL, shown so a connection failure is easy to diagnose.
///
/// A wrong address is the most common first-run problem, and an opaque "cannot
/// reach the server" with no address gives the user nothing to act on.
///
/// This lives in core rather than beside either screen because both the login
/// form and the notes list need it, and neither feature owns the network's
/// address. Defining it in the login screen forced the list to import that
/// screen for a widget it never displayed.
class ApiEndpointHint extends StatelessWidget {
  const ApiEndpointHint({super.key});

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(top: AppTokens.sm),
      child: Text(
        AppConfig.baseUrl,
        style: Theme.of(context).textTheme.bodySmall?.copyWith(
              color: Theme.of(context).colorScheme.onSurfaceVariant,
            ),
        textAlign: TextAlign.center,
      ),
    );
  }
}