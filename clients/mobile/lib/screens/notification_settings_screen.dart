import 'package:flutter/material.dart';
import 'package:graphql_flutter/graphql_flutter.dart';
import '../responsive.dart';
import '../widgets/skeleton.dart';

const String notificationPrefsQuery = r'''
  query NotificationPrefs {
    myNotificationPreferences {
      category
      label
      enabled
      pushEnabled
      mutedUntil
    }
  }
''';

const String setCategoryEnabledMutation = r'''
  mutation SetCategoryEnabled($category: String!, $enabled: Boolean!) {
    setNotificationCategoryEnabled(category: $category, enabled: $enabled)
  }
''';

const String setCategoryPushEnabledMutation = r'''
  mutation SetCategoryPushEnabled($category: String!, $enabled: Boolean!) {
    setNotificationCategoryPushEnabled(category: $category, enabled: $enabled)
  }
''';

const String muteNotificationsMutation = r'''
  mutation MuteNotifications($category: String, $until: Time!) {
    muteNotifications(category: $category, until: $until)
  }
''';

const String clearMuteMutation = r'''
  mutation ClearNotificationMute($category: String) {
    clearNotificationMute(category: $category)
  }
''';

// Mirrors the web /notifications page presets.
const List<({String label, Duration duration})> _muteOptions = [
  (label: '1 hour', duration: Duration(hours: 1)),
  (label: '8 hours', duration: Duration(hours: 8)),
  (label: '1 day', duration: Duration(days: 1)),
  (label: '1 week', duration: Duration(days: 7)),
];

String _fmtUntil(String? iso) {
  final t = iso == null ? null : DateTime.tryParse(iso);
  if (t == null) return '';
  final local = t.toLocal();
  const months = [
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
  final hour = local.hour % 12 == 0 ? 12 : local.hour % 12;
  final ampm = local.hour < 12 ? 'AM' : 'PM';
  final min = local.minute.toString().padLeft(2, '0');
  return '${months[local.month - 1]} ${local.day}, $hour:$min $ampm';
}

class NotificationSettingsScreen extends StatefulWidget {
  const NotificationSettingsScreen({super.key});

  @override
  State<NotificationSettingsScreen> createState() =>
      _NotificationSettingsScreenState();
}

class _NotificationSettingsScreenState
    extends State<NotificationSettingsScreen> {
  String? _error;
  VoidCallback? _refetch;
  // Snapshot "now" once at mount — a mute expiring while the screen is
  // open is harmless (matches the web page's purity fix).
  final DateTime _mountedAt = DateTime.now();

  Future<void> _mutate(String doc, Map<String, dynamic> vars) async {
    final client = GraphQLProvider.of(context).value;
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

  bool _isMuted(Map<String, dynamic> pref) {
    final until = DateTime.tryParse(pref['mutedUntil'] as String? ?? '');
    return until != null && until.isAfter(_mountedAt);
  }

  void _muteMenu(Map<String, dynamic> pref) {
    final isAll = pref['category'] == '_all';
    showModalBottomSheet<void>(
      context: context,
      builder: (ctx) => SafeArea(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            for (final o in _muteOptions)
              ListTile(
                leading: const Icon(Icons.snooze),
                title: Text('Mute for ${o.label}'),
                onTap: () {
                  Navigator.pop(ctx);
                  _mutate(muteNotificationsMutation, {
                    'category': isAll ? null : pref['category'],
                    'until': DateTime.now()
                        .add(o.duration)
                        .toUtc()
                        .toIso8601String(),
                  });
                },
              ),
          ],
        ),
      ),
    );
  }

  Widget _prefTile(Map<String, dynamic> pref) {
    final isAll = pref['category'] == '_all';
    final enabled = pref['enabled'] as bool? ?? true;
    final pushEnabled = pref['pushEnabled'] as bool? ?? false;
    final muted = _isMuted(pref);
    final label = pref['label'] as String? ?? pref['category'] as String? ?? '';

    final subtitle = muted
        ? 'Muted until ${_fmtUntil(pref['mutedUntil'] as String?)}'
        : isAll
            ? 'Pause every notification for a while — nothing is delivered until it expires.'
            : (enabled ? 'Delivered' : 'Turned off');

    return Card(
      child: Padding(
        padding: const EdgeInsets.symmetric(vertical: 4.0),
        child: Row(
          children: [
            Expanded(
              child: Padding(
                padding: const EdgeInsets.only(left: 16.0),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      isAll ? '$label (global mute)' : label,
                      style: Theme.of(context).textTheme.titleSmall,
                    ),
                    Text(
                      subtitle,
                      style: Theme.of(context).textTheme.bodySmall,
                    ),
                  ],
                ),
              ),
            ),
            if (muted)
              IconButton(
                tooltip: 'Clear mute',
                icon: const Icon(Icons.notifications_active_outlined),
                onPressed: () => _mutate(clearMuteMutation, {
                  'category': isAll ? null : pref['category'],
                }),
              ),
            // A bare bell icon next to the delivery switch read as the
            // same control — label the temporary mute explicitly.
            TextButton.icon(
              icon: const Icon(Icons.snooze_outlined, size: 18),
              label: const Text('Snooze'),
              onPressed: () => _muteMenu(pref),
            ),
            // Push is a second channel — _all is its master switch, so the
            // push toggle shows even where the feed toggle is hidden.
            _labeledSwitch(
              label: 'Push',
              value: pushEnabled,
              onChanged: (v) => _mutate(setCategoryPushEnabledMutation, {
                'category': pref['category'],
                'enabled': v,
              }),
            ),
            if (!isAll)
              _labeledSwitch(
                label: 'Feed',
                value: enabled,
                onChanged: (v) => _mutate(setCategoryEnabledMutation, {
                  'category': pref['category'],
                  'enabled': v,
                }),
              ),
            const SizedBox(width: 8),
          ],
        ),
      ),
    );
  }

  Widget _labeledSwitch({
    required String label,
    required bool value,
    required ValueChanged<bool> onChanged,
  }) {
    return Column(
      mainAxisSize: MainAxisSize.min,
      children: [
        SizedBox(
          height: 36,
          child: Switch(value: value, onChanged: onChanged),
        ),
        Text(label, style: Theme.of(context).textTheme.labelSmall),
      ],
    );
  }

  @override
  Widget build(BuildContext context) {
    return Query(
      options: QueryOptions(document: gql(notificationPrefsQuery)),
      builder: (result, {refetch, fetchMore}) {
        _refetch = refetch;
        return Scaffold(
          appBar: AppBar(title: const Text('Notification settings')),
          body: ConstrainedContent(child: _body(context, result)),
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
    final prefs = (result.data?['myNotificationPreferences'] as List? ?? [])
        .cast<Map<String, dynamic>>();
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
          const Padding(
            padding: EdgeInsets.only(bottom: 8.0),
            child: Text(
              'Choose which notifications reach your feed and which arrive '
              'as pushes. Muting pauses a category for a while without '
              'turning it off; the global mute pauses everything.',
            ),
          ),
          ...prefs.map(_prefTile),
        ],
      ),
    );
  }
}
