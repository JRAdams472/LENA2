/** Format a computed quantity for display — at most 2 decimals, trailing zeros trimmed. */
export function fmtQty(n: number): string {
  return Number(n.toFixed(2)).toString();
}
