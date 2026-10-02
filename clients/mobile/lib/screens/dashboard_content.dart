import 'package:flutter/material.dart';

import '../dashboard_helpers.dart';
import '../theme.dart';
import 'edit_recipe_screen.dart';
import 'meal_plans_screen.dart';

const _weekdays = [
  'Monday',
  'Tuesday',
  'Wednesday',
  'Thursday',
  'Friday',
  'Saturday',
  'Sunday',
];
const _months = [
  'Jan',
  'Feb',
  'Mar',
  'Apr',
  'May',
  'Jun',
  'Jul',
  'Aug',
  'Sep',
  'Oct',
  'Nov',
  'Dec',
];

String dashboardHeaderDate(DateTime d) =>
    '${_weekdays[d.weekday - 1]}, ${_months[d.month - 1]} ${d.day}'
        .toUpperCase();

/// Loaded-state dashboard body — separated from the Query wrapper so it
/// can be rendered directly in widget/golden tests with fixture data.
class DashboardContent extends StatelessWidget {
  const DashboardContent({super.key, required this.data, this.onRefresh});

  final Map<String, dynamic> data;
  final VoidCallback? onRefresh;

  @override
  Widget build(BuildContext context) {
    final me = data['me'] as Map<String, dynamic>?;
    final name =
        (me?['displayName'] as String?) ?? (me?['email'] as String?) ?? 'there';
    final plans = data['mealPlans']?['items'] as List? ?? [];
    final recommendations = data['recommendedRecipes'] as List? ?? [];
    final myId = me?['id'] as String?;
    final incomingInvites = (data['householdInvites'] as List? ?? [])
        .cast<Map<String, dynamic>>()
        .where(
          (i) =>
              (i['toUser'] as Map?)?['id'] == myId && i['status'] == 'PENDING',
        )
        .toList();
    final unread = data['unreadNotificationCount'] as int? ?? 0;

    final slots = todaysSlots(plans, jsWeekday(DateTime.now()));

    return RefreshIndicator(
      onRefresh: () async => onRefresh?.call(),
      child: ListView(
        padding: const EdgeInsets.fromLTRB(16, 16, 16, 24),
        children: [
          _greetingHeader(context, name),
          if (incomingInvites.isNotEmpty || unread > 0) ...[
            const SizedBox(height: 12),
            for (final inv in incomingInvites)
              Card(
                color: Theme.of(context).colorScheme.secondaryContainer,
                child: ListTile(
                  leading: const Icon(Icons.mail_outline),
                  title: Text(
                    '${_inviteSender(inv['fromUser'] as Map<String, dynamic>?)} invited you to their household',
                  ),
                  subtitle: const Text('Open the Household tab to respond'),
                ),
              ),
            if (unread > 0)
              Card(
                child: ListTile(
                  leading: Badge(
                    label: Text('$unread'),
                    child: const Icon(Icons.notifications_outlined),
                  ),
                  title: Text(
                    unread == 1
                        ? '1 unread household notification'
                        : '$unread unread household notifications',
                  ),
                ),
              ),
          ],
          const SizedBox(height: 20),
          _todaysMeals(context, slots),
          const SizedBox(height: 20),
          _suggestions(context, recommendations),
        ],
      ),
    );
  }

  Widget _greetingHeader(BuildContext context, String name) {
    final initial = name.trim().isEmpty ? '?' : name.trim()[0].toUpperCase();
    return Row(
      crossAxisAlignment: CrossAxisAlignment.center,
      children: [
        Expanded(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                dashboardHeaderDate(DateTime.now()),
                style: Theme.of(context).textTheme.labelSmall,
              ),
              const SizedBox(height: 2),
              Text(
                'Hello, $name',
                style: Theme.of(context).textTheme.headlineSmall,
              ),
            ],
          ),
        ),
        CircleAvatar(
          radius: 20,
          backgroundColor: lenaSage.withValues(alpha: 0.25),
          child: Text(
            initial,
            style: const TextStyle(
              color: lenaSageDark,
              fontWeight: FontWeight.w700,
            ),
          ),
        ),
      ],
    );
  }

  Widget _todaysMeals(BuildContext context, List<Map<String, dynamic>> slots) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text('Today\'s meals', style: Theme.of(context).textTheme.titleLarge),
        const SizedBox(height: 8),
        if (slots.isEmpty)
          Card(
            child: Padding(
              padding: const EdgeInsets.all(16),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    'Nothing planned for today yet',
                    style: Theme.of(context).textTheme.titleSmall,
                  ),
                  const SizedBox(height: 4),
                  TextButton(
                    style: TextButton.styleFrom(
                      padding: EdgeInsets.zero,
                      minimumSize: const Size(0, 36),
                      tapTargetSize: MaterialTapTargetSize.shrinkWrap,
                    ),
                    onPressed: () => Navigator.push(
                      context,
                      MaterialPageRoute(
                        builder: (_) => const MealPlansScreen(),
                      ),
                    ),
                    child: const Text('Plan it →'),
                  ),
                ],
              ),
            ),
          )
        else
          Card(
            child: Column(
              children: [
                for (final (i, slot) in slots.indexed) ...[
                  if (i > 0) const Divider(height: 1, indent: 72),
                  _mealRow(context, slot),
                ],
              ],
            ),
          ),
      ],
    );
  }

  Widget _mealRow(BuildContext context, Map<String, dynamic> slot) {
    final recipe = slot['recipe'] as Map<String, dynamic>?;
    final recipeId = recipe?['id'] as String?;
    final meta = suggestionMeta(recipe);
    final servings = slot['servings'];
    final detail = [
      if (servings != null) '$servings servings',
      if (meta.isNotEmpty) meta,
    ].join(' · ');

    return InkWell(
      onTap: recipeId == null
          ? null
          : () => Navigator.push(
                context,
                MaterialPageRoute(
                  builder: (_) => EditRecipeScreen(recipeId: recipeId),
                ),
              ),
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
        child: Row(
          children: [
            _iconBadge(
              mealIcon(slot['mealType'] as String?),
              lenaSage,
              size: 40,
            ),
            const SizedBox(width: 12),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    mealLabel(slot['mealType'] as String?),
                    style: Theme.of(context).textTheme.labelSmall,
                  ),
                  Text(
                    recipe?['name'] as String? ?? 'Unknown recipe',
                    style: Theme.of(context).textTheme.titleSmall,
                  ),
                  if (detail.isNotEmpty)
                    Text(detail, style: Theme.of(context).textTheme.bodySmall),
                ],
              ),
            ),
            if (recipeId != null)
              const Icon(Icons.chevron_right, color: lenaInkMuted),
          ],
        ),
      ),
    );
  }

  Widget _suggestions(BuildContext context, List<dynamic> recommendations) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          'Delicious ideas for tonight',
          style: Theme.of(context).textTheme.titleLarge,
        ),
        const SizedBox(height: 8),
        if (recommendations.isEmpty)
          Card(
            child: Padding(
              padding: const EdgeInsets.all(16),
              child: Text(
                'Nothing to suggest yet — rate a few recipes and we\'ll get '
                'ideas flowing.',
                style: Theme.of(context).textTheme.bodyMedium,
              ),
            ),
          )
        else ...[
          _featuredCard(context, recommendations.first as Map<String, dynamic>),
          const SizedBox(height: 8),
          for (final rec in recommendations.skip(1).take(4))
            _suggestionRow(context, rec as Map<String, dynamic>),
        ],
      ],
    );
  }

  Widget _featuredCard(BuildContext context, Map<String, dynamic> rec) {
    final recipe = rec['recipe'] as Map<String, dynamic>?;
    final recipeId = recipe?['id'] as String?;
    final meta = suggestionMeta(recipe);
    final accent = accentFor(_recipeNum(recipeId));

    return Card(
      clipBehavior: Clip.antiAlias,
      child: InkWell(
        onTap: recipeId == null ? null : () => _openRecipe(context, recipeId),
        child: Padding(
          padding: const EdgeInsets.all(16),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Row(
                children: [
                  Flexible(
                    child: _reasonChip(context, rec['reason'] as String? ?? ''),
                  ),
                  const SizedBox(width: 8),
                  _iconBadge(suggestionIcon(_categories(recipe)), accent),
                ],
              ),
              const SizedBox(height: 10),
              Text(
                recipe?['name'] as String? ?? 'Unknown',
                style: Theme.of(context).textTheme.titleLarge,
              ),
              if (meta.isNotEmpty) ...[
                const SizedBox(height: 4),
                Text(meta, style: Theme.of(context).textTheme.bodySmall),
              ],
              const SizedBox(height: 12),
              Row(
                children: [
                  Text(
                    'View recipe',
                    style: Theme.of(context).textTheme.bodyMedium?.copyWith(
                          color: lenaSageDark,
                          fontWeight: FontWeight.w600,
                        ),
                  ),
                  const SizedBox(width: 4),
                  const Icon(
                    Icons.arrow_forward,
                    size: 16,
                    color: lenaSageDark,
                  ),
                ],
              ),
            ],
          ),
        ),
      ),
    );
  }

  Widget _suggestionRow(BuildContext context, Map<String, dynamic> rec) {
    final recipe = rec['recipe'] as Map<String, dynamic>?;
    final recipeId = recipe?['id'] as String?;
    final meta = suggestionMeta(recipe);
    final accent = accentFor(_recipeNum(recipeId));

    return Card(
      clipBehavior: Clip.antiAlias,
      margin: const EdgeInsets.only(bottom: 8),
      child: InkWell(
        onTap: recipeId == null ? null : () => _openRecipe(context, recipeId),
        child: Padding(
          padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
          child: Row(
            children: [
              _iconBadge(suggestionIcon(_categories(recipe)), accent, size: 40),
              const SizedBox(width: 12),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      recipe?['name'] as String? ?? 'Unknown',
                      style: Theme.of(context).textTheme.titleSmall,
                    ),
                    if (meta.isNotEmpty)
                      Text(
                        meta,
                        style: Theme.of(context).textTheme.bodySmall,
                      ),
                    Text(
                      reasonLabel(rec['reason'] as String? ?? ''),
                      style: Theme.of(
                        context,
                      ).textTheme.bodySmall?.copyWith(color: lenaOliveDark),
                    ),
                  ],
                ),
              ),
              const Icon(Icons.chevron_right, color: lenaInkMuted),
            ],
          ),
        ),
      ),
    );
  }

  Widget _reasonChip(BuildContext context, String reason) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
      decoration: BoxDecoration(
        color: lenaOlive.withValues(alpha: 0.14),
        borderRadius: BorderRadius.circular(999),
      ),
      child: Text(
        reasonLabel(reason),
        overflow: TextOverflow.ellipsis,
        style: Theme.of(context).textTheme.labelSmall?.copyWith(
              color: lenaOliveDark,
              fontWeight: FontWeight.w600,
            ),
      ),
    );
  }

  Widget _iconBadge(IconData icon, Color accent, {double size = 48}) {
    return Container(
      width: size,
      height: size,
      decoration: BoxDecoration(
        color: accent,
        borderRadius: BorderRadius.circular(10),
      ),
      child: Icon(icon, color: lenaPaper, size: size * 0.55),
    );
  }

  void _openRecipe(BuildContext context, String recipeId) {
    Navigator.push(
      context,
      MaterialPageRoute(
        builder: (_) => EditRecipeScreen(recipeId: recipeId),
      ),
    );
  }

  List<dynamic>? _categories(Map<String, dynamic>? recipe) =>
      recipe?['categories'] as List<dynamic>?;

  int _recipeNum(String? id) => int.tryParse(id ?? '') ?? 0;

  String _inviteSender(Map<String, dynamic>? u) {
    if (u == null) return 'Someone';
    final display = u['displayName'] as String?;
    if (display != null && display.isNotEmpty) return display;
    final combined = [
      u['firstName'],
      u['lastName'],
    ].whereType<String>().join(' ');
    return combined.isNotEmpty ? combined : 'Someone';
  }
}
