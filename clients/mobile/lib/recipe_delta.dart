// Household recipe-delta draft model (LEN-25).
//
// One delta per recipe+household; a change set of line/step tweaks applied
// over the canonical recipe. Drafts here are mutable UI state — convert a
// `householdDelta` GraphQL payload with [DeltaItemDraft.fromRow] /
// [DeltaStepDraft.fromRow], let the tweak sheets edit them, and serialize
// back with [toDeltaItemInput] / [toDeltaStepInput] for `setRecipeDelta`.

/// Kind labels for the badge chips on effective recipe lines/steps.
const Map<String, String> deltaKindLabels = {
  'substitute': 'Swapped',
  'adjust': 'Adjusted',
  'add': 'Added',
  'remove': 'Removed',
  'replace': 'Replaced',
};

/// Item-line tweak: swap the target, adjust qty/unit/notes, remove the
/// line, or add a new one (no anchor for adds).
class DeltaItemDraft {
  DeltaItemDraft({
    this.id,
    this.recipeItemId,
    required this.kind,
    this.itemId,
    this.ingredientId,
    this.itemName,
    this.ingredientName,
    this.quantity,
    this.unit,
    this.unitId,
    this.section,
    this.displayOrder,
    this.notes,
    this.isOptional,
    this.orphaned = false,
  });

  factory DeltaItemDraft.fromRow(Map<String, dynamic> r) {
    final item = r['item'] as Map<String, dynamic>?;
    final ingredient = r['ingredient'] as Map<String, dynamic>?;
    return DeltaItemDraft(
      id: r['id'] as String?,
      recipeItemId: r['recipeItemId'] as String?,
      kind: r['kind'] as String,
      itemId: item?['id'] as String?,
      ingredientId: ingredient?['id'] as String?,
      itemName: item?['name'] as String?,
      ingredientName: ingredient?['name'] as String?,
      quantity: (r['quantity'] as num?)?.toDouble(),
      unit: r['unit'] as String?,
      unitId: r['unitId'] as String?,
      section: r['section'] as String?,
      displayOrder: (r['displayOrder'] as num?)?.toInt(),
      notes: r['notes'] as String?,
      isOptional: r['isOptional'] as bool?,
      orphaned: r['orphaned'] as bool? ?? false,
    );
  }

  final String? id;
  String? recipeItemId;
  String kind;
  String? itemId;
  String? ingredientId;
  String? itemName;
  String? ingredientName;
  double? quantity;
  String? unit;
  String? unitId;
  String? section;
  int? displayOrder;
  String? notes;
  bool? isOptional;
  bool orphaned;

  /// Display name of the swap/add target (brand item or ingredient).
  String get targetLabel =>
      ingredientName ?? itemName ?? (ingredientId != null || itemId != null
          ? 'selected item'
          : 'ingredient');
}

/// Step tweak: replace a step's text/meta, remove it, or add a new step
/// at a position (no anchor for adds).
class DeltaStepDraft {
  DeltaStepDraft({
    this.id,
    this.stepId,
    required this.kind,
    this.stepNumber,
    this.instruction,
    this.durationMinutes,
    this.stepType,
    this.isPassive,
    this.dependsOnStepNumber,
    this.appliance,
    this.orphaned = false,
  });

  factory DeltaStepDraft.fromRow(Map<String, dynamic> r) {
    return DeltaStepDraft(
      id: r['id'] as String?,
      stepId: r['stepId'] as String?,
      kind: r['kind'] as String,
      stepNumber: (r['stepNumber'] as num?)?.toInt(),
      instruction: r['instruction'] as String?,
      durationMinutes: (r['durationMinutes'] as num?)?.toInt(),
      stepType: r['stepType'] as String?,
      isPassive: r['isPassive'] as bool?,
      dependsOnStepNumber: (r['dependsOnStepNumber'] as num?)?.toInt(),
      appliance: r['appliance'] as String?,
      orphaned: r['orphaned'] as bool? ?? false,
    );
  }

  final String? id;
  String? stepId;
  String kind;
  int? stepNumber;
  String? instruction;
  int? durationMinutes;
  String? stepType;
  bool? isPassive;
  int? dependsOnStepNumber;
  String? appliance;
  bool orphaned;
}

/// Human-readable summary shown in the tweaks list, e.g.
/// "Swap garlic for shallot", "Adjust pasta — 0.5 lb".
String describeItemChange(DeltaItemDraft d, String? baseLabel) {
  final base = (baseLabel == null || baseLabel.isEmpty)
      ? 'the original line'
      : baseLabel;
  switch (d.kind) {
    case 'substitute':
      return 'Swap $base for ${d.targetLabel}';
    case 'adjust':
      final qty = d.quantity;
      final bits = [
        if (qty != null) '$qty',
        if (d.unit != null && d.unit!.isNotEmpty) d.unit!,
      ].join(' ');
      return bits.isEmpty ? 'Adjust $base' : 'Adjust $base — $bits';
    case 'remove':
      return 'Remove $base';
    case 'add':
      return 'Add ${d.targetLabel}';
    default:
      return 'Tweak $base';
  }
}

String describeStepChange(DeltaStepDraft d, int? baseNumber) {
  final base = baseNumber == null ? 'the original step' : 'step $baseNumber';
  switch (d.kind) {
    case 'replace':
      return 'Replace $base';
    case 'remove':
      return 'Remove $base';
    case 'add':
      return d.stepNumber == null
          ? 'Add a step'
          : 'Add a step at ${d.stepNumber}';
    default:
      return 'Tweak $base';
  }
}

/// Serializes drafts to `setRecipeDelta` variables. Only fields relevant
/// to each kind are sent — removed lines carry just their anchor.
Map<String, dynamic> toDeltaItemInput(DeltaItemDraft d) {
  final input = <String, dynamic>{
    'recipeItemId': d.recipeItemId,
    'kind': d.kind,
  };
  if (d.kind != 'remove') {
    input['itemId'] = d.itemId;
    input['ingredientId'] = d.ingredientId;
    input['quantity'] = d.quantity;
    input['unitId'] = d.unitId;
    input['section'] = d.section;
    input['displayOrder'] = d.displayOrder;
    input['notes'] = d.notes;
    input['isOptional'] = d.isOptional;
  }
  return input;
}

Map<String, dynamic> toDeltaStepInput(DeltaStepDraft d) {
  final input = <String, dynamic>{
    'stepId': d.stepId,
    'kind': d.kind,
  };
  if (d.kind != 'remove') {
    input['stepNumber'] = d.stepNumber;
    input['instruction'] = d.instruction;
    input['durationMinutes'] = d.durationMinutes;
    input['stepType'] = d.stepType;
    input['isPassive'] = d.isPassive;
    input['dependsOnStepNumber'] = d.dependsOnStepNumber;
    input['appliance'] = d.appliance;
  }
  return input;
}

/// True when two draft lists describe the same change set — powers the
/// dirty check for Save/Discard. Order matters (server stores rows).
bool itemDraftsEqual(List<DeltaItemDraft> a, List<DeltaItemDraft> b) {
  if (a.length != b.length) return false;
  for (var i = 0; i < a.length; i++) {
    final x = a[i];
    final y = b[i];
    if (x.recipeItemId != y.recipeItemId ||
        x.kind != y.kind ||
        x.itemId != y.itemId ||
        x.ingredientId != y.ingredientId ||
        x.quantity != y.quantity ||
        x.unitId != y.unitId ||
        x.section != y.section ||
        x.displayOrder != y.displayOrder ||
        x.notes != y.notes ||
        x.isOptional != y.isOptional) {
      return false;
    }
  }
  return true;
}

bool stepDraftsEqual(List<DeltaStepDraft> a, List<DeltaStepDraft> b) {
  if (a.length != b.length) return false;
  for (var i = 0; i < a.length; i++) {
    final x = a[i];
    final y = b[i];
    if (x.stepId != y.stepId ||
        x.kind != y.kind ||
        x.stepNumber != y.stepNumber ||
        x.instruction != y.instruction ||
        x.durationMinutes != y.durationMinutes ||
        x.stepType != y.stepType ||
        x.isPassive != y.isPassive ||
        x.dependsOnStepNumber != y.dependsOnStepNumber ||
        x.appliance != y.appliance) {
      return false;
    }
  }
  return true;
}
