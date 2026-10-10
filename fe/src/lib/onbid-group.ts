/**
 * 공매 목록 「같은 건물 묶기」 (2026-10-03 R4). 한 건물의 호실 20~30건이 한 쪽을 통째로 채우던 것을
 * 첫 물건 한 장 + 「같은 건물 N건 더」로. 주소에서 층·호·숫자 동을 지운 것이 건물 키다.
 */
export function buildingKey(address: string): string {
  return address
    .replace(/\s*(지하\s*)?(제\s*)?[A-Za-z]?\d+(-\d+)?\s*(층|호)(?=\s|$|,)/g, "")
    .replace(/\s*(제\s*)?\d+\s*동(?=\s|$|,)/g, "")
    .replace(/\s+/g, " ")
    .trim();
}

export type BuildingGroup<T> = { lead: T; rest: T[] };

/** 처음 나온 순서를 지켜 묶는다. 주소가 비면 묶지 않는다. */
export function groupByBuilding<T extends { address: string }>(items: T[]): BuildingGroup<T>[] {
  const out: BuildingGroup<T>[] = [];
  const at = new Map<string, number>();
  for (const it of items) {
    const k = it.address ? buildingKey(it.address) : "";
    const i = k ? at.get(k) : undefined;
    if (i == null) {
      if (k) at.set(k, out.length);
      out.push({ lead: it, rest: [] });
    } else {
      out[i].rest.push(it);
    }
  }
  return out;
}
