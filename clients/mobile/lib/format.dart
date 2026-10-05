/// Shared display helpers — keep user-facing text humanized and
/// deduplicated across screens. Mirrors clients/web/lib/format.ts.
library;

const _monthAbbr = [
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

/// '2026-10-04T18:48:42Z' → 'Oct 4, 2026'. Passes the input through
/// when it isn't a parseable timestamp.
String fmtIso(String? iso) {
  if (iso == null || iso.isEmpty) return '';
  final dt = DateTime.tryParse(iso);
  if (dt == null) return iso;
  final d = dt.toLocal();
  return '${_monthAbbr[d.month - 1]} ${d.day}, ${d.year}';
}

/// Item name with its brand, without doubling the brand when the item
/// name already leads with it — "Ahold Ahold Garlic Salt" → "Ahold
/// Garlic Salt".
String brandedName(String? brand, String? name) {
  final n = (name ?? '').trim();
  final b = (brand ?? '').trim();
  if (n.isEmpty) return b;
  if (b.isEmpty || n.toLowerCase().startsWith(b.toLowerCase())) return n;
  return '$b $n';
}

/// Day-of-week index → short weekday (0 = Sunday, matching the web
/// client's DAY_NAMES).
const weekdayNames = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat'];

String weekdayName(int? day) =>
    (day != null && day >= 0 && day < 7) ? weekdayNames[day] : 'Day $day';

/// Grocery-line source enums → human copy.
String humanSource(String? source) {
  switch (source) {
    case 'manual':
      return 'Added manually';
    case 'mealplan':
      return 'From meal plan';
    case 'pantry':
      return 'From pantry';
    case 'recipe':
      return 'From recipe';
    default:
      return source ?? '';
  }
}
