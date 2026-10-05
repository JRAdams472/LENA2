import 'package:flutter/material.dart';
import 'items_screen.dart';
import 'meal_plans_screen.dart';
import 'recipes_screen.dart';
import 'wine_screen.dart';

/// Secondary destinations that don't fit in the bottom nav — the
/// planning-and-browsing surfaces rather than the in-the-moment tabs.
class MoreScreen extends StatelessWidget {
  const MoreScreen({super.key});

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('More')),
      body: ListView(
        padding: const EdgeInsets.all(16),
        children: [
          _MoreTile(
            icon: Icons.restaurant_menu,
            title: 'Recipes',
            subtitle: 'Search, filter, and edit your recipe book',
            builder: (_) => const RecipesScreen(),
          ),
          _MoreTile(
            icon: Icons.calendar_month,
            title: 'Meal plans',
            subtitle: 'Plan the week and adjust meal slots',
            builder: (_) => const MealPlansScreen(),
          ),
          _MoreTile(
            icon: Icons.wine_bar,
            title: 'Wine cellar',
            subtitle: 'Your bottles and counts',
            builder: (_) => const WineScreen(),
          ),
          _MoreTile(
            icon: Icons.inventory_2,
            title: 'Items',
            subtitle: 'The household item catalog',
            builder: (_) => const ItemsScreen(),
          ),
        ],
      ),
    );
  }
}

class _MoreTile extends StatelessWidget {
  const _MoreTile({
    required this.icon,
    required this.title,
    required this.subtitle,
    required this.builder,
  });

  final IconData icon;
  final String title;
  final String subtitle;
  final WidgetBuilder builder;

  @override
  Widget build(BuildContext context) {
    return Card(
      child: ListTile(
        leading: CircleAvatar(
          backgroundColor: Theme.of(context).colorScheme.primaryContainer,
          child: Icon(icon,
              color: Theme.of(context).colorScheme.onPrimaryContainer),
        ),
        title: Text(title, style: const TextStyle(fontWeight: FontWeight.w600)),
        subtitle: Text(subtitle),
        trailing: const Icon(Icons.chevron_right),
        onTap: () =>
            Navigator.push(context, MaterialPageRoute(builder: builder)),
      ),
    );
  }
}
