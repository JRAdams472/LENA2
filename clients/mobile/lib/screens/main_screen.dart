import 'dart:async';

import 'package:flutter/material.dart';
import 'package:graphql_flutter/graphql_flutter.dart';
import 'dashboard_screen.dart';
import 'grocery_lists_screen.dart';
import 'household_screen.dart';
import 'pantry_screen.dart';
import 'scan_screen.dart';

const String unreadCountQuery = r'''
  query UnreadCount {
    unreadNotificationCount
  }
''';

class MainScreen extends StatefulWidget {
  const MainScreen({super.key});

  @override
  State<MainScreen> createState() => _MainScreenState();
}

class _MainScreenState extends State<MainScreen> {
  int _index = 0;
  int _unreadNotifications = 0;
  Timer? _poll;
  GraphQLClient? _client;

  final _screens = const [
    DashboardScreen(),
    GroceryListsScreen(),
    ScanScreen(),
    PantryScreen(),
    HouseholdScreen(),
  ];

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    if (_poll != null) return;
    _client = GraphQLProvider.of(context).value;
    _loadUnread();
    // Timer.periodic (not QueryOptions.pollInterval) so the timer cancels
    // on dispose and widget tests stay clean.
    _poll = Timer.periodic(const Duration(seconds: 30), (_) => _loadUnread());
  }

  Future<void> _loadUnread() async {
    final client = _client;
    if (client == null) return;
    final result = await client.query(
      QueryOptions(document: gql(unreadCountQuery)),
    );
    final n = result.data?['unreadNotificationCount'] as int?;
    if (mounted && n != null) setState(() => _unreadNotifications = n);
  }

  @override
  void dispose() {
    _poll?.cancel();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      body: IndexedStack(
        index: _index,
        children: _screens,
      ),
      bottomNavigationBar: BottomNavigationBar(
        currentIndex: _index,
        onTap: (i) {
          setState(() => _index = i);
          // Opening the Household tab auto-marks notifications read;
          // re-poll so the badge clears without waiting for the interval.
          _loadUnread();
        },
        type: BottomNavigationBarType.fixed,
        items: [
          const BottomNavigationBarItem(
            icon: Icon(Icons.dashboard),
            label: 'Dashboard',
          ),
          const BottomNavigationBarItem(
            icon: Icon(Icons.shopping_cart),
            label: 'Grocery',
          ),
          const BottomNavigationBarItem(
            icon: Icon(Icons.qr_code_scanner),
            label: 'Scan',
          ),
          const BottomNavigationBarItem(
            icon: Icon(Icons.kitchen),
            label: 'Pantry',
          ),
          BottomNavigationBarItem(
            icon: Badge(
              isLabelVisible: _unreadNotifications > 0,
              label: Text('$_unreadNotifications'),
              child: const Icon(Icons.group),
            ),
            label: 'Household',
          ),
        ],
      ),
    );
  }
}
