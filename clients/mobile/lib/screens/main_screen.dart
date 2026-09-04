import 'package:flutter/material.dart';
import 'home_screen.dart';
import 'recipes_screen.dart';
import 'meal_plans_screen.dart';
import 'wine_screen.dart';
import 'grocery_screen.dart';

class MainScreen extends StatefulWidget {
  const MainScreen({super.key});

  @override
  State<MainScreen> createState() => _MainScreenState();
}

class _MainScreenState extends State<MainScreen> {
  int _index = 0;

  final _screens = const [
    HomeScreen(),
    RecipesScreen(),
    MealPlansScreen(),
    WineScreen(),
    GroceryScreen(),
  ];

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      body: IndexedStack(
        index: _index,
        children: _screens,
      ),
      bottomNavigationBar: BottomNavigationBar(
        currentIndex: _index,
        onTap: (i) => setState(() => _index = i),
        type: BottomNavigationBarType.fixed,
        items: const [
          BottomNavigationBarItem(icon: Icon(Icons.home), label: 'Home'),
          BottomNavigationBarItem(icon: Icon(Icons.menu_book), label: 'Recipes'),
          BottomNavigationBarItem(icon: Icon(Icons.calendar_today), label: 'Meal Plans'),
          BottomNavigationBarItem(icon: Icon(Icons.wine_bar), label: 'Wine'),
          BottomNavigationBarItem(icon: Icon(Icons.shopping_cart), label: 'Grocery'),
        ],
      ),
    );
  }
}
