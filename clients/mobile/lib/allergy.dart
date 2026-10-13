import 'package:flutter/material.dart';
import 'package:graphql_flutter/graphql_flutter.dart';
import 'screens/disclaimer_screen.dart';

// The GraphQL field block for entity allergen flags + household-member
// conflicts. Paste inside an entity's selection set — screens use raw
// strings (GraphQL $vars), so it can't be interpolated into queries.
//
//   allergyWarnings {
//     memberKind
//     entityKind
//     member { id displayName firstName lastName }
//     allergen { id name }
//   }
//   allergens {
//     kind
//     allergen { id name }
//   }
const String allergyWarningFieldBlock = r'''
  allergyWarnings {
    memberKind
    entityKind
    member { id displayName firstName lastName }
    allergen { id name }
  }
  allergens {
    kind
    allergen { id name }
  }
''';

const String myAllergiesQuery = r'''
  query MyAllergies {
    allergens { id name description isActive }
    myAllergies {
      kind
      allergen { id name }
    }
  }
''';

const String setMyAllergyMutation = r'''
  mutation SetMyAllergy($allergenId: ID!, $kind: MemberAllergyKind!, $on: Boolean!) {
    setMyAllergy(allergenId: $allergenId, kind: $kind, on: $on)
  }
''';

const String acceptDisclaimerMutation = r'''
  mutation AcceptDisclaimer {
    acceptDisclaimer
  }
''';

// LEN-31: the server refuses setMyAllergy(on: true) until the medical &
// allergen disclaimer is on file. Match the extension code, not the
// human-readable message.
bool hasDisclaimerRequired(OperationException? e) =>
    (e?.graphqlErrors ?? const <GraphQLError>[]).any(
      (g) => g.extensions?['code'] == 'DISCLAIMER_REQUIRED',
    );

// The one-time consent gate for recording allergies/dietary records.
// Self-contained — runs acceptDisclaimer itself. Resolves true when the
// disclaimer is on file, false when the user dismisses or acceptance fails.
Future<bool> showDisclaimerConsent(BuildContext context) async =>
    await showModalBottomSheet<bool>(
      context: context,
      isScrollControlled: true,
      builder: (ctx) => const _DisclaimerConsentSheet(),
    ) ==
    true;

class _DisclaimerConsentSheet extends StatefulWidget {
  const _DisclaimerConsentSheet();

  @override
  State<_DisclaimerConsentSheet> createState() =>
      _DisclaimerConsentSheetState();
}

class _DisclaimerConsentSheetState extends State<_DisclaimerConsentSheet> {
  bool _busy = false;
  String? _error;

  Future<void> _accept() async {
    setState(() {
      _busy = true;
      _error = null;
    });
    final result = await GraphQLProvider.of(context).value.mutate(
          MutationOptions(document: gql(acceptDisclaimerMutation)),
        );
    if (!mounted) return;
    if (result.hasException && result.data?['acceptDisclaimer'] != true) {
      setState(() {
        _busy = false;
        _error = result.exception?.graphqlErrors.isNotEmpty == true
            ? result.exception!.graphqlErrors.first.message
            : 'Could not record acceptance';
      });
      return;
    }
    Navigator.pop(context, true);
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return SafeArea(
      child: Padding(
        padding: const EdgeInsets.fromLTRB(20, 20, 20, 16),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text('Medical & allergen disclaimer',
                style: theme.textTheme.titleLarge),
            const SizedBox(height: 12),
            Text(
              "Allergen and dietary information in Lena's Kitchen is a "
              'convenience, not medical, nutritional, or dietary advice. '
              'Records may be incomplete, inaccurate, or out of date — '
              "always check the product's actual label, and consult a "
              'qualified healthcare professional about allergies or '
              'medical conditions.',
              style: theme.textTheme.bodyMedium,
            ),
            const SizedBox(height: 8),
            Text(
              'By accepting you acknowledge this and use these features at '
              'your own risk.',
              style: theme.textTheme.bodyMedium,
            ),
            TextButton(
              onPressed: () => Navigator.push(
                context,
                MaterialPageRoute(
                  builder: (_) => const DisclaimerScreen(),
                ),
              ),
              child: const Text('Read the full disclaimer'),
            ),
            if (_error != null)
              Padding(
                padding: const EdgeInsets.only(top: 4),
                child: Text(
                  _error!,
                  style: theme.textTheme.bodySmall
                      ?.copyWith(color: theme.colorScheme.error),
                ),
              ),
            const SizedBox(height: 8),
            SizedBox(
              width: double.infinity,
              child: FilledButton(
                onPressed: _busy ? null : _accept,
                child: Text(_busy ? 'Accepting…' : 'I understand and accept'),
              ),
            ),
            Center(
              child: TextButton(
                onPressed: _busy ? null : () => Navigator.pop(context),
                child: const Text('Not now'),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

// Compact "check the label" line for recipe and cook surfaces. Tapping it
// opens the full disclaimer.
class DisclaimerCaption extends StatelessWidget {
  const DisclaimerCaption({super.key});

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return InkWell(
      onTap: () => Navigator.push(
        context,
        MaterialPageRoute(builder: (_) => const DisclaimerScreen()),
      ),
      child: Padding(
        padding: const EdgeInsets.symmetric(vertical: 6),
        child: Text.rich(
          TextSpan(
            style: theme.textTheme.bodySmall
                ?.copyWith(color: theme.colorScheme.onSurfaceVariant),
            children: [
              const TextSpan(
                text: 'Allergy and dietary info is a convenience, not '
                    "medical advice — always check the product's label. ",
              ),
              TextSpan(
                text: 'Disclaimer',
                style: TextStyle(color: theme.colorScheme.primary),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

List<Map<String, dynamic>> allergyWarningsOf(Map<String, dynamic>? entity) =>
    (entity?['allergyWarnings'] as List? ?? []).cast<Map<String, dynamic>>();

List<Map<String, dynamic>> allergenFlagsOf(Map<String, dynamic>? entity) =>
    (entity?['allergens'] as List? ?? []).cast<Map<String, dynamic>>();

String _memberName(Map<String, dynamic> w) {
  final member = w['member'] as Map<String, dynamic>? ?? const {};
  final display = member['displayName'] as String?;
  if (display != null && display.isNotEmpty) return display;
  final parts = [member['firstName'], member['lastName']]
      .whereType<String>()
      .where((s) => s.isNotEmpty);
  return parts.isNotEmpty ? parts.join(' ') : 'Household member';
}

// "Ada — Peanuts (allergy; contains)"
String allergyWarningText(Map<String, dynamic> w) {
  final allergen =
      (w['allergen'] as Map<String, dynamic>?)?['name'] ?? 'allergen';
  final memberKind = w['memberKind'] == 'allergy' ? 'allergy' : 'dietary';
  final entityKind = w['entityKind'] == 'contains' ? 'contains' : 'may contain';
  return '${_memberName(w)} — $allergen ($memberKind; $entityKind)';
}

// Confirmed allergen against a true allergy record is the severe case;
// advisory flags and dietary preferences stay at warning level.
bool allergyWarningSevere(List<Map<String, dynamic>> warnings) => warnings
    .any((w) => w['memberKind'] == 'allergy' && w['entityKind'] == 'contains');

// Compact conflict indicator for list rows and cards. Tap for the
// member-level detail; empty renders nothing.
class AllergyWarningBadge extends StatelessWidget {
  const AllergyWarningBadge({super.key, required this.warnings});

  final List<Map<String, dynamic>> warnings;

  @override
  Widget build(BuildContext context) {
    if (warnings.isEmpty) return const SizedBox.shrink();
    final severe = allergyWarningSevere(warnings);
    final scheme = Theme.of(context).colorScheme;
    final color = severe ? scheme.error : scheme.tertiary;
    final labelStyle = Theme.of(context).textTheme.labelSmall?.copyWith(
          color: color,
          fontWeight: FontWeight.w700,
          letterSpacing: 0,
        );
    final lines = warnings.map(allergyWarningText).join('\n');
    return Tooltip(
      message: lines,
      child: InkWell(
        borderRadius: BorderRadius.circular(12),
        onTap: () => showDialog<void>(
          context: context,
          builder: (ctx) => AlertDialog(
            title: const Text('Allergy warnings'),
            content: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                for (final w in warnings)
                  Padding(
                    padding: const EdgeInsets.only(bottom: 4),
                    child: Text(allergyWarningText(w)),
                  ),
              ],
            ),
            actions: [
              TextButton(
                onPressed: () => Navigator.pop(ctx),
                child: const Text('OK'),
              ),
            ],
          ),
        ),
        child: Container(
          padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
          decoration: BoxDecoration(
            color: color.withValues(alpha: 0.12),
            borderRadius: BorderRadius.circular(8),
            border: Border.all(color: color.withValues(alpha: 0.45)),
          ),
          child: Row(
            mainAxisSize: MainAxisSize.min,
            children: [
              Icon(Icons.warning_amber_rounded, color: color, size: 16),
              const SizedBox(width: 2),
              Text('${warnings.length}', style: labelStyle),
            ],
          ),
        ),
      ),
    );
  }
}
