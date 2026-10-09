import 'package:flutter/material.dart';

import '../responsive.dart';
import 'empty_state.dart';

/// Master/detail two-pane layout for list→detail surfaces.
///
/// On expanded widths (≥840dp) [list] renders in a fixed-width left pane
/// beside [detail] — or an [EmptyState] placeholder when nothing is
/// selected. On compact/medium it renders [list] alone; callers select
/// via [openOrSelect], which falls back to `Navigator.push`.
class AdaptiveDetail extends StatelessWidget {
  const AdaptiveDetail({
    super.key,
    required this.list,
    required this.detail,
    this.placeholderIcon = Icons.touch_app_outlined,
    this.placeholderTitle = 'Select an item',
    this.listWidth = 360,
    this.onDetailClosed,
  });

  /// The list pane — always rendered.
  final Widget list;

  /// The selected item's detail screen (a plain screen widget — its
  /// Scaffold/AppBar render as the pane content). Null shows the
  /// placeholder instead. Set a `ValueKey` on it so switching selection
  /// rebuilds the pane.
  final Widget? detail;

  final IconData placeholderIcon;
  final String placeholderTitle;
  final double listWidth;

  /// Called when the detail pane's root page is popped (e.g. the detail
  /// screen calls `Navigator.pop` after a save). The parent should clear
  /// its selection so the placeholder returns.
  final VoidCallback? onDetailClosed;

  @override
  Widget build(BuildContext context) {
    if (!context.isExpanded) return list;
    final d = detail;
    // The detail lives in a nested Navigator so screens that push pickers
    // or pop-after-save stay scoped to the pane instead of popping the
    // enclosing route (a tab has nothing to pop).
    final rootPage = d == null ? null : MaterialPage(child: d);
    return Row(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        SizedBox(width: listWidth, child: list),
        const VerticalDivider(width: 1),
        Expanded(
          child: d == null
              ? EmptyState(
                  icon: placeholderIcon,
                  title: placeholderTitle,
                )
              : Navigator(
                  key: ValueKey(d.key),
                  pages: [rootPage!],
                  onDidRemovePage: (page) {
                    if (identical(page, rootPage)) onDetailClosed?.call();
                  },
                ),
        ),
      ],
    );
  }
}

/// Detail-open routing for list→detail screens. On expanded widths
/// [select] runs so the screen can render the detail inline in an
/// [AdaptiveDetail] pane; anywhere else [builder] is pushed as a full
/// route so phone behavior is unchanged.
///
/// Returns the push Future on push (so callers can refetch on pop) and
/// null when the detail was selected inline.
Future<void>? openOrSelect(
  BuildContext context, {
  required VoidCallback select,
  required WidgetBuilder builder,
}) {
  if (context.isExpanded) {
    select();
    return null;
  }
  return Navigator.push(context, MaterialPageRoute(builder: builder));
}
