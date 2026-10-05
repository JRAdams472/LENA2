import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import 'auth_service.dart';
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
        return auth.isSignedIn ? const MainScreen() : const LoginScreen();
      },
    );
  }
}
