import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import 'auth_service.dart';
import '../screens/approval_gate_screen.dart';
import '../screens/main_screen.dart';
import '../screens/login_screen.dart';
import '../widgets/skeleton.dart';

class AuthGate extends StatelessWidget {
  const AuthGate({super.key});

  @override
  Widget build(BuildContext context) {
    return Consumer<AuthService>(
      builder: (context, auth, child) {
        if (auth.isLoading) {
          return const LenaSplash();
        }
        // Approval barrier (LEN-127): a signed-in-but-gated account gets
        // the gate screen instead of member UI — no household data mounts.
        if (auth.accountGate == 'pending') {
          return const ApprovalGateScreen();
        }
        if (auth.accountGate == 'denied') {
          return const ApprovalGateScreen(denied: true);
        }
        return auth.isSignedIn ? const MainScreen() : const LoginScreen();
      },
    );
  }
}
