import 'dart:async';

import 'package:flutter/material.dart';
import 'package:graphql_flutter/graphql_flutter.dart';
import '../widgets/skeleton.dart';
import '../widgets/status_chip.dart';

import '../allergy.dart';
import '../notification_links.dart';
import 'grocery_lists_screen.dart';
import 'notification_settings_screen.dart';

const int maxHouseholdMembers = 10;

const String householdQuery = r'''
  query HouseholdScreen($term: String!, $search: Boolean!) {
    me {
      id
      isSearchable
    }
    myHousehold {
      id
      name
      myRole
      createdAt
      members {
        user {
          id
          displayName
          firstName
          lastName
        }
        role
        isMe
      }
    }
    myHouseholds {
      id
      name
      myRole
      isActive
      members {
        role
        isMe
      }
    }
    householdInvites {
      id
      status
      fromUser {
        id
        displayName
        firstName
        lastName
      }
      toUser {
        id
        displayName
        firstName
        lastName
      }
    }
    myNotifications(limit: 10) {
      id
      kind
      foodEventId
      title
      body
      recipeId
      itemId
      createdAt
      actor {
        id
        displayName
        firstName
        lastName
      }
    }
    unreadNotificationCount
    searchHouseholdUsers(term: $term, limit: 10) @include(if: $search) {
      id
      displayName
      firstName
      lastName
    }
  }
''';

const String inviteMutation = r'''
  mutation InviteMember($userId: ID!) {
    inviteHouseholdMember(userId: $userId) { id }
  }
''';

const String acceptInviteMutation = r'''
  mutation AcceptInvite($inviteId: ID!, $mergeFromHouseholdId: ID) {
    acceptHouseholdInvite(inviteId: $inviteId, mergeFromHouseholdId: $mergeFromHouseholdId) { id }
  }
''';

const String setActiveHouseholdMutation = r'''
  mutation SetActiveHousehold($householdId: ID!) {
    setActiveHousehold(householdId: $householdId) { id }
  }
''';

const String createHouseholdMutation = r'''
  mutation CreateHousehold($name: String) {
    createHousehold(name: $name) { id name }
  }
''';

const String declineInviteMutation = r'''
  mutation DeclineInvite($inviteId: ID!) {
    declineHouseholdInvite(inviteId: $inviteId) { id }
  }
''';

const String cancelInviteMutation = r'''
  mutation CancelInvite($inviteId: ID!) {
    cancelHouseholdInvite(inviteId: $inviteId) { id }
  }
''';

const String leaveMutation = r'''
  mutation LeaveHousehold($householdId: ID) {
    leaveHousehold(householdId: $householdId)
  }
''';

const String renameMutation = r'''
  mutation RenameHousehold($name: String!) {
    renameHousehold(name: $name) { id name }
  }
''';

const String setRoleMutation = r'''
  mutation SetHouseholdRole($userId: ID!, $role: HouseholdRole!) {
    setHouseholdRole(userId: $userId, role: $role) { id }
  }
''';

const String removeMemberMutation = r'''
  mutation RemoveHouseholdMember($userId: ID!) {
    removeHouseholdMember(userId: $userId) { id }
  }
''';

const String transferMutation = r'''
  mutation TransferHouseholdOwnership($userId: ID!) {
    transferHouseholdOwnership(userId: $userId) { id }
  }
''';

const String updateProfileMutation = r'''
  mutation UpdateProfile($input: UpdateProfileInput!) {
    updateMyProfile(input: $input) { id isSearchable }
  }
''';

const String markAllReadMutation = r'''
  mutation MarkAllRead {
    markAllNotificationsRead
  }
''';

const String addToGroceryMutation = r'''
  mutation AddToGrocery($itemId: ID!) {
    addItemToCurrentGroceryList(itemId: $itemId) { id }
  }
''';

String _notificationText(Map<String, dynamic> n) {
  final actor = n['actor'] as Map<String, dynamic>?;
  final who = actor == null ? 'Someone' : _userName(actor);
  switch (n['kind'] as String?) {
    case 'INVITE_RECEIVED':
      return '$who invited you to their household';
    case 'INVITE_ACCEPTED':
      return '$who accepted your household invite';
    case 'INVITE_DECLINED':
      return '$who declined your household invite';
    case 'INVITE_CANCELLED':
      return '$who cancelled a household invite';
    case 'MEMBER_JOINED':
      return '$who joined your household';
    case 'MEMBER_LEFT':
      return '$who left your household';
    case 'MEMBER_REMOVED':
      return 'You were removed from a household';
    case 'ROLE_CHANGED':
      return '$who changed a household role';
    case 'HOUSEHOLD_RENAMED':
      return '$who renamed the household';
    case 'EVENT_CREATED':
      return '$who created an event';
    case 'EVENT_UPDATED':
      return '$who updated an event';
    case 'EVENT_DELETED':
      return '$who deleted an event';
    case 'PROTEIN_DEFROST':
      return 'Protein defrost reminder';
    case 'MEAL_PREP_ADVANCE':
      return 'Meal prep reminder';
    case 'ITEM_EXPIRING':
      return 'Pantry item expiring soon';
    default:
      return 'Household update';
  }
}

String _timeAgo(String? iso) {
  if (iso == null) return '';
  final then = DateTime.tryParse(iso);
  if (then == null) return '';
  final mins = DateTime.now().difference(then).inMinutes;
  if (mins < 1) return 'just now';
  if (mins < 60) return '${mins}m ago';
  final hours = mins ~/ 60;
  if (hours < 24) return '${hours}h ago';
  return '${hours ~/ 24}d ago';
}

String _userName(Map<String, dynamic> u) {
  final display = u['displayName'] as String?;
  if (display != null && display.isNotEmpty) return display;
  final first = u['firstName'] as String?;
  final last = u['lastName'] as String?;
  final combined = [first, last].whereType<String>().join(' ');
  if (combined.isNotEmpty) return combined;
  return 'User ${u['id']}';
}

class HouseholdScreen extends StatefulWidget {
  const HouseholdScreen({super.key});

  @override
  State<HouseholdScreen> createState() => _HouseholdScreenState();
}

class _HouseholdScreenState extends State<HouseholdScreen> {
  final TextEditingController _searchCtrl = TextEditingController();
  final TextEditingController _nameCtrl = TextEditingController();
  String _searchTerm = '';
  String? _error;
  VoidCallback? _refetch;
  Timer? _poll;
  bool _markedReadOnOpen = false;

  @override
  void initState() {
    super.initState();
    // The notification feed lives on this screen, so while it's open we
    // poll at the 5s feed cadence (web uses the same split: 30s idle /
    // 5s feed-open).
    _poll = Timer.periodic(
      const Duration(seconds: 5),
      (_) => _refetch?.call(),
    );
  }

  @override
  void dispose() {
    _poll?.cancel();
    _searchCtrl.dispose();
    _nameCtrl.dispose();
    super.dispose();
  }

  Future<void> _mutate(
    String doc,
    Map<String, dynamic> vars, {
    String? confirm,
    // Pointer-moving mutations (switch/create/accept/leave-active) change
    // which household every other screen sees — reset the normalized
    // cache so nothing serves stale household-scoped data.
    bool resetCache = false,
  }) async {
    final client = GraphQLProvider.of(context).value;
    if (confirm != null) {
      final ok = await showDialog<bool>(
        context: context,
        builder: (ctx) => AlertDialog(
          content: Text(confirm),
          actions: [
            TextButton(
              onPressed: () => Navigator.pop(ctx, false),
              child: const Text('Cancel'),
            ),
            FilledButton(
              onPressed: () => Navigator.pop(ctx, true),
              child: const Text('Confirm'),
            ),
          ],
        ),
      );
      if (ok != true || !mounted) return;
    }
    final result = await client.mutate(
      MutationOptions(document: gql(doc), variables: vars),
    );
    if (!mounted) return;
    if (result.hasException) {
      setState(() {
        _error = result.exception?.graphqlErrors.isNotEmpty == true
            ? result.exception!.graphqlErrors.first.message
            : 'Request failed';
      });
    } else {
      setState(() => _error = null);
      if (resetCache) client.cache.store.reset();
      _refetch?.call();
    }
  }

  // Set or clear one of the caller's allergy/dietary records, then
  // refresh the allergy query (kind is required even when clearing).
  Future<void> _setAllergy(
    String allergenId,
    String value,
    String? current,
    VoidCallback? refetch,
  ) async {
    final vars = value == 'none'
        ? {'allergenId': allergenId, 'kind': current ?? 'allergy', 'on': false}
        : {'allergenId': allergenId, 'kind': value, 'on': true};
    await _mutate(setMyAllergyMutation, vars);
    refetch?.call();
  }

  // Deep link for a notification row: recipe reminders open the recipe,
  // expiry rows open the pantry, event kinds open the event detail. Shared
  // with push-tap routing (notification_links.dart).
  void Function()? _notificationLink(Map<String, dynamic> n) {
    final dest = notificationDestination(n);
    if (dest == null) return null;
    return () => Navigator.push(
          context,
          MaterialPageRoute(builder: (_) => dest),
        );
  }

  Future<void> _addReplacement(String itemId) async {
    final client = GraphQLProvider.of(context).value;
    final result = await client.mutate(
      MutationOptions(
        document: gql(addToGroceryMutation),
        variables: {'itemId': itemId},
      ),
    );
    if (!mounted) return;
    if (result.hasException) {
      setState(() {
        _error = result.exception?.graphqlErrors.isNotEmpty == true
            ? result.exception!.graphqlErrors.first.message
            : 'Request failed';
      });
      return;
    }
    setState(() => _error = null);
    Navigator.push(
      context,
      MaterialPageRoute(builder: (_) => const GroceryListsScreen()),
    );
  }

  // Mirrors the backend matrix: owner acts on admins/members, admin acts
  // on members only, nobody acts on the owner or themselves.
  bool _canManage(String myRole, Map<String, dynamic> member) {
    if (member['isMe'] == true || member['role'] == 'OWNER') return false;
    if (myRole == 'OWNER') return true;
    return myRole == 'ADMIN' && member['role'] == 'MEMBER';
  }

  void _memberMenu(Map<String, dynamic> member, String myRole) {
    final user = member['user'] as Map<String, dynamic>;
    final userId = user['id'] as String;
    final name = _userName(user);
    showModalBottomSheet<void>(
      context: context,
      builder: (ctx) => SafeArea(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            if (myRole == 'OWNER')
              ListTile(
                leading: const Icon(Icons.admin_panel_settings),
                title: Text(
                  member['role'] == 'ADMIN' ? 'Demote to member' : 'Make admin',
                ),
                onTap: () {
                  Navigator.pop(ctx);
                  _mutate(setRoleMutation, {
                    'userId': userId,
                    'role': member['role'] == 'ADMIN' ? 'MEMBER' : 'ADMIN',
                  });
                },
              ),
            if (myRole == 'OWNER')
              ListTile(
                leading: const Icon(Icons.swap_horiz),
                title: const Text('Transfer ownership'),
                onTap: () {
                  Navigator.pop(ctx);
                  _mutate(
                    transferMutation,
                    {'userId': userId},
                    confirm:
                        'Transfer ownership to $name? You will become a regular member.',
                  );
                },
              ),
            ListTile(
              leading: const Icon(Icons.person_remove),
              title: const Text('Remove from household'),
              onTap: () {
                Navigator.pop(ctx);
                _mutate(
                  removeMemberMutation,
                  {'userId': userId},
                  confirm:
                      'Remove $name from the household? They get a fresh empty pantry; their shared data stays here.',
                );
              },
            ),
          ],
        ),
      ),
    );
  }

  static String _householdName(Map<String, dynamic> h) =>
      (h['name'] as String?) ?? 'Unnamed household';

  // A household the caller solely owns (single member + OWNER) can be
  // merged into a household they join — see _acceptInvite.
  static Map<String, dynamic>? _mergeSource(
    List<Map<String, dynamic>> households,
  ) {
    for (final h in households) {
      final members = (h['members'] as List? ?? []);
      if (h['myRole'] == 'OWNER' && members.length == 1) return h;
    }
    return null;
  }

  // Switch + per-household leave + new-household entry point.
  void _householdSwitcher(List<Map<String, dynamic>> households) {
    showModalBottomSheet<void>(
      context: context,
      builder: (ctx) => SafeArea(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            for (final h in households)
              ListTile(
                leading: h['isActive'] == true
                    ? Icon(Icons.check_circle,
                        color: Theme.of(ctx).colorScheme.primary)
                    : const Icon(Icons.home_outlined),
                title: Text(_householdName(h)),
                subtitle: Text(
                  '${(h['members'] as List? ?? []).length} member(s) · '
                  '${(h['myRole'] as String? ?? 'MEMBER').toLowerCase()}',
                ),
                trailing: h['isActive'] == true
                    ? const StatusChip(
                        label: 'Active',
                        tone: StatusTone.primary,
                      )
                    : Row(
                        mainAxisSize: MainAxisSize.min,
                        children: [
                          TextButton(
                            onPressed: () {
                              Navigator.pop(ctx);
                              _mutate(
                                setActiveHouseholdMutation,
                                {'householdId': h['id']},
                                resetCache: true,
                              );
                            },
                            child: const Text('Switch'),
                          ),
                          IconButton(
                            tooltip: 'Leave ${_householdName(h)}',
                            icon: Icon(
                              Icons.logout,
                              color: Theme.of(ctx).colorScheme.error,
                            ),
                            onPressed: () {
                              Navigator.pop(ctx);
                              _mutate(
                                leaveMutation,
                                {'householdId': h['id']},
                                confirm:
                                    'Leave ${_householdName(h)}? Your shared data stays with the household.',
                                resetCache: true,
                              );
                            },
                          ),
                        ],
                      ),
              ),
            const Divider(height: 1),
            ListTile(
              leading: const Icon(Icons.add_home_outlined),
              title: const Text('New household'),
              onTap: () {
                Navigator.pop(ctx);
                _newHouseholdDialog();
              },
            ),
          ],
        ),
      ),
    );
  }

  void _newHouseholdDialog() {
    final ctrl = TextEditingController();
    showDialog<void>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('New household'),
        content: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            const Text(
              'Creates a separate pantry, cellar, meal plans, and grocery '
              'lists, and switches you to it.',
            ),
            const SizedBox(height: 12),
            TextField(
              controller: ctrl,
              maxLength: 100,
              decoration: const InputDecoration(
                labelText: 'Household name',
                counterText: '',
              ),
            ),
          ],
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(ctx),
            child: const Text('Cancel'),
          ),
          FilledButton(
            onPressed: () {
              Navigator.pop(ctx);
              final name = ctrl.text.trim();
              _mutate(
                createHouseholdMutation,
                {'name': name.isEmpty ? null : name},
                resetCache: true,
              );
            },
            child: const Text('Create'),
          ),
        ],
      ),
    );
  }

  // Accepts an invite. When the caller solely owns another household, a
  // prompt offers merge-and-dissolve; otherwise a plain confirm.
  void _acceptInvite(
    Map<String, dynamic> inv,
    Map<String, dynamic>? mergeSource,
  ) {
    final inviteId = inv['id'] as String;
    if (mergeSource == null) {
      _mutate(acceptInviteMutation, {'inviteId': inviteId}, resetCache: true);
      return;
    }
    final sourceName = _householdName(mergeSource);
    showDialog<void>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Join this household?'),
        content: Text(
          'You can merge $sourceName into the new household — its pantry, '
          'cellar, meal plans, and grocery lists move over and the empty '
          'household is deleted. Or just join and keep it as a separate '
          'household.',
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(ctx),
            child: const Text('Cancel'),
          ),
          TextButton(
            onPressed: () {
              Navigator.pop(ctx);
              _mutate(
                acceptInviteMutation,
                {'inviteId': inviteId},
                resetCache: true,
              );
            },
            child: const Text('Just join'),
          ),
          FilledButton(
            onPressed: () {
              Navigator.pop(ctx);
              _mutate(
                acceptInviteMutation,
                {
                  'inviteId': inviteId,
                  'mergeFromHouseholdId': mergeSource['id'],
                },
                resetCache: true,
              );
            },
            child: const Text('Join and merge'),
          ),
        ],
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    return Query(
      options: QueryOptions(
        document: gql(householdQuery),
        variables: {'term': _searchTerm, 'search': _searchTerm.length >= 2},
      ),
      builder: (result, {refetch, fetchMore}) {
        _refetch = refetch;
        return Scaffold(
          appBar: AppBar(
            title: const Text('Household'),
            actions: [
              IconButton(
                tooltip: 'Notification settings',
                icon: const Icon(Icons.settings_outlined),
                onPressed: () => Navigator.push(
                  context,
                  MaterialPageRoute(
                    builder: (_) => const NotificationSettingsScreen(),
                  ),
                ),
              ),
            ],
          ),
          body: _body(context, result),
        );
      },
    );
  }

  Widget _body(BuildContext context, QueryResult result) {
    if (result.isLoading && result.data == null) {
      return const SkeletonList();
    }
    if (result.hasException && result.data == null) {
      return Center(child: Text('Error: ${result.exception}'));
    }

    final me = result.data?['me'] as Map<String, dynamic>? ?? {};
    final myId = me['id'] as String?;
    final isSearchable = me['isSearchable'] as bool? ?? true;
    final household = result.data?['myHousehold'] as Map<String, dynamic>?;
    final households = (result.data?['myHouseholds'] as List? ?? [])
        .cast<Map<String, dynamic>>();
    final mergeSource = _mergeSource(households);
    final members =
        (household?['members'] as List? ?? []).cast<Map<String, dynamic>>();
    final myRole = household?['myRole'] as String? ?? 'MEMBER';
    final isOwner = myRole == 'OWNER';
    final invites = (result.data?['householdInvites'] as List? ?? [])
        .cast<Map<String, dynamic>>();
    final incoming =
        invites.where((i) => (i['toUser'] as Map?)?['id'] == myId).toList();
    final outgoing =
        invites.where((i) => (i['fromUser'] as Map?)?['id'] == myId).toList();
    final searchResults = (result.data?['searchHouseholdUsers'] as List? ?? [])
        .cast<Map<String, dynamic>>();
    final notifications = (result.data?['myNotifications'] as List? ?? [])
        .cast<Map<String, dynamic>>();
    final unread = result.data?['unreadNotificationCount'] as int? ?? 0;
    final atCap = members.length >= maxHouseholdMembers;

    // Mark-read-on-open, matching the web bell: once the feed has been
    // displayed, clear the unread set. Runs once per screen mount so the
    // 5s poll doesn't keep re-marking. Deferred past the build phase —
    // _mutate touches GraphQLProvider and setState.
    if (!_markedReadOnOpen) {
      _markedReadOnOpen = true;
      if (unread > 0) {
        WidgetsBinding.instance.addPostFrameCallback((_) {
          _mutate(markAllReadMutation, const {});
        });
      }
    }

    if (_nameCtrl.text.isEmpty && household?['name'] != null) {
      _nameCtrl.text = household!['name'] as String;
    }

    return RefreshIndicator(
      onRefresh: () async => _refetch?.call(),
      child: ListView(
        padding: const EdgeInsets.all(16.0),
        children: [
          if (_error != null)
            Card(
              color: Theme.of(context).colorScheme.errorContainer,
              child: Padding(
                padding: const EdgeInsets.all(12.0),
                child: Text(_error!),
              ),
            ),
          if (household != null)
            Card(
              child: ListTile(
                leading: const Icon(Icons.home_outlined),
                title: Text(_householdName(household)),
                subtitle: const Text('Your active household'),
                trailing: TextButton(
                  onPressed: households.isEmpty
                      ? null
                      : () => _householdSwitcher(households),
                  child: const Text('Switch'),
                ),
              ),
            ),
          if (isOwner)
            Padding(
              padding: const EdgeInsets.only(bottom: 8.0),
              child: Row(
                children: [
                  Expanded(
                    child: TextField(
                      controller: _nameCtrl,
                      maxLength: 100,
                      decoration: InputDecoration(
                        labelText: 'Household name',
                        counterText: '',
                        // Inline save — a floating trailing icon detached
                        // from the field read as unconnected.
                        suffixIcon: IconButton(
                          icon: const Icon(Icons.save),
                          tooltip: 'Save household name',
                          onPressed: () =>
                              _mutate(renameMutation, {'name': _nameCtrl.text}),
                        ),
                      ),
                    ),
                  ),
                ],
              ),
            )
          else if (household?['name'] != null)
            Padding(
              padding: const EdgeInsets.only(bottom: 8.0),
              child: Text(
                household!['name'] as String,
                style: Theme.of(context).textTheme.titleLarge,
              ),
            ),
          if (notifications.isNotEmpty) ...[
            Row(
              children: [
                Expanded(
                  child: Text(
                    'Notifications',
                    style: Theme.of(context).textTheme.titleMedium,
                  ),
                ),
                Badge(
                  isLabelVisible: unread > 0,
                  label: Text('$unread'),
                  child: const Icon(Icons.notifications),
                ),
              ],
            ),
            ...notifications.map((n) {
              final ago = _timeAgo(n['createdAt'] as String?);
              // Scheduled reminders carry server-rendered text; event
              // kinds fall back to client-side strings.
              final title = n['title'] as String? ?? _notificationText(n);
              final body = n['body'] as String? ?? '';
              final itemId = n['itemId'] as String?;
              return Card(
                child: ListTile(
                  dense: true,
                  leading: const Icon(Icons.notifications_outlined),
                  title: Text(title),
                  subtitle: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      // Server-rendered bodies sometimes repeat the title
                      // verbatim — don't print it twice.
                      if (body.isNotEmpty && body != title) Text(body),
                      if (ago.isNotEmpty) Text(ago),
                    ],
                  ),
                  trailing: n['kind'] == 'ITEM_EXPIRING' && itemId != null
                      ? TextButton(
                          onPressed: () => _addReplacement(itemId),
                          child: const Text('Add to list'),
                        )
                      : null,
                  onTap: _notificationLink(n),
                ),
              );
            }),
            const SizedBox(height: 8),
          ],
          Text(
            'Members — ${members.length} of $maxHouseholdMembers',
            style: Theme.of(context).textTheme.titleMedium,
          ),
          ...members.map((m) {
            final user = m['user'] as Map<String, dynamic>;
            final role = (m['role'] as String? ?? 'MEMBER').toLowerCase();
            final isElevated = role == 'owner' || role == 'admin';
            return Card(
              child: ListTile(
                title: Text(_userName(user)),
                subtitle: Row(
                  children: [
                    StatusChip(
                      label: role,
                      tone: isElevated
                          ? StatusTone.primary
                          : StatusTone.neutral,
                    ),
                    if (m['isMe'] == true) ...[
                      const SizedBox(width: 6),
                      Text('you',
                          style: Theme.of(context).textTheme.bodySmall),
                    ],
                  ],
                ),
                trailing: _canManage(myRole, m)
                    ? IconButton(
                        icon: const Icon(Icons.more_vert),
                        onPressed: () => _memberMenu(m, myRole),
                      )
                    : null,
              ),
            );
          }),
          const SizedBox(height: 16),
          Text(
            'Allergies & dietary restrictions',
            style: Theme.of(context).textTheme.titleMedium,
          ),
          Text(
            'Warnings on recipes, meal plans, events, and grocery lists '
            'fire for every household member\'s records.',
            style: Theme.of(context).textTheme.bodySmall,
          ),
          Query(
            options: QueryOptions(document: gql(myAllergiesQuery)),
            builder: (aResult, {refetch, fetchMore}) {
              if (aResult.isLoading && aResult.data == null) {
                return const SkeletonCard();
              }
              final allergens = ((aResult.data?['allergens'] as List? ?? [])
                      .cast<Map<String, dynamic>>())
                  .where((a) => a['isActive'] == true)
                  .toList();
              final mine = <String, String>{
                for (final m in (aResult.data?['myAllergies'] as List? ?? [])
                    .cast<Map<String, dynamic>>())
                  (m['allergen'] as Map)['id'] as String: m['kind'] as String,
              };
              if (allergens.isEmpty) {
                return const Padding(
                  padding: EdgeInsets.symmetric(vertical: 8.0),
                  child: Text('No allergens are registered yet.'),
                );
              }
              return Column(
                children: [
                  for (final a in allergens)
                    Padding(
                      padding: const EdgeInsets.symmetric(vertical: 2.0),
                      child: Row(
                        children: [
                          Expanded(child: Text(a['name'] as String)),
                          SegmentedButton<String>(
                            segments: const [
                              ButtonSegment(value: 'none', label: Text('None')),
                              ButtonSegment(
                                  value: 'dietary', label: Text('Dietary')),
                              ButtonSegment(
                                  value: 'allergy', label: Text('Allergy')),
                            ],
                            selected: {mine[a['id']] ?? 'none'},
                            showSelectedIcon: false,
                            style: const ButtonStyle(
                              visualDensity: VisualDensity.compact,
                              tapTargetSize: MaterialTapTargetSize.shrinkWrap,
                            ),
                            onSelectionChanged: (sel) => _setAllergy(
                              a['id'] as String,
                              sel.first,
                              mine[a['id']],
                              refetch,
                            ),
                          ),
                        ],
                      ),
                    ),
                ],
              );
            },
          ),
          if (members.length > 1)
            TextButton.icon(
              icon: Icon(Icons.logout,
                  color: Theme.of(context).colorScheme.error),
              label: Text(
                'Leave household',
                style: TextStyle(color: Theme.of(context).colorScheme.error),
              ),
              onPressed: () => _mutate(
                leaveMutation,
                {'householdId': household?['id']},
                confirm:
                    'Leave this household? You will get a fresh empty pantry, cellar, meal plans, and grocery lists; your data stays with the household.',
                resetCache: true,
              ),
            ),
          const SizedBox(height: 16),
          SwitchListTile(
            title: const Text('Discoverable in member search'),
            subtitle: const Text(
              'Other users can find and invite you by name or email.',
            ),
            value: isSearchable,
            onChanged: (v) => _mutate(updateProfileMutation, {
              'input': {'isSearchable': v},
            }),
          ),
          const SizedBox(height: 16),
          if (incoming.isNotEmpty) ...[
            Text('Invitations', style: Theme.of(context).textTheme.titleMedium),
            ...incoming.map((inv) {
              final from = inv['fromUser'] as Map<String, dynamic>;
              return Card(
                child: ListTile(
                  title: Text('${_userName(from)} invited you'),
                  subtitle: const Text(
                    'Joining makes theirs your active household.',
                  ),
                  trailing: Wrap(
                    spacing: 8,
                    children: [
                      IconButton(
                        icon: Icon(Icons.check,
                            color: Theme.of(context).colorScheme.primary),
                        onPressed: () => _acceptInvite(inv, mergeSource),
                      ),
                      IconButton(
                        icon: Icon(Icons.close,
                            color: Theme.of(context).colorScheme.error),
                        onPressed: () => _mutate(
                          declineInviteMutation,
                          {'inviteId': inv['id']},
                        ),
                      ),
                    ],
                  ),
                ),
              );
            }),
            const SizedBox(height: 8),
          ],
          if (outgoing.isNotEmpty) ...[
            Text(
              'Sent invitations',
              style: Theme.of(context).textTheme.titleMedium,
            ),
            ...outgoing.map((inv) {
              final to = inv['toUser'] as Map<String, dynamic>;
              return Card(
                child: ListTile(
                  title: Text(_userName(to)),
                  subtitle: const Text('Pending'),
                  trailing: IconButton(
                    icon: const Icon(Icons.cancel_outlined),
                    onPressed: () => _mutate(
                      cancelInviteMutation,
                      {'inviteId': inv['id']},
                    ),
                  ),
                ),
              );
            }),
            const SizedBox(height: 8),
          ],
          Text('Invite someone',
              style: Theme.of(context).textTheme.titleMedium),
          if (atCap)
            const Padding(
              padding: EdgeInsets.symmetric(vertical: 4.0),
              child: Text('This household is at the 10-member limit.'),
            ),
          Row(
            children: [
              Expanded(
                child: TextField(
                  controller: _searchCtrl,
                  decoration: const InputDecoration(
                    labelText: 'Name or email',
                  ),
                  onSubmitted: (v) => setState(() => _searchTerm = v.trim()),
                ),
              ),
              IconButton(
                icon: const Icon(Icons.search),
                onPressed: () =>
                    setState(() => _searchTerm = _searchCtrl.text.trim()),
              ),
            ],
          ),
          if (_searchCtrl.text.trim().isNotEmpty && _searchTerm.length < 2)
            const Padding(
              padding: EdgeInsets.symmetric(vertical: 4.0),
              child: Text('Type at least 2 characters to search.'),
            ),
          ...searchResults.map((u) {
            return Card(
              child: ListTile(
                title: Text(_userName(u)),
                trailing: FilledButton(
                  onPressed: atCap
                      ? null
                      : () => _mutate(inviteMutation, {'userId': u['id']}),
                  child: const Text('Invite'),
                ),
              ),
            );
          }),
          if (_searchTerm.length >= 2 && searchResults.isEmpty)
            const Padding(
              padding: EdgeInsets.symmetric(vertical: 8.0),
              child: Text('No users found.'),
            ),
        ],
      ),
    );
  }
}
