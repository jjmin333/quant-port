/*
정사각형에 가까운 트리맵 배치(Bruls·Huizing·van Wijk의 squarified). 시장 히트맵(티켓 QP-10)이 쓴다.
값이 큰 것부터 짧은 변을 따라 한 줄씩 채우고, 줄의 가장 나쁜 가로세로비가 나빠지면 줄을 닫는다.
*/

export type Rect = { x: number; y: number; w: number; h: number };

/** values(양수, 내림차순 권장)를 rect 안에 배치한 사각형 — 입력 순서 그대로 돌려준다. */
export function squarify(values: number[], rect: Rect): Rect[] {
  const out: Rect[] = new Array(values.length);
  const total = values.reduce((a, b) => a + b, 0);
  if (total <= 0 || rect.w <= 0 || rect.h <= 0) return values.map(() => ({ ...rect, w: 0, h: 0 }));
  const scale = (rect.w * rect.h) / total;
  const order = values.map((v, i) => ({ i, a: v * scale })).sort((p, q) => q.a - p.a);

  let { x, y, w, h } = rect;
  let row: { i: number; a: number }[] = [];
  const worst = (r: { a: number }[], side: number) => {
    const sum = r.reduce((s, e) => s + e.a, 0);
    const max = Math.max(...r.map((e) => e.a));
    const min = Math.min(...r.map((e) => e.a));
    return Math.max((side * side * max) / (sum * sum), (sum * sum) / (side * side * min));
  };
  const layRow = (r: { i: number; a: number }[]) => {
    const sum = r.reduce((s, e) => s + e.a, 0);
    if (w >= h) {
      // 세로 한 줄(왼쪽 기둥)
      const cw = sum / h;
      let cy = y;
      for (const e of r) {
        const eh = e.a / cw;
        out[e.i] = { x, y: cy, w: cw, h: eh };
        cy += eh;
      }
      x += cw;
      w -= cw;
    } else {
      const ch = sum / w;
      let cx = x;
      for (const e of r) {
        const ew = e.a / ch;
        out[e.i] = { x: cx, y, w: ew, h: ch };
        cx += ew;
      }
      y += ch;
      h -= ch;
    }
  };

  for (const e of order) {
    if (e.a <= 0) {
      out[e.i] = { x, y, w: 0, h: 0 };
      continue;
    }
    const side = Math.min(w, h);
    if (row.length === 0 || worst([...row, e], side) <= worst(row, side)) {
      row.push(e);
    } else {
      layRow(row);
      row = [e];
    }
  }
  if (row.length) layRow(row);
  return out;
}
