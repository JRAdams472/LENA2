import 'package:flutter/material.dart';

/// Semantic tint for [StatusChip].
enum StatusTone { primary, secondary, neutral, error }

/// Soft-tinted status label — container fill at 12% tone, label in the
/// matching readable-on-surface tone. Used for active/role markers,
/// invitation states, and similar inline status (design.md v2 §6).
class StatusChip extends StatelessWidget {
  const StatusChip({super.key, required this.label, this.tone = StatusTone.neutral});

  final String label;
  final StatusTone tone;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final (fill, text) = switch (tone) {
      StatusTone.primary => (scheme.primary, scheme.onPrimaryContainer),
      StatusTone.secondary => (scheme.secondary, scheme.onSecondaryContainer),
      StatusTone.error => (scheme.error, scheme.error),
      StatusTone.neutral => (
          scheme.onSurfaceVariant,
          scheme.onSurfaceVariant,
        ),
    };
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 2),
      decoration: BoxDecoration(
        color: fill.withValues(alpha: 0.14),
        borderRadius: BorderRadius.circular(999),
      ),
      child: Text(
        label,
        style: Theme.of(context).textTheme.labelSmall?.copyWith(
              color: text,
              fontWeight: FontWeight.w600,
              letterSpacing: 0,
            ),
      ),
    );
  }
}
