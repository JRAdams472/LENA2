import 'package:flutter/material.dart';

/// Motion spec from docs/design.md §7 — decelerate curves, three
/// durations, content enters rather than snapping in.
const lenaMotionFast = Duration(milliseconds: 150);
const lenaMotionMedium = Duration(milliseconds: 250);
const lenaMotionSlow = Duration(milliseconds: 400);
const lenaDecelerate = Curves.easeOutCubic;

/// One-shot entrance: fade + 8px rise over [lenaMotionSlow]. Wrap
/// loaded content that replaces a skeleton so the swap reads as a
/// transition instead of a flash. Honors no accessibility flags — the
/// animation is short, decelerating, and non-looping.
class LenaFadeIn extends StatelessWidget {
  const LenaFadeIn({super.key, required this.child});

  final Widget child;

  @override
  Widget build(BuildContext context) {
    return TweenAnimationBuilder<double>(
      tween: Tween(begin: 0, end: 1),
      duration: lenaMotionSlow,
      curve: lenaDecelerate,
      builder: (context, t, child) => Opacity(
        opacity: t,
        child: Transform.translate(
          offset: Offset(0, 8 * (1 - t)),
          child: child,
        ),
      ),
      child: child,
    );
  }
}
