import 'package:flutter/material.dart';

/// Shared skeleton placeholders for list/form loading states.
///
/// LEN-50: the audit counted 35 bare `CircularProgressIndicator`s across
/// the app. Skeletons preview the incoming layout instead of a spinner
/// on a blank surface.
class SkeletonBox extends StatelessWidget {
  const SkeletonBox({
    super.key,
    this.width,
    this.height = 14,
    this.radius = 6,
  });

  final double? width;
  final double height;
  final double radius;

  @override
  Widget build(BuildContext context) {
    return Container(
      width: width,
      height: height,
      decoration: BoxDecoration(
        color: Theme.of(context).colorScheme.outlineVariant,
        borderRadius: BorderRadius.circular(radius),
      ),
    );
  }
}

/// A card-shaped row placeholder: leading block + two text lines.
class SkeletonCard extends StatelessWidget {
  const SkeletonCard({super.key});

  @override
  Widget build(BuildContext context) {
    return const Card(
      child: Padding(
        padding: EdgeInsets.symmetric(horizontal: 16, vertical: 14),
        child: Row(
          children: [
            SkeletonBox(width: 36, height: 36, radius: 18),
            SizedBox(width: 12),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  SkeletonBox(width: 180, height: 14),
                  SizedBox(height: 8),
                  SkeletonBox(width: 120, height: 10),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }
}

/// A list of skeleton cards sized to fill the scroll area.
class SkeletonList extends StatelessWidget {
  const SkeletonList({super.key, this.count = 6, this.padding});

  final int count;
  final EdgeInsets? padding;

  @override
  Widget build(BuildContext context) {
    return ListView.builder(
      padding: padding ?? const EdgeInsets.all(16),
      physics: const NeverScrollableScrollPhysics(),
      itemCount: count,
      itemBuilder: (_, __) => const SkeletonCard(),
    );
  }
}

/// Form-shaped placeholder for create/edit screens — a few label +
/// field bar pairs.
class SkeletonForm extends StatelessWidget {
  const SkeletonForm({super.key, this.fields = 6});

  final int fields;

  @override
  Widget build(BuildContext context) {
    return ListView(
      padding: const EdgeInsets.all(16),
      physics: const NeverScrollableScrollPhysics(),
      children: [
        for (var i = 0; i < fields; i++) ...[
          const SkeletonBox(width: 90, height: 10),
          const SizedBox(height: 6),
          const SkeletonBox(height: 44, radius: 10),
          const SizedBox(height: 14),
        ],
      ],
    );
  }
}

/// Branded boot splash — replaces the blank cream + lone spinner the
/// app showed while auth/initial queries warm up.
class LenaSplash extends StatelessWidget {
  const LenaSplash({super.key});

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      body: Center(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Container(
              width: 72,
              height: 72,
              decoration: BoxDecoration(
                color: Theme.of(context).colorScheme.primary,
                borderRadius: BorderRadius.circular(20),
              ),
              child: Center(
                child: Text(
                  'L',
                  style: TextStyle(
                    fontFamily: 'Nunito',
                    fontSize: 40,
                    fontWeight: FontWeight.w800,
                    color: Theme.of(context).colorScheme.onPrimary,
                  ),
                ),
              ),
            ),
            const SizedBox(height: 16),
            Text(
              'Lena',
              style: Theme.of(context)
                  .textTheme
                  .headlineMedium
                  ?.copyWith(
                      color: Theme.of(context).colorScheme.onPrimaryContainer),
            ),
          ],
        ),
      ),
    );
  }
}
