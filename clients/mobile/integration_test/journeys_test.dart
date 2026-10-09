import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:integration_test/integration_test.dart';
import 'package:lena_mobile/main.dart' as app;

/// Journey assertions against a running e2e stack (docs/testing.md).
///
///   flutter drive \
///     --driver=test_driver/integration_test.dart \
///     --target=integration_test/journeys_test.dart \
///     -d emulator-5554 \
///     --dart-define=LENA_API_URL=http://10.0.2.2/graphql \
///     --dart-define=LENA_DEBUG_ID_TOKEN=<test-issuer token for the app user> \
///     --dart-define=LENA_E2E_INVITER_TOKEN=<test-issuer token for a 2nd user>
///
/// Data is seeded over GraphQL with the same tokens the app uses. The Scan
/// tab is never opened, so the run needs no camera.
const _apiUrl = String.fromEnvironment('LENA_API_URL');
const _appToken = String.fromEnvironment('LENA_DEBUG_ID_TOKEN');
const _inviterToken = String.fromEnvironment('LENA_E2E_INVITER_TOKEN');

const _navIcons = <String, IconData>{
  'Home': Icons.dashboard,
  'Grocery': Icons.shopping_cart,
  'People': Icons.group,
};

Future<Map<String, dynamic>> _gql(
  String token,
  String query, [
  Map<String, dynamic> variables = const {},
]) async {
  final res = await http.post(
    Uri.parse(_apiUrl),
    headers: {
      'Authorization': 'Bearer $token',
      'Content-Type': 'application/json',
    },
    body: jsonEncode({'query': query, 'variables': variables}),
  );
  final body = jsonDecode(res.body) as Map<String, dynamic>;
  final errors = body['errors'] as List?;
  if (errors != null && errors.isNotEmpty) {
    throw StateError('GraphQL error: ${errors.map((e) => e['message'])}');
  }
  return body['data'] as Map<String, dynamic>;
}

String _unique(String prefix) =>
    '$prefix ${DateTime.now().microsecondsSinceEpoch.toRadixString(36)}';

String _thisMonday() {
  final now = DateTime.now();
  final monday = DateTime(now.year, now.month, now.day)
      .subtract(Duration(days: now.weekday - DateTime.monday));
  return monday.toIso8601String().substring(0, 10);
}

/// Pumps frames for [wait] — pumpAndSettle hangs on loading spinners.
Future<void> settle(WidgetTester tester,
    [Duration wait = const Duration(seconds: 3)]) async {
  final end = DateTime.now().add(wait);
  while (DateTime.now().isBefore(end)) {
    await tester.pump(const Duration(milliseconds: 100));
  }
}

/// Pumps until [finder] matches or [timeout] passes.
Future<void> waitFor(WidgetTester tester, Finder finder,
    {Duration timeout = const Duration(seconds: 20)}) async {
  final end = DateTime.now().add(timeout);
  while (finder.evaluate().isEmpty && DateTime.now().isBefore(end)) {
    await tester.pump(const Duration(milliseconds: 200));
  }
  expect(finder, findsWidgets);
}

Future<void> nav(WidgetTester tester, String label) async {
  await tester.tap(find.descendant(
    of: find.byType(BottomNavigationBar),
    matching: find.byIcon(_navIcons[label]!),
  ));
  await settle(tester, const Duration(seconds: 2));
}

Future<void> back(WidgetTester tester) async {
  await tester.binding.handlePopRoute();
  await settle(tester, const Duration(seconds: 1));
}

Future<List<Map<String, dynamic>>> _households(String token) async {
  final data = await _gql(token, '{ myHouseholds { id name isActive } }');
  return (data['myHouseholds'] as List).cast<Map<String, dynamic>>();
}

void main() {
  IntegrationTestWidgetsFlutterBinding.ensureInitialized();

  setUpAll(() {
    if (_apiUrl.isEmpty || _appToken.isEmpty || _inviterToken.isEmpty) {
      fail('LENA_API_URL, LENA_DEBUG_ID_TOKEN and LENA_E2E_INVITER_TOKEN '
          'must all be passed with --dart-define');
    }
  });

  testWidgets('dashboard, grocery check-off, household invite and switch',
      (tester) async {
    // ── Seed: today's dinner slot and a grocery list generated from it ──
    final recipeName = _unique('Journey Rice Bowl');
    final itemName = _unique('Journey Rice');
    final cat = await _gql(_appToken,
        r'mutation ($input: CreateCategoryInput!) { createCategory(input: $input) { id } }',
        {'input': {'name': _unique('Journey Cat'), 'description': null}});
    final catId = cat['createCategory']['id'] as String;
    final item = await _gql(_appToken,
        r'mutation ($input: CreateItemInput!) { createItem(input: $input) { id } }',
        {'input': {'name': itemName, 'categoryId': catId, 'unit': 'ea'}});
    final itemId = item['createItem']['id'] as String;
    await _gql(_appToken,
        r'mutation ($itemId: ID!) { adjustUserItem(itemId: $itemId, quantity: 0) { id } }',
        {'itemId': itemId});
    final recipe = await _gql(_appToken,
        r'mutation ($input: CreateRecipeInput!) { createRecipe(input: $input) { id } }',
        {
          'input': {
            'name': recipeName,
            'servings': 2,
            'items': [
              {'itemId': itemId, 'quantity': 2, 'unit': 'ea'}
            ],
            'steps': [
              {'stepNumber': 1, 'instruction': 'Cook the rice'}
            ],
          }
        });
    final recipeId = recipe['createRecipe']['id'] as String;
    final plan = await _gql(_appToken,
        r'mutation ($input: CreateMealPlanInput!) { createMealPlan(input: $input) { id } }',
        {'input': {'name': _unique('Journey Plan'), 'weekStartDate': _thisMonday()}});
    final planId = plan['createMealPlan']['id'] as String;
    await _gql(_appToken,
        r'mutation ($input: AddMealSlotInput!) { addMealSlot(input: $input) { id } }',
        {
          'input': {
            'mealPlanId': planId,
            'dayOfWeek': DateTime.now().weekday - DateTime.monday,
            'mealType': 'Dinner',
            'recipeId': recipeId,
            'servings': 2,
          }
        });
    final list = await _gql(_appToken,
        r'mutation ($id: ID!) { generateGroceryList(mealPlanId: $id) { id items { id name isChecked } } }',
        {'id': planId});
    final listId = list['generateGroceryList']['id'] as String;
    final line = (list['generateGroceryList']['items'] as List)
        .cast<Map<String, dynamic>>()
        .firstWhere((i) => (i['name'] as String?) == itemName);
    final lineId = line['id'] as String;
    expect(line['isChecked'], isFalse);

    Future<bool> lineChecked() async {
      final data = await _gql(_appToken,
          r'query ($id: ID!) { groceryList(id: $id) { items { id isChecked } } }',
          {'id': listId});
      return (data['groceryList']['items'] as List)
          .cast<Map<String, dynamic>>()
          .firstWhere((i) => i['id'] == lineId)['isChecked'] as bool;
    }

    final appHouseholds = await _households(_appToken);
    final home = appHouseholds.firstWhere((h) => h['isActive'] == true);
    String? inviterHouseholdId;

    try {
      // ── Sign-in lands on the dashboard with today's slot ──
      app.main();
      await waitFor(tester, find.text("Today's meals"));
      await waitFor(tester, find.text(recipeName));

      // ── Grocery tab lists the seeded list; check-off persists ──
      await nav(tester, 'Grocery');
      await waitFor(tester, find.text('Grocery Lists'));
      // Lists sort newest-first, so the seeded list is the top tile.
      await tester.tap(find.byType(ListTile).first);
      final row = find.byKey(ValueKey('grocery-item-$lineId'));
      await waitFor(tester, row);
      expect(tester.widget<CheckboxListTile>(row).value, isFalse);
      await tester.tap(row);
      await settle(tester);
      expect(await lineChecked(), isTrue);

      await back(tester);
      await tester.tap(find.byType(ListTile).first);
      await waitFor(tester, row);
      expect(tester.widget<CheckboxListTile>(row).value, isTrue,
          reason: 'check state survives reopening the list');
      await back(tester);

      // ── Household: accept an invite, then switch back ──
      final me = await _gql(_appToken, '{ me { id } }');
      final myId = me['me']['id'] as String;
      final inviterName = _unique('Journey Inviter Home');
      final inviter = await _households(_inviterToken);
      if (inviter.any((h) => h['isActive'] == true)) {
        await _gql(_inviterToken,
            r'mutation ($n: String!) { renameHousehold(name: $n) { id } }',
            {'n': inviterName});
      } else {
        await _gql(_inviterToken,
            r'mutation ($n: String) { createHousehold(name: $n) { id } }',
            {'n': inviterName});
      }
      inviterHouseholdId = (await _households(_inviterToken))
          .firstWhere((h) => h['isActive'] == true)['id'] as String;
      await _gql(_appToken,
          'mutation { updateMyProfile(input: { isSearchable: true }) { id } }');
      await _gql(_inviterToken,
          r'mutation ($id: ID!) { inviteHouseholdMember(userId: $id) { id } }',
          {'id': myId});

      await nav(tester, 'People');
      final invite = find.ancestor(
        of: find.textContaining('invited you'),
        matching: find.byType(ListTile),
      );
      await waitFor(tester, invite);
      await tester.tap(
          find.descendant(of: invite.first, matching: find.byIcon(Icons.check)));
      await settle(tester, const Duration(seconds: 2));
      // A sole-owned household triggers the merge prompt; join without
      // merging so the original household survives for the switch back.
      if (find.text('Join this household?').evaluate().isNotEmpty) {
        await tester.tap(find.text('Just join'));
        await settle(tester);
      }
      await waitFor(tester, find.text(inviterName));
      final active = (await _households(_appToken))
          .firstWhere((h) => h['isActive'] == true);
      expect(active['id'], inviterHouseholdId);

      await tester.tap(find.widgetWithText(TextButton, 'Switch').first);
      await settle(tester, const Duration(seconds: 1));
      final homeLabel = (home['name'] as String?) ?? 'Unnamed household';
      final homeTile = find.ancestor(
        of: find.text(homeLabel),
        matching: find.byType(ListTile),
      );
      await tester.tap(find
          .descendant(of: homeTile.last, matching: find.text('Switch'))
          .first);
      await settle(tester);
      final switched = (await _households(_appToken))
          .firstWhere((h) => h['isActive'] == true);
      expect(switched['id'], home['id']);
    } finally {
      if (inviterHouseholdId != null) {
        await _gql(_appToken,
                r'mutation ($id: ID) { leaveHousehold(householdId: $id) }',
                {'id': inviterHouseholdId})
            .catchError((_) => <String, dynamic>{});
      }
      await _gql(_appToken, r'mutation ($id: ID!) { setActiveHousehold(householdId: $id) { id } }',
              {'id': home['id']})
          .catchError((_) => <String, dynamic>{});
      await _gql(_appToken, r'mutation ($id: ID!) { deleteMealPlan(id: $id) }',
          {'id': planId}).catchError((_) => <String, dynamic>{});
      await _gql(_appToken, r'mutation ($id: ID!) { deleteRecipe(id: $id) }',
          {'id': recipeId}).catchError((_) => <String, dynamic>{});
      await _gql(_appToken, r'mutation ($id: ID!) { deleteItem(id: $id) }',
          {'id': itemId}).catchError((_) => <String, dynamic>{});
      await _gql(_appToken, r'mutation ($id: ID!) { deleteCategory(id: $id) }',
          {'id': catId}).catchError((_) => <String, dynamic>{});
    }
  });
}
