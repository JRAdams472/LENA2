import 'package:flutter/material.dart';

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
    final color = severe ? Colors.red.shade700 : Colors.orange.shade800;
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
        child: Icon(Icons.warning_amber_rounded, color: color, size: 20),
      ),
    );
  }
}
