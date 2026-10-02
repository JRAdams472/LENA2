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

// Deterministic accent per recipe so suggestion tiles differentiate
// without real photos: sage, olive, terracotta, wheat.
const lenaAccents = [lenaSage, lenaOlive, lenaTerracotta, lenaWheat];

ThemeData lenaTheme() {
  const scheme = ColorScheme(
    brightness: Brightness.light,
    primary: lenaSage,
    onPrimary: lenaPaper,
    primaryContainer: Color(0xFFE4EBDF),
    onPrimaryContainer: lenaSageDark,
    secondary: lenaOlive,
    onSecondary: lenaPaper,
    secondaryContainer: Color(0xFFEAEEDD),
    onSecondaryContainer: lenaOliveDark,
    tertiary: lenaTerracotta,
    onTertiary: lenaPaper,
    surface: lenaPaper,
    onSurface: lenaInk,
    surfaceContainerHighest: Color(0xFFF1EBDF),
    onSurfaceVariant: lenaInkMuted,
    outline: lenaDivider,
    outlineVariant: Color(0xFFEFE9DC),
    error: Color(0xFFB3543F),
    onError: lenaPaper,
  );

  return ThemeData(
    useMaterial3: true,
    colorScheme: scheme,
    scaffoldBackgroundColor: lenaCream,
    fontFamily: 'Nunito',
    textTheme: const TextTheme(
      headlineMedium: TextStyle(fontWeight: FontWeight.w700, color: lenaInk),
      headlineSmall: TextStyle(fontWeight: FontWeight.w700, color: lenaInk),
      titleLarge: TextStyle(fontWeight: FontWeight.w700, color: lenaInk),
      titleMedium: TextStyle(fontWeight: FontWeight.w600, color: lenaInk),
      titleSmall: TextStyle(fontWeight: FontWeight.w600, color: lenaInk),
      bodyLarge: TextStyle(color: lenaInk),
      bodyMedium: TextStyle(color: lenaInk),
      bodySmall: TextStyle(color: lenaInkMuted),
      labelSmall: TextStyle(color: lenaInkMuted, letterSpacing: 1.1),
    ),
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
      color: lenaPaper,
      surfaceTintColor: Colors.transparent,
      elevation: 1,
      shadowColor: const Color(0x0F3E3E34),
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
      margin: const EdgeInsets.symmetric(vertical: 4),
    ),
    dividerTheme: const DividerThemeData(color: lenaDivider, thickness: 1),
    dividerColor: lenaDivider,
    bottomNavigationBarTheme: const BottomNavigationBarThemeData(
      backgroundColor: lenaPaper,
      selectedItemColor: lenaSageDark,
      unselectedItemColor: lenaInkMuted,
      type: BottomNavigationBarType.fixed,
      selectedLabelStyle: TextStyle(fontWeight: FontWeight.w600, fontSize: 12),
      unselectedLabelStyle: TextStyle(fontSize: 12),
    ),
    navigationBarTheme: NavigationBarThemeData(
      backgroundColor: lenaPaper,
      indicatorColor: lenaSage.withValues(alpha: 0.14),
      labelTextStyle: WidgetStateProperty.resolveWith(
        (states) => TextStyle(
          fontSize: 12,
          fontWeight: states.contains(WidgetState.selected)
              ? FontWeight.w600
              : FontWeight.w400,
          color: states.contains(WidgetState.selected)
              ? lenaSageDark
              : lenaInkMuted,
        ),
      ),
      iconTheme: WidgetStateProperty.resolveWith(
        (states) => IconThemeData(
          color: states.contains(WidgetState.selected)
              ? lenaSageDark
              : lenaInkMuted,
        ),
      ),
    ),
    chipTheme: ChipThemeData(
      backgroundColor: lenaPaper,
      selectedColor: lenaSage.withValues(alpha: 0.18),
      side: const BorderSide(color: lenaDivider),
      labelStyle: const TextStyle(color: lenaInk, fontWeight: FontWeight.w500),
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(8)),
    ),
    elevatedButtonTheme: ElevatedButtonThemeData(
      style: ElevatedButton.styleFrom(
        backgroundColor: lenaSage,
        foregroundColor: lenaPaper,
        elevation: 0,
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(10)),
      ),
    ),
    textButtonTheme: TextButtonThemeData(
      style: TextButton.styleFrom(foregroundColor: lenaSageDark),
    ),
    outlinedButtonTheme: OutlinedButtonThemeData(
      style: OutlinedButton.styleFrom(
        foregroundColor: lenaSageDark,
        side: const BorderSide(color: lenaSage),
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(10)),
      ),
    ),
    floatingActionButtonTheme: const FloatingActionButtonThemeData(
      backgroundColor: lenaSage,
      foregroundColor: lenaPaper,
    ),
    inputDecorationTheme: InputDecorationTheme(
      filled: true,
      fillColor: lenaPaper,
      border: OutlineInputBorder(
        borderRadius: BorderRadius.circular(10),
        borderSide: const BorderSide(color: lenaDivider),
      ),
      enabledBorder: OutlineInputBorder(
        borderRadius: BorderRadius.circular(10),
        borderSide: const BorderSide(color: lenaDivider),
      ),
      focusedBorder: OutlineInputBorder(
        borderRadius: BorderRadius.circular(10),
        borderSide: const BorderSide(color: lenaSage, width: 1.5),
      ),
    ),
    progressIndicatorTheme: const ProgressIndicatorThemeData(color: lenaSage),
    snackBarTheme: SnackBarThemeData(
      behavior: SnackBarBehavior.floating,
      backgroundColor: lenaInk,
      contentTextStyle: const TextStyle(fontFamily: 'Nunito', color: lenaPaper),
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(10)),
    ),
    tabBarTheme: const TabBarThemeData(
      labelColor: lenaSageDark,
      unselectedLabelColor: lenaInkMuted,
      indicatorColor: lenaSage,
    ),
    listTileTheme: const ListTileThemeData(
      iconColor: lenaInkMuted,
      textColor: lenaInk,
    ),
  );
}
