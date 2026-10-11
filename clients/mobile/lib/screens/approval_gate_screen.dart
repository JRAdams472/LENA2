import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import '../auth/auth_service.dart';
import '../theme.dart';

/// Full-screen gate for accounts held at the admin-approval barrier
/// (LEN-127). Pending accounts see "access request sent"; denied
/// accounts see a blocking message. Mirrors the web ApprovalGate.
class ApprovalGateScreen extends StatelessWidget {
  const ApprovalGateScreen({super.key, this.denied = false});

  final bool denied;

  @override
  Widget build(BuildContext context) {
    final auth = context.watch<AuthService>();
    final theme = Theme.of(context);
    return Scaffold(
      body: Center(
        child: SingleChildScrollView(
          padding: const EdgeInsets.all(24),
          child: ConstrainedBox(
            constraints: const BoxConstraints(maxWidth: 380),
            child: ClipRRect(
              borderRadius: BorderRadius.circular(20),
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  Container(
                    width: double.infinity,
                    color: lenaSageDark,
                    padding: const EdgeInsets.symmetric(
                      horizontal: 24,
                      vertical: 36,
                    ),
                    child: Column(
                      children: [
                        Icon(
                          denied ? Icons.block : Icons.hourglass_top,
                          size: 48,
                          color: lenaPaper,
                        ),
                        const SizedBox(height: 14),
                        Text(
                          denied ? 'Access not granted' : 'Access request sent',
                          textAlign: TextAlign.center,
                          style: const TextStyle(
                            fontFamily: 'Nunito',
                            fontSize: 20,
                            fontWeight: FontWeight.w800,
                            color: lenaPaper,
                          ),
                        ),
                      ],
                    ),
                  ),
                  Container(
                    width: double.infinity,
                    color: theme.colorScheme.surface,
                    padding: const EdgeInsets.symmetric(
                      horizontal: 24,
                      vertical: 28,
                    ),
                    child: Column(
                      children: [
                        Text(
                          denied
                              ? 'An administrator has declined this account\u2019s '
                                  'request for access. If you believe this is a '
                                  'mistake, contact your household administrator.'
                              : 'An administrator will review your request '
                                  'before you can use LENA. You will gain access '
                                  'once approved \u2014 check back soon.',
                          textAlign: TextAlign.center,
                          style: theme.textTheme.bodySmall,
                        ),
                        const SizedBox(height: 20),
                        SizedBox(
                          width: double.infinity,
                          child: OutlinedButton(
                            onPressed: auth.signOut,
                            child: const Text('Sign out'),
                          ),
                        ),
                      ],
                    ),
                  ),
                ],
              ),
            ),
          ),
        ),
      ),
    );
  }
}
