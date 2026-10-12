import 'package:flutter/material.dart';

/// Pre-permission explainer shown once before the OS push dialog (LEN-136).
/// Store review requires the reason a permission is needed to be clear
/// before the native prompt appears; the native dialog itself can't carry
/// custom copy, so the app shows this sheet first. Resolves true when the
/// user wants to continue to the system prompt.
Future<bool> showPushPermissionRationale(BuildContext context) async {
  final proceed = await showDialog<bool>(
    context: context,
    builder: (context) => AlertDialog(
      title: const Text('Stay in the loop'),
      content: const Text(
        "Lena's Kitchen requires notification permissions to alert you "
        'about real-time dashboard updates and account activity.',
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.of(context).pop(false),
          child: const Text('Not now'),
        ),
        FilledButton(
          onPressed: () => Navigator.of(context).pop(true),
          child: const Text('Enable notifications'),
        ),
      ],
    ),
  );
  // A dismissed sheet (barrier tap/back) counts as "not now" — never
  // prompt the OS dialog without an explicit continue.
  return proceed ?? false;
}
