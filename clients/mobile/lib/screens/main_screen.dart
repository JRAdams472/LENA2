import 'dart:async';

import 'package:flutter/material.dart';
import 'package:graphql_flutter/graphql_flutter.dart';
import '../push/push_service.dart';
import 'assistant_screen.dart';
import 'dashboard_screen.dart';
import 'events_screen.dart';
import 'grocery_lists_screen.dart';
import 'household_screen.dart';
import 'more_screen.dart';
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

  // Tabs build lazily on first visit — an eager IndexedStack mounts
  // ScanScreen at launch, which requests CAMERA permission before the
  // user ever touches Scan. Placeholders keep unvisited tabs unbuilt.
  static final _builders = <Widget Function()>[
    () => const DashboardScreen(),
    () => const GroceryListsScreen(),
    () => const EventsScreen(),
    () => const ScanScreen(),
    () => const PantryScreen(),
    () => const HouseholdScreen(),
    () => const AssistantScreen(),
    () => const MoreScreen(),
  ];
  final List<Widget?> _screens = List.filled(8, null);
  final Set<int> _visited = {0};

  Widget _tab(int i) {
    if (!_visited.contains(i)) return const SizedBox.shrink();
    return _screens[i] ??= _builders[i]();
  }

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    if (_poll != null) return;
    _client = GraphQLProvider.of(context).value;
    _loadUnread();
    // Timer.periodic (not QueryOptions.pollInterval) so the timer cancels
    // on dispose and widget tests stay clean.
    _poll = Timer.periodic(const Duration(seconds: 30), (_) => _loadUnread());
    // Push arrives only after sign-in — permission + token registration
    // happen here, and a foreground push bumps the badge immediately.
    pushService.start();
    pushService.onPushReceived = _loadUnread;
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
    if (pushService.onPushReceived == _loadUnread) {
      pushService.onPushReceived = null;
    }
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      body: IndexedStack(
        index: _index,
        children: [for (var i = 0; i < _builders.length; i++) _tab(i)],
      ),
      bottomNavigationBar: BottomNavigationBar(
        currentIndex: _index,
        onTap: (i) {
          setState(() {
            _index = i;
            _visited.add(i);
          });
          // Opening the Household tab auto-marks notifications read;
          // re-poll so the badge clears without waiting for the interval.
          _loadUnread();
        },
        type: BottomNavigationBarType.fixed,
        // Eight fixed destinations can't fit full labels — show the
        // selected label only and keep every label ≤7 chars.
        showUnselectedLabels: false,
        selectedFontSize: 12,
        items: [
          const BottomNavigationBarItem(
            icon: Icon(Icons.dashboard),
            label: 'Home',
          ),
          const BottomNavigationBarItem(
            icon: Icon(Icons.shopping_cart),
            label: 'Grocery',
          ),
          const BottomNavigationBarItem(
            icon: Icon(Icons.event),
            label: 'Events',
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
            label: 'People',
          ),
          const BottomNavigationBarItem(
            icon: Icon(Icons.auto_awesome),
            label: 'Ask Dot',
          ),
          const BottomNavigationBarItem(
            icon: Icon(Icons.more_horiz),
            label: 'More',
          ),
        ],
      ),
    );
  }
}
