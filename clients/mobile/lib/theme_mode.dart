import 'package:flutter/material.dart';
import 'package:shared_preferences/shared_preferences.dart';

/// Light/dark/system selection, persisted on-device. Mirrors the web
/// client's `mui-mode` Light/System/Dark control (LEN-32).
class ThemeModeController extends ChangeNotifier {
  ThemeMode _mode = ThemeMode.system;

  ThemeMode get mode => _mode;

  static const prefKey = 'theme_mode';

  /// Reads the persisted choice. Awaited before runApp so the first
  /// frame already renders in the right scheme — no flash.
  Future<void> load() async {
    final prefs = await SharedPreferences.getInstance();
    _mode = _decode(prefs.getString(prefKey));
  }

  Future<void> setMode(ThemeMode mode) async {
    if (mode == _mode) return;
    _mode = mode;
    notifyListeners();
    final prefs = await SharedPreferences.getInstance();
    await prefs.setString(prefKey, mode.name);
  }

  static ThemeMode _decode(String? raw) => switch (raw) {
        'light' => ThemeMode.light,
        'dark' => ThemeMode.dark,
        _ => ThemeMode.system,
      };
}
