import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

// Warm sage/cream palette shared with the web client
// (clients/web/app/providers.tsx). Accent list also feeds dashboard
// suggestion tiles.
const lenaSage = Color(0xFF7C9473);
const lenaSageLight = Color(0xFFA4B79C);
const lenaSageDark = Color(0xFF5F7A57);
const lenaCream = Color(0xFFFAF6EF);
const lenaPaper = Color(0xFFFFFDF8);
const lenaOlive = Color(0xFF8A9A5B);
const lenaOliveDark = Color(0xFF6E7B45);
const lenaOliveLight = Color(0xFFA9B87F);
const lenaDivider = Color(0xFFE5DFD3);
const lenaInk = Color(0xFF3E3E34);
const lenaInkMuted = Color(0xFF6B6B5E);
const lenaTerracotta = Color(0xFFC0876B);
const lenaWheat = Color(0xFFD9B26A);

// Dark scheme — docs/design.md v2. Warm charcoal surfaces (no pure
// black/white), sage lifted for AA contrast, hairline borders carry
// elevation in place of shadows.
const lenaDarkCanvas = Color(0xFF211F1A);
const lenaDarkPaper = Color(0xFF2B2922);
const lenaDarkSage = Color(0xFF93A88B);
const lenaDarkSageLight = Color(0xFFAFC2A8);
const lenaDarkDivider = Color(0xFF3A372E);
const lenaDarkInk = Color(0xFFEFEAE0);
const lenaDarkInkMuted = Color(0xFFB5AE9D);
const lenaDarkTerracotta = Color(0xFFD09B7F);
const lenaDarkWheat = Color(0xFFE3C584);
const lenaDarkError = Color(0xFFDA8672);

// Deterministic accent per recipe so suggestion tiles differentiate
// without real photos: sage, olive, terracotta, wheat.
const lenaAccents = [lenaSage, lenaOlive, lenaTerracotta, lenaWheat];
const lenaDarkAccents = [
  lenaDarkSage,
  lenaOliveLight,
  lenaDarkTerracotta,
  lenaDarkWheat,
];

/// Accent list for the active brightness — suggestion tiles, badges.
List<Color> lenaAccentsFor(Brightness brightness) =>
    brightness == Brightness.dark ? lenaDarkAccents : lenaAccents;

ThemeData lenaTheme() => _lenaTheme(dark: false);

ThemeData lenaDarkTheme() => _lenaTheme(dark: true);

ThemeData _lenaTheme({required bool dark}) {
  final canvas = dark ? lenaDarkCanvas : lenaCream;
  final paper = dark ? lenaDarkPaper : lenaPaper;
  final ink = dark ? lenaDarkInk : lenaInk;
  final muted = dark ? lenaDarkInkMuted : lenaInkMuted;
  final divider = dark ? lenaDarkDivider : lenaDivider;
  final sage = dark ? lenaDarkSage : lenaSage;
  final sageText = dark ? lenaDarkSageLight : lenaSageDark;

  final scheme = ColorScheme(
    brightness: dark ? Brightness.dark : Brightness.light,
    primary: sage,
    onPrimary: dark ? lenaDarkCanvas : lenaPaper,
    primaryContainer: dark ? const Color(0xFF3A4530) : const Color(0xFFE4EBDF),
    onPrimaryContainer: sageText,
    secondary: dark ? lenaOliveLight : lenaOlive,
    onSecondary: dark ? lenaDarkCanvas : lenaPaper,
    secondaryContainer:
        dark ? const Color(0xFF42462F) : const Color(0xFFEAEEDD),
    onSecondaryContainer: dark ? const Color(0xFFC9D3A4) : lenaOliveDark,
    tertiary: dark ? lenaDarkTerracotta : lenaTerracotta,
    onTertiary: dark ? lenaDarkCanvas : lenaPaper,
    surface: paper,
    onSurface: ink,
    surfaceContainerHighest:
        dark ? const Color(0xFF35322A) : const Color(0xFFF1EBDF),
    onSurfaceVariant: muted,
    outline: divider,
    outlineVariant: dark ? const Color(0xFF322F27) : const Color(0xFFEFE9DC),
    error: dark ? lenaDarkError : const Color(0xFFB3543F),
    onError: dark ? lenaDarkCanvas : lenaPaper,
  );

  return ThemeData(
    useMaterial3: true,
    colorScheme: scheme,
    scaffoldBackgroundColor: canvas,
    fontFamily: 'Nunito',
    textTheme: TextTheme(
      headlineMedium: TextStyle(fontWeight: FontWeight.w700, color: ink),
      headlineSmall: TextStyle(fontWeight: FontWeight.w700, color: ink),
      titleLarge: TextStyle(fontWeight: FontWeight.w700, color: ink),
      titleMedium: TextStyle(fontWeight: FontWeight.w600, color: ink),
      titleSmall: TextStyle(fontWeight: FontWeight.w600, color: ink),
      bodyLarge: TextStyle(color: ink),
      bodyMedium: TextStyle(color: ink),
      bodySmall: TextStyle(color: muted),
      labelSmall: TextStyle(color: muted, letterSpacing: 1.1),
    ),
    // The sage band stays the brand anchor in both schemes.
    appBarTheme: const AppBarTheme(
      backgroundColor: lenaSage,
      foregroundColor: lenaPaper,
      elevation: 0,
      scrolledUnderElevation: 0,
      centerTitle: false,
      systemOverlayStyle: SystemUiOverlayStyle(
        statusBarColor: Colors.transparent,
        statusBarIconBrightness: Brightness.light,
        statusBarBrightness: Brightness.dark,
      ),
      titleTextStyle: TextStyle(
        fontFamily: 'Nunito',
        fontSize: 20,
        fontWeight: FontWeight.w700,
        color: lenaPaper,
      ),
    ),
    cardTheme: CardThemeData(
      color: paper,
      surfaceTintColor: Colors.transparent,
      // Hairline borders carry elevation in dark mode — shadows on
      // near-black surfaces just smear.
      elevation: dark ? 0 : 1,
      shadowColor: const Color(0x0F3E3E34),
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(12),
        side: dark ? BorderSide(color: divider) : BorderSide.none,
      ),
      margin: const EdgeInsets.symmetric(vertical: 4),
    ),
    dividerTheme: DividerThemeData(color: divider, thickness: 1),
    dividerColor: divider,
    bottomNavigationBarTheme: BottomNavigationBarThemeData(
      backgroundColor: paper,
      selectedItemColor: sageText,
      unselectedItemColor: muted,
      type: BottomNavigationBarType.fixed,
      selectedLabelStyle:
          const TextStyle(fontWeight: FontWeight.w600, fontSize: 12),
      unselectedLabelStyle: const TextStyle(fontSize: 12),
    ),
    navigationBarTheme: NavigationBarThemeData(
      backgroundColor: paper,
      indicatorColor: sage.withValues(alpha: 0.14),
      labelTextStyle: WidgetStateProperty.resolveWith(
        (states) => TextStyle(
          fontSize: 12,
          fontWeight: states.contains(WidgetState.selected)
              ? FontWeight.w600
              : FontWeight.w400,
          color: states.contains(WidgetState.selected) ? sageText : muted,
        ),
      ),
      iconTheme: WidgetStateProperty.resolveWith(
        (states) => IconThemeData(
          color: states.contains(WidgetState.selected) ? sageText : muted,
        ),
      ),
    ),
    chipTheme: ChipThemeData(
      backgroundColor: paper,
      selectedColor: sage.withValues(alpha: 0.18),
      side: BorderSide(color: divider),
      labelStyle: TextStyle(color: ink, fontWeight: FontWeight.w500),
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(8)),
    ),
    elevatedButtonTheme: ElevatedButtonThemeData(
      style: ElevatedButton.styleFrom(
        backgroundColor: sage,
        foregroundColor: dark ? lenaDarkCanvas : lenaPaper,
        elevation: 0,
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(10)),
      ),
    ),
    textButtonTheme: TextButtonThemeData(
      style: TextButton.styleFrom(foregroundColor: sageText),
    ),
    outlinedButtonTheme: OutlinedButtonThemeData(
      style: OutlinedButton.styleFrom(
        foregroundColor: sageText,
        side: BorderSide(color: sage),
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(10)),
      ),
    ),
    floatingActionButtonTheme: FloatingActionButtonThemeData(
      backgroundColor: sage,
      foregroundColor: dark ? lenaDarkCanvas : lenaPaper,
    ),
    inputDecorationTheme: InputDecorationTheme(
      filled: true,
      fillColor: paper,
      // Async-populated fields (edit screens) never re-float the label
      // after the value lands — always-float keeps labels off the text.
      floatingLabelBehavior: FloatingLabelBehavior.always,
      border: OutlineInputBorder(
        borderRadius: BorderRadius.circular(10),
        borderSide: BorderSide(color: divider),
      ),
      enabledBorder: OutlineInputBorder(
        borderRadius: BorderRadius.circular(10),
        borderSide: BorderSide(color: divider),
      ),
      focusedBorder: OutlineInputBorder(
        borderRadius: BorderRadius.circular(10),
        borderSide: BorderSide(color: sage, width: 1.5),
      ),
    ),
    progressIndicatorTheme: ProgressIndicatorThemeData(color: sage),
    snackBarTheme: SnackBarThemeData(
      behavior: SnackBarBehavior.floating,
      backgroundColor: ink,
      contentTextStyle: TextStyle(fontFamily: 'Nunito', color: paper),
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(10)),
    ),
    tabBarTheme: TabBarThemeData(
      labelColor: sageText,
      unselectedLabelColor: muted,
      indicatorColor: sage,
    ),
    listTileTheme: ListTileThemeData(
      iconColor: muted,
      textColor: ink,
    ),
  );
}
