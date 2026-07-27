/** Korean market convention: up = red, down = blue */
export function trendTextClass(value: number, muted = false): string {
  if (value >= 0) {
    return muted ? "text-[var(--profit)]/80" : "text-[var(--profit)]";
  }
  return muted ? "text-[var(--loss)]/80" : "text-[var(--loss)]";
}

export function trendBadgeClass(value: number): string {
  if (value >= 0) {
    return "bg-red-500/10 text-[var(--profit)]";
  }
  return "bg-blue-500/10 text-[var(--loss)]";
}
