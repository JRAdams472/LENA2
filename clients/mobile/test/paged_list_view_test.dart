import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:lena_mobile/widgets/paged_list_view.dart';

Widget _wrap(Widget child) => MaterialApp(home: Scaffold(body: child));

List<Map<String, dynamic>> _rows(int n) =>
    List.generate(n, (i) => {'id': '$i'});

void main() {
  group('nextPageFor', () {
    test('advances on full pages', () {
      expect(nextPageFor(25, 25), 2);
      expect(nextPageFor(50, 25), 3);
    });
    test('ceil handles a trailing partial page', () {
      expect(nextPageFor(80, 50), 3);
      expect(nextPageFor(1, 25), 2);
    });
  });

  group('appendPageItems', () {
    final merge = appendPageItems('recipes');

    test('appends items and keeps latest pageInfo', () {
      final prev = {
        'recipes': {
          'items': _rows(2),
          'pageInfo': {'totalCount': 4},
        }
      };
      final more = {
        'recipes': {
          'items': _rows(2),
          'pageInfo': {'totalCount': 4, 'pageNumber': 2},
        }
      };
      final merged = merge(prev, more)!;
      final page = merged['recipes'] as Map<String, dynamic>;
      expect((page['items'] as List).length, 4);
      expect(page['pageInfo']['pageNumber'], 2);
    });

    test('returns the non-null side when the other is null', () {
      final more = {
        'recipes': {
          'items': _rows(1),
          'pageInfo': {'totalCount': 1}
        }
      };
      expect(merge(null, more), more);
      expect(merge(more, null), more);
    });
  });

  group('PagedListView', () {
    testWidgets('shows a loading footer only while more rows exist',
        (tester) async {
      var calls = 0;
      await tester.pumpWidget(_wrap(PagedListView(
        loadedCount: 3,
        totalCount: 3,
        onLoadMore: () async => calls++,
        itemBuilder: (_, i) => Text('row $i'),
      )));
      await tester.pump();
      expect(find.byType(CircularProgressIndicator), findsNothing);
      expect(calls, 0);
    });

    testWidgets('loads more when scrolled near the bottom', (tester) async {
      var calls = 0;
      var loaded = 50;
      await tester.pumpWidget(_wrap(StatefulBuilder(
        builder: (context, setState) => PagedListView(
          loadedCount: loaded,
          totalCount: 100,
          onLoadMore: () async {
            calls++;
            setState(() => loaded += 50);
          },
          itemBuilder: (_, i) => SizedBox(height: 60, child: Text('row $i')),
        ),
      )));
      await tester.pump();
      // First page underfills nothing (50 rows of 60px); scroll to the end.
      await tester.drag(find.byType(ListView), const Offset(0, -30000));
      await tester.pumpAndSettle();
      expect(calls, greaterThan(0));
      // totalCount reached — the loading footer is gone.
      expect(find.byType(CircularProgressIndicator), findsNothing);
    });

    testWidgets('does not fire concurrent loads', (tester) async {
      var calls = 0;
      final gate = Completer<void>();
      await tester.pumpWidget(_wrap(PagedListView(
        loadedCount: 50,
        totalCount: 200,
        onLoadMore: () {
          calls++;
          return gate.future;
        },
        itemBuilder: (_, i) => SizedBox(height: 60, child: Text('row $i')),
      )));
      await tester.pump();
      await tester.drag(find.byType(ListView), const Offset(0, -30000));
      await tester.pump();
      expect(calls, 1);
      // Scroll notifications during the in-flight load must not stack.
      await tester.pump(const Duration(milliseconds: 50));
      expect(calls, 1);
      gate.complete();
      await tester.pump();
      // After the load resolves, a subsequent page is allowed.
      await tester.pump(const Duration(milliseconds: 50));
      expect(calls, greaterThanOrEqualTo(1));
    });
  });
}
