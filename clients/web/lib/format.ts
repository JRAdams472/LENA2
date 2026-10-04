/** Format a computed quantity for display — at most 2 decimals, trailing zeros trimmed. */
export function fmtQty(n: number): string {
  return Number(n.toFixed(2)).toString();
}

const SIZE_RE =
  /\b(\d+(?:\.\d+)?\s?(?:pk|ct|count|pack|oz|fl\.?\s?oz|lb|g|kg|ml|l))\b/i;

/** Extract a pack-size token from an item name, falling back to the unit. */
export function sizeBadge(name: string, unit: string): string | null {
  const m = SIZE_RE.exec(name);
  if (m) return m[1];
  return unit && unit !== "each" ? unit : null;
}

const EDGE_CHARS = new Set([" ", "\t", "\n", "\r", "-", "–", "—", ","]);

// Manual edge strip — equivalent to /^[<set>]+|[<set>]+$/g without a
// regex the scanner flags as super-linear.
function stripEdges(s: string): string {
  let a = 0;
  let b = s.length;
  while (a < b && EDGE_CHARS.has(s[a])) a++;
  while (b > a && EDGE_CHARS.has(s[b - 1])) b--;
  return s.slice(a, b);
}

/** Strip an embedded size token and a duplicated brand prefix from an item name. */
export function stripSize(
  name: string,
  size: string | null,
  brand?: string | null,
): string {
  let n = size ? name.replace(size, "") : name;
  const b = brand?.trim();
  if (b && n.trim().toLowerCase().startsWith(b.toLowerCase())) {
    n = n.trim().slice(b.length);
  }
  return stripEdges(n.replace(/\s{2,}/g, " ")).trim();
}
