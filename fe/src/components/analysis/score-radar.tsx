import { SCORE_AXIS_LABELS, SCORE_AXIS_ORDER, type ScoreBreakdown } from "@/lib/stock-score-api";

/**
 * 다섯 축 레이더 (2026-10-10, 종목 화면 A안 · BUSINESS_REVIEW §2.2).
 *
 * 공유 카드(`stocks/[symbol]/opengraph-image.tsx`)와 **같은 모양**이다 — 카톡에서 본 그림과 화면의
 * 그림이 같아야 한다. 축 순서도 같다(추세에서 시계 방향). 값이 없는 축은 중심 가까이(4%)에 둔다 —
 * 0으로 그리면 「최악」으로 읽힌다.
 */
const SIZE = 180;
const C = SIZE / 2;
const R = 58;

function point(i: number, ratio: number): [number, number] {
  const angle = -Math.PI / 2 + (i * 2 * Math.PI) / SCORE_AXIS_ORDER.length;
  return [C + Math.cos(angle) * R * ratio, C + Math.sin(angle) * R * ratio];
}

function polygon(ratios: number[]): string {
  return ratios.map((r, i) => point(i, r).map((v) => v.toFixed(1)).join(",")).join(" ");
}

/** `labels={false}` — 작은 카드(홈 대표 종목)용. 축 이름·값을 빼고 도형만. */
export function ScoreRadar({
  breakdown,
  className,
  labels = true,
}: {
  breakdown: ScoreBreakdown | undefined;
  className?: string;
  labels?: boolean;
}) {
  const values = SCORE_AXIS_ORDER.map((axis) => breakdown?.[axis]);
  const ratios = values.map((v) => (typeof v === "number" ? Math.max(0.04, Math.min(1, v / 100)) : 0.04));
  const label = SCORE_AXIS_ORDER.map((axis, i) => {
    const v = values[i];
    return `${SCORE_AXIS_LABELS[axis]} ${typeof v === "number" ? Math.round(v) : "없음"}`;
  }).join(", ");

  return (
    // 축 이름(「심리·수급」)이 원 밖으로 나가므로 좌우·위에 여백을 둔다 — 375px에서 잘렸다.
    <svg viewBox={labels ? `-24 -6 ${SIZE + 48} ${SIZE + 6}` : `${C - R - 4} ${C - R - 4} ${2 * R + 8} ${2 * R + 8}`} role="img" aria-label={`다섯 축 점수: ${label}`} className={className}>
      {[0.5, 1].map((r) => (
        <polygon
          key={r}
          points={polygon(SCORE_AXIS_ORDER.map(() => r))}
          fill="none"
          style={{ stroke: "var(--border)" }}
          strokeWidth={1}
        />
      ))}
      {SCORE_AXIS_ORDER.map((axis, i) => {
        const [x, y] = point(i, 1);
        return <line key={axis} x1={C} y1={C} x2={x} y2={y} style={{ stroke: "var(--border)" }} strokeWidth={1} />;
      })}
      <polygon
        points={polygon(ratios)}
        style={{ fill: "color-mix(in oklab, var(--brand) 22%, transparent)", stroke: "var(--brand)" }}
        strokeWidth={2}
      />
      {labels && SCORE_AXIS_ORDER.map((axis, i) => {
        const [x, y] = point(i, 1.42);
        const v = values[i];
        return (
          <text key={axis} x={x} y={y} textAnchor="middle" fontSize={11} style={{ fill: "var(--text-muted)" }}>
            <tspan x={x} dy="-0.2em">
              {SCORE_AXIS_LABELS[axis]}
            </tspan>
            <tspan x={x} dy="1.15em" fontWeight={600} style={{ fill: "var(--text-primary)" }}>
              {typeof v === "number" ? Math.round(v) : "–"}
            </tspan>
          </text>
        );
      })}
    </svg>
  );
}
