/** Format a computed quantity for display — at most 2 decimals, trailing zeros trimmed. */
export function fmtQty(n: number): string {
  return Number(n.toFixed(2)).toString();
}

/**
 * Format a date string for display — date components ("2024-01-01" or an ISO
 * timestamp's date part) parse as a local date so the rendered day doesn't
 * shift back in western timezones.
 */
export function fmtDate(value: string | null | undefined): string {
  if (!value) return "";
  const dm = /^(\d{4})-(\d{2})-(\d{2})/.exec(value);
  if (dm) {
    return new Date(
      Number(dm[1]),
      Number(dm[2]) - 1,
      Number(dm[3])
    ).toLocaleDateString();
  }
  const parsed = Date.parse(value);
  return Number.isNaN(parsed) ? value : new Date(parsed).toLocaleDateString();
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

/** "Brand{sep}Name" — or just Name when it already leads with the brand ("Ahold Ahold corn" → "Ahold corn"). */
export function brandedName(
  brand: string | null | undefined,
  name: string,
  sep = " ",
): string {
  const b = brand?.trim();
  if (!b || name.trim().toLowerCase().startsWith(b.toLowerCase())) return name;
  return `${b}${sep}${name}`;
}

/** " — Brand" suffix for a name that doesn't already mention it. */
export function brandSuffix(
  brand: string | null | undefined,
  name: string,
): string {
  const b = brand?.trim();
  if (!b || name.trim().toLowerCase().includes(b.toLowerCase())) return "";
  return ` — ${b}`;
}
