import type { IndexPoint } from "@/lib/wealth-api";

/**
 * 두 금리 계열의 차(%p)를 달마다 — 금리 화면 「장단기 차 · 예대금리차」(2026-10-03 F4).
 * 국고채는 월말 날짜, 예금은행 평균은 그달 1일이라 날짜가 아니라 「연-월」로 맞춘다. 한쪽만 있는 달은 뺀다.
 */
export function spreadByMonth(a: IndexPoint[], b: IndexPoint[]): IndexPoint[] {
  const bm = new Map(b.map((p) => [p.date.slice(0, 7), p.value]));
  const out: IndexPoint[] = [];
  const seen = new Set<string>();
  for (const p of [...a].sort((x, y) => x.date.localeCompare(y.date))) {
    const m = p.date.slice(0, 7);
    const v = bm.get(m);
    if (v == null) continue;
    const row = { date: `${m}-01`, value: Math.round((p.value - v) * 1000) / 1000 };
    if (seen.has(m)) out[out.length - 1] = row; // 같은 달이 둘이면 늦은 값
    else out.push(row);
    seen.add(m);
  }
  return out;
}
