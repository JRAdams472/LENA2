import 'package:flutter/material.dart';
import 'package:graphql_flutter/graphql_flutter.dart';
import 'grocery_screen.dart';

const String dashboardQuery = r'''
  query Dashboard {
    me {
      id
      email
      displayName
    }
    userItems(page: 1, pageSize: 10) {
      items {
        id
        item {
          name
        }
        currentQty
      }
      pageInfo {
        totalCount
      }
    }
  }
''';

class HomeScreen extends StatelessWidget {
  const HomeScreen({super.key});

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('LENA')),
      body: Query(
        options: QueryOptions(document: gql(dashboardQuery)),
        builder: (QueryResult result, {VoidCallback? refetch, FetchMore? fetchMore}) {
          if (result.isLoading) {
            return const Center(child: CircularProgressIndicator());
          }
          if (result.hasException) {
            return Center(child: Text('Error: ${result.exception.toString()}'));
          }

          final me = result.data?['me'];
          final userItems = result.data?['userItems']?['items'] as List? ?? [];

          return Padding(
            padding: const EdgeInsets.all(16.0),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text('Hello, ${me?['displayName'] ?? me?['email'] ?? 'guest'}'),
                const SizedBox(height: 16),
                const Text('Pantry', style: TextStyle(fontSize: 18, fontWeight: FontWeight.bold)),
                Expanded(
                  child: ListView.builder(
                    itemCount: userItems.length,
                    itemBuilder: (context, index) {
                      final item = userItems[index];
                      return ListTile(
                        title: Text(item['item']['name'] as String),
                        subtitle: Text('Qty: ${item['currentQty']}'),
                      );
                    },
                  ),
                ),
                const SizedBox(height: 16),
                ElevatedButton(
                  onPressed: () => Navigator.push(
                    context,
                    MaterialPageRoute(builder: (_) => const GroceryScreen()),
                  ),
                  child: const Text('Grocery List'),
                ),
              ],
            ),
          );
        },
      ),
    );
  }
}
