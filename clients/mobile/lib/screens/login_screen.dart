import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import '../auth/auth_service.dart';
import '../theme.dart';

/// Split brand card, vertical for mobile — the sage panel carries the
/// wordmark and tagline, the paper panel carries the sign-in actions.
/// Mirrors the web LoginScreen layout (LEN-76).
///
/// Store-compliance copy (LEN-136): the app is login-only — accounts are
/// created on the website, so the footer is plain non-clickable text (a
/// hyperlink here breaks store review). Provider buttons are stacked
/// vertically at equal prominence; Apple is required by App Store review
/// whenever other social providers are offered.
///
/// Dead-UI guard (LEN-154): only configured providers render. Apple shows
/// on iOS/macOS only (no native Android path); Discord/Facebook show only
/// when their client ids are compiled in — a button that errors on tap
/// fails store review for non-functionality.
class LoginScreen extends StatelessWidget {
  const LoginScreen({super.key});

  bool get _showApple =>
      defaultTargetPlatform == TargetPlatform.iOS ||
      defaultTargetPlatform == TargetPlatform.macOS;

  @override
  Widget build(BuildContext context) {
    final auth = context.watch<AuthService>();
    final theme = Theme.of(context);
    final providers = <Widget>[
      if (_showApple)
        _ProviderButton(
          label: 'Continue with Apple',
          background: Colors.black,
          foreground: Colors.white,
          icon: const Icon(Icons.apple, size: 20),
          onPressed: () => auth.signInWithProvider('Apple'),
        ),
      _ProviderButton(
        label: 'Continue with Google',
        background: Colors.white,
        foreground: Colors.black87,
        side: const BorderSide(color: Colors.black26),
        icon: const _BrandLetter('G', Color(0xFF4285F4)),
        onPressed: auth.signIn,
      ),
      if (auth.isProviderConfigured('discord'))
        _ProviderButton(
          label: 'Continue with Discord',
          background: const Color(0xFF5865F2),
          foreground: Colors.white,
          icon: const _BrandLetter('D', Color(0xFF5865F2)),
          onPressed: () => auth.signInWithProvider('Discord'),
        ),
      if (auth.isProviderConfigured('microsoft'))
        _ProviderButton(
          label: 'Continue with Microsoft',
          background: const Color(0xFF2F2F2F),
          foreground: Colors.white,
          icon: const _BrandLetter('⊞', Color(0xFF00A4EF)),
          onPressed: () => auth.signInWithProvider('Microsoft'),
        ),
      if (auth.isProviderConfigured('facebook'))
        _ProviderButton(
          label: 'Continue with Facebook',
          background: const Color(0xFF1877F2),
          foreground: Colors.white,
          icon: const Icon(Icons.facebook, size: 20),
          onPressed: () => auth.signInWithProvider('Facebook'),
        ),
    ];
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
                  // Brand panel — deep sage holds up in both schemes.
                  Container(
                    width: double.infinity,
                    color: lenaSageDark,
                    padding: const EdgeInsets.symmetric(
                      horizontal: 24,
                      vertical: 36,
                    ),
                    child: Column(
                      children: [
                        Container(
                          width: 56,
                          height: 56,
                          decoration: BoxDecoration(
                            color: lenaPaper.withValues(alpha: 0.16),
                            borderRadius: BorderRadius.circular(16),
                          ),
                          child: const Center(
                            child: Text(
                              'L',
                              style: TextStyle(
                                fontFamily: 'Nunito',
                                fontSize: 30,
                                fontWeight: FontWeight.w800,
                                color: lenaPaper,
                              ),
                            ),
                          ),
                        ),
                        const SizedBox(height: 14),
                        const Text(
                          'LENA',
                          style: TextStyle(
                            fontFamily: 'Nunito',
                            fontSize: 22,
                            fontWeight: FontWeight.w800,
                            letterSpacing: 3,
                            color: lenaPaper,
                          ),
                        ),
                        const SizedBox(height: 8),
                        Text(
                          'The household kitchen, kept — pantry, '
                          'recipes, meal plans, and wine in one place.',
                          textAlign: TextAlign.center,
                          style: TextStyle(
                            fontFamily: 'Nunito',
                            fontSize: 13,
                            color: lenaPaper.withValues(alpha: 0.82),
                          ),
                        ),
                      ],
                    ),
                  ),
                  // Sign-in panel.
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
                          'Welcome to Lena',
                          style: theme.textTheme.titleLarge,
                        ),
                        const SizedBox(height: 4),
                        Text(
                          'Sign in with the provider associated with your '
                          'web account to access your dashboard and '
                          'configure notifications.',
                          textAlign: TextAlign.center,
                          style: theme.textTheme.bodySmall,
                        ),
                        const SizedBox(height: 20),
                        if (auth.lastError != null)
                          Padding(
                            padding: const EdgeInsets.only(bottom: 12),
                            child: Text(
                              auth.lastError!,
                              style: TextStyle(
                                color: theme.colorScheme.error,
                              ),
                              textAlign: TextAlign.center,
                            ),
                          ),
                        if (auth.isLoading)
                          const Center(child: CircularProgressIndicator())
                        else
                          for (final b in providers) ...[
                            b,
                            const SizedBox(height: 10),
                          ],
                        const SizedBox(height: 16),
                        // Registration is web-only — plain text on purpose:
                        // a tappable link here fails store review (LEN-136).
                        Text(
                          'Account registration and plan management are '
                          'handled on our website. New users must set up '
                          'their accounts on lenaskitchen.app before '
                          'logging in to the mobile companion app.',
                          textAlign: TextAlign.center,
                          style: theme.textTheme.bodySmall?.copyWith(
                            color: theme.colorScheme.onSurface
                                .withValues(alpha: 0.6),
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

/// One provider sign-in button — all share the same size and weight so
/// Apple sits at equal prominence per App Store review guidelines.
class _ProviderButton extends StatelessWidget {
  const _ProviderButton({
    required this.label,
    required this.background,
    required this.foreground,
    required this.icon,
    required this.onPressed,
    this.side = BorderSide.none,
  });

  final String label;
  final Color background;
  final Color foreground;
  final Widget icon;
  final VoidCallback onPressed;
  final BorderSide side;

  @override
  Widget build(BuildContext context) {
    return SizedBox(
      width: double.infinity,
      child: ElevatedButton.icon(
        onPressed: onPressed,
        icon: icon,
        label: Text(label),
        style: ElevatedButton.styleFrom(
          backgroundColor: background,
          foregroundColor: foreground,
          shape: RoundedRectangleBorder(
            borderRadius: BorderRadius.circular(12),
            side: side,
          ),
        ),
      ),
    );
  }
}

/// Simple lettered brand glyph used where Material has no logo icon
/// (Google's 'G', Discord's mark). White disc so it reads on colored
/// buttons.
class _BrandLetter extends StatelessWidget {
  const _BrandLetter(this.letter, this.color);

  final String letter;
  final Color color;

  @override
  Widget build(BuildContext context) {
    return Container(
      width: 20,
      height: 20,
      decoration: const BoxDecoration(
        color: Colors.white,
        shape: BoxShape.circle,
      ),
      alignment: Alignment.center,
      child: Text(
        letter,
        style: TextStyle(
          fontSize: 12,
          fontWeight: FontWeight.w800,
          color: color,
          height: 1,
        ),
      ),
    );
  }
}
