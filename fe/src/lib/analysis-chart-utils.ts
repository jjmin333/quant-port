/** 현재값 기준 역산 트렌드 (히스토리 API 연동 전 시각화용) */
function hashSeed(seed: string): number {
  let h = 0;
  for (let i = 0; i < seed.length; i++) h = (h * 31 + seed.charCodeAt(i)) | 0;
  return Math.abs(h);
}

export function buildTrendSeries(
  current: number,
  seed: string,
  points = 12,
): { label: string; value: number }[] {
  if (!Number.isFinite(current) || current <= 0) return [];

  const out: { label: string; value: number }[] = [];
  let v = current;
  let state = hashSeed(seed) || 1;
  // KST calendar year, not the browser's local one — matters right at the
  // year boundary when the viewer isn't in Asia/Seoul.
  const year = Number(
    new Intl.DateTimeFormat("en-US", { timeZone: "Asia/Seoul", year: "numeric" }).format(new Date()),
  );

  for (let i = points - 1; i >= 0; i--) {
    out.push({ label: String(year - i), value: Math.max(0.01, v) });
    state = (state * 1103515245 + 12345) & 0x7fffffff;
    const drift = 1 + ((state % 1000) / 1000 - 0.52) * 0.1;
    v = v / drift;
  }
  out[out.length - 1] = { label: String(year), value: current };
  return out;
}

export function percentileInRange(value: number, min: number, max: number): number {
  if (max <= min) return 50;
  return Math.min(100, Math.max(0, ((value - min) / (max - min)) * 100));
}

export function rangeLabel(value: number, median: number): { label: string; tone: "low" | "mid" | "high" } {
  const ratio = median > 0 ? value / median : 1;
  if (ratio < 0.85) return { label: "Low", tone: "low" };
  if (ratio > 1.15) return { label: "High", tone: "high" };
  return { label: "Mid", tone: "mid" };
}

export function medianOf(values: number[]): number {
  const sorted = values.filter((v) => Number.isFinite(v)).sort((a, b) => a - b);
  if (sorted.length === 0) return 0;
  const mid = Math.floor(sorted.length / 2);
  return sorted.length % 2 ? sorted[mid]! : (sorted[mid - 1]! + sorted[mid]!) / 2;
}
