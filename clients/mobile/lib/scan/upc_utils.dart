/// Normalizes a scanned barcode value into a UPC lookup code.
///
/// - 12 digits → UPC-12 (returned as-is)
/// - 13 digits → left-pad with 0 → GTIN-14
/// - 14 digits → GTIN-14 (returned as-is)
/// - anything else → null
String? normalizeUpc(String raw) {
  final digits = raw.replaceAll(RegExp(r'[^0-9]'), '');
  if (digits.length == 12) return digits;
  if (digits.length == 13) return '0$digits';
  if (digits.length == 14) return digits;
  return null;
}
