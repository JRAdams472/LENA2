import 'dart:async';

import 'package:flutter/material.dart';
import 'package:graphql_flutter/graphql_flutter.dart';

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
  mutation AcceptInvite($inviteId: ID!) {
    acceptHouseholdInvite(inviteId: $inviteId) { id }
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
  mutation LeaveHousehold {
    leaveHousehold
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

String _notificationText(Map<String, dynamic> n) {
  final actor = n['actor'] as Map<String, dynamic>?;
  final who =
      actor == null ? 'Someone' : _userName(actor);
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
      _refetch?.call();
    }
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
          appBar: AppBar(title: const Text('Household')),
          body: _body(context, result),
        );
      },
    );
  }

  Widget _body(BuildContext context, QueryResult result) {
    if (result.isLoading && result.data == null) {
      return const Center(child: CircularProgressIndicator());
    }
    if (result.hasException && result.data == null) {
      return Center(child: Text('Error: ${result.exception}'));
    }

    final me = result.data?['me'] as Map<String, dynamic>? ?? {};
    final myId = me['id'] as String?;
    final isSearchable = me['isSearchable'] as bool? ?? true;
    final household = result.data?['myHousehold'] as Map<String, dynamic>?;
    final members =
        (household?['members'] as List? ?? []).cast<Map<String, dynamic>>();
    final myRole = household?['myRole'] as String? ?? 'MEMBER';
    final isOwner = myRole == 'OWNER';
    final invites =
        (result.data?['householdInvites'] as List? ?? [])
            .cast<Map<String, dynamic>>();
    final incoming = invites
        .where((i) => (i['toUser'] as Map?)?['id'] == myId)
        .toList();
    final outgoing = invites
        .where((i) => (i['fromUser'] as Map?)?['id'] == myId)
        .toList();
    final searchResults =
        (result.data?['searchHouseholdUsers'] as List? ?? [])
            .cast<Map<String, dynamic>>();
    final notifications =
        (result.data?['myNotifications'] as List? ?? [])
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
          if (isOwner)
            Padding(
              padding: const EdgeInsets.only(bottom: 8.0),
              child: Row(
                children: [
                  Expanded(
                    child: TextField(
                      controller: _nameCtrl,
                      maxLength: 100,
                      decoration: const InputDecoration(
                        labelText: 'Household name',
                        counterText: '',
                      ),
                    ),
                  ),
                  IconButton(
                    icon: const Icon(Icons.save),
                    onPressed: () =>
                        _mutate(renameMutation, {'name': _nameCtrl.text}),
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
              return ListTile(
                dense: true,
                leading: const Icon(Icons.notifications_outlined),
                title: Text(_notificationText(n)),
                subtitle: ago.isEmpty ? null : Text(ago),
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
            return Card(
              child: ListTile(
                title: Text(_userName(user)),
                subtitle: Text(m['isMe'] == true ? '$role — you' : role),
                trailing: _canManage(myRole, m)
                    ? IconButton(
                        icon: const Icon(Icons.more_vert),
                        onPressed: () => _memberMenu(m, myRole),
                      )
                    : null,
              ),
            );
          }),
          if (members.length > 1)
            TextButton.icon(
              icon: const Icon(Icons.logout, color: Colors.red),
              label: const Text(
                'Leave household',
                style: TextStyle(color: Colors.red),
              ),
              onPressed: () => _mutate(
                leaveMutation,
                const {},
                confirm:
                    'Leave this household? You will get a fresh empty pantry, cellar, meal plans, and grocery lists; your data stays with the household.',
              ),
            ),
          const SizedBox(height: 16),
          SwitchListTile(
            title: const Text('Discoverable in member search'),
            subtitle: const Text(
              'Other users can find and invite you by name or email.',
            ),
            value: isSearchable,
            onChanged: (v) =>
                _mutate(updateProfileMutation, {
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
                    'Joining merges your pantry, cellar, meal plans, and grocery lists into theirs.',
                  ),
                  trailing: Wrap(
                    spacing: 8,
                    children: [
                      IconButton(
                        icon: const Icon(Icons.check, color: Colors.green),
                        onPressed: () => _mutate(
                          acceptInviteMutation,
                          {'inviteId': inv['id']},
                        ),
                      ),
                      IconButton(
                        icon: const Icon(Icons.close, color: Colors.red),
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
          Text('Invite someone', style: Theme.of(context).textTheme.titleMedium),
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
                  onSubmitted: (v) =>
                      setState(() => _searchTerm = v.trim()),
                ),
              ),
              IconButton(
                icon: const Icon(Icons.search),
                onPressed: () =>
                    setState(() => _searchTerm = _searchCtrl.text.trim()),
              ),
            ],
          ),
          if (_searchCtrl.text.trim().isNotEmpty &&
              _searchTerm.length < 2)
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
                      : () =>
                          _mutate(inviteMutation, {'userId': u['id']}),
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
