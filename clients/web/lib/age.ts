// isOfDrinkingAge gates alcohol suggestion entry points at 21+. It's a
// display gate only — the resolver re-checks server-side from the stored
// birthdate, so a stale client value can't bypass it.
export function isOfDrinkingAge(
  birthdate: string | null | undefined,
  now = new Date()
): boolean {
  if (!birthdate) return false;
  const [y, m, d] = birthdate.split("-").map(Number);
  if (!y || !m || !d) return false;
  let age = now.getFullYear() - y;
  if (now.getMonth() + 1 < m || (now.getMonth() + 1 === m && now.getDate() < d)) {
    age--;
  }
  return age >= 21;
}
