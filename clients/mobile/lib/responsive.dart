import 'package:flutter/widgets.dart';

/// Material 3 window size classes, keyed off layout width in logical
/// pixels. `shortestSide` is deliberately not used — a rotated phone is
/// wide, and width is what the adaptive layouts respond to.
enum WindowSizeClass { compact, medium, expanded }

WindowSizeClass windowSizeClassOf(BuildContext context) {
  final w = MediaQuery.sizeOf(context).width;
  if (w >= 840) return WindowSizeClass.expanded;
  if (w >= 600) return WindowSizeClass.medium;
  return WindowSizeClass.compact;
}

extension ResponsiveContext on BuildContext {
  WindowSizeClass get windowSize => windowSizeClassOf(this);

  /// medium or expanded — enough width for a nav rail / roomier layouts.
  bool get isWide => windowSize != WindowSizeClass.compact;

  /// expanded — enough width for true two-pane master/detail.
  bool get isExpanded => windowSize == WindowSizeClass.expanded;
}

/// Centers content in a max-width column on wide screens; a no-op on
/// compact. Wraps scrollable bodies so tablet/landscape layouts don't
/// stretch rows and cards to full width.
class ConstrainedContent extends StatelessWidget {
  const ConstrainedContent(
      {super.key, required this.child, this.maxWidth = 840});

  final Widget child;
  final double maxWidth;

  @override
  Widget build(BuildContext context) {
    if (context.isWide) {
      return Align(
        alignment: Alignment.topCenter,
        child: ConstrainedBox(
          constraints: BoxConstraints(maxWidth: maxWidth),
          child: child,
        ),
      );
    }
    return child;
  }
}
