"use client";

import {
  SCORE_AXIS_LABELS,
  SCORE_AXIS_ORDER,
  scoreTone,
  type ScoreBreakdown,
} from "@/lib/stock-score-api";
import { cn } from "@/lib/utils";

/**
 * 5축 점수 레이더.
 *
 * 08-11 사용자 결정: 우측 레일에서는 가로 막대를 이걸로 **교체**한다. 값을 정확히 읽는
 * 용도는 AI 분석 탭의 막대가 이미 맡고 있고, 레일에서는 "이 종목이 어느 쪽으로 치우쳤나"
 * 라는 모양을 보는 게 목적이다.
 *
 * recharts를 쓰지 않고 SVG로 직접 그린다 — 5점짜리 폴리곤 하나에 차트 라이브러리를
 * 끌어오면 번들만 커지고, 축 라벨 위치를 이만큼 세밀하게 잡기도 어렵다.
 *
 * **값이 없는 축은 0으로 그리지 않는다.** 커버리지가 낮은 종목(재무 미수집 등)에서
 * 0으로 찍으면 "가치 점수가 바닥"이라는 없는 사실을 만들어낸다. 빠진 축은 중심에서
 * 끌어오는 대신 그 축을 뺀 다각형을 그리고, 라벨에 `—`를 붙인다.
 */

type ScoreRadarChartProps = {
  breakdown: ScoreBreakdown;
  /** 전체 점수 — 폴리곤 색을 정하는 데 쓴다 */
  score: number;
  className?: string;
};

const SIZE = 200;
const CENTER = SIZE / 2;
const RADIUS = 62;
const LABEL_RADIUS = RADIUS + 22;

/** 12시 방향에서 시작해 시계방향. */
function pointAt(index: number, count: number, radius: number) {
  const angle = (Math.PI * 2 * index) / count - Math.PI / 2;
  return {
    x: CENTER + radius * Math.cos(angle),
    y: CENTER + radius * Math.sin(angle),
  };
}

function polygon(radiusFor: (i: number) => number, count: number): string {
  return Array.from({ length: count }, (_, i) => {
    const p = pointAt(i, count, radiusFor(i));
    return `${p.x.toFixed(1)},${p.y.toFixed(1)}`;
  }).join(" ");
}

export function ScoreRadarChart({ breakdown, score, className }: ScoreRadarChartProps) {
  const axes = SCORE_AXIS_ORDER.map((id) => ({
    id,
    label: SCORE_AXIS_LABELS[id],
    value: breakdown[id],
  }));
  const count = axes.length;
  const tone = scoreTone(score);
  const stroke =
    tone === "high"
      ? "var(--score-high)"
      : tone === "mid"
        ? "var(--score-mid)"
        : "var(--score-low)";

  // 값이 있는 축만으로 폴리곤을 만든다. 두 개 미만이면 모양이 성립하지 않는다.
  const present = axes.filter((a) => typeof a.value === "number");
  const canDraw = present.length >= 3;

  return (
    <div className={cn("min-w-0", className)}>
      <svg
        viewBox={`0 0 ${SIZE} ${SIZE}`}
        className="mx-auto block w-full max-w-[210px]"
        role="img"
        aria-label={`5축 점수 레이더 — ${axes
          .map((a) => `${a.label} ${a.value ?? "없음"}`)
          .join(", ")}`}
      >
        {/* 격자 — 25 / 50 / 75 / 100 */}
        {[0.25, 0.5, 0.75, 1].map((r) => (
          <polygon
            key={r}
            points={polygon(() => RADIUS * r, count)}
            fill="none"
            stroke="var(--border)"
            strokeWidth={r === 1 ? 1 : 0.75}
          />
        ))}

        {/* 축선 */}
        {axes.map((axis, i) => {
          const p = pointAt(i, count, RADIUS);
          return (
            <line
              key={axis.id}
              x1={CENTER}
              y1={CENTER}
              x2={p.x}
              y2={p.y}
              stroke="var(--border)"
              strokeWidth={0.75}
            />
          );
        })}

        {canDraw && (
          <polygon
            points={axes
              .map((axis, i) => {
                if (typeof axis.value !== "number") return null;
                const clamped = Math.min(100, Math.max(0, axis.value));
                const p = pointAt(i, count, (RADIUS * clamped) / 100);
                return `${p.x.toFixed(1)},${p.y.toFixed(1)}`;
              })
              .filter(Boolean)
              .join(" ")}
            fill={stroke}
            fillOpacity={0.18}
            stroke={stroke}
            strokeWidth={2}
            strokeLinejoin="round"
          />
        )}

        {/* 값이 있는 축의 꼭짓점 */}
        {axes.map((axis, i) => {
          if (typeof axis.value !== "number") return null;
          const clamped = Math.min(100, Math.max(0, axis.value));
          const p = pointAt(i, count, (RADIUS * clamped) / 100);
          return <circle key={axis.id} cx={p.x} cy={p.y} r={2.5} fill={stroke} />;
        })}

        {/* 라벨 */}
        {axes.map((axis, i) => {
          const p = pointAt(i, count, LABEL_RADIUS);
          // 좌우 끝 축은 바깥으로 밀어야 글자가 도형에 겹치지 않는다.
          const anchor = p.x < CENTER - 6 ? "end" : p.x > CENTER + 6 ? "start" : "middle";
          return (
            <text
              key={axis.id}
              x={p.x}
              y={p.y + 3}
              textAnchor={anchor}
              className="fill-muted-foreground text-micro"
            >
              {axis.label}{" "}
              <tspan className="fill-foreground font-semibold">
                {typeof axis.value === "number" ? Math.round(axis.value) : "—"}
              </tspan>
            </text>
          );
        })}
      </svg>

      {!canDraw && (
        <p className="mt-1 text-center text-cell text-muted-foreground">
          축 데이터가 부족해 모양을 그리지 못했습니다
        </p>
      )}
    </div>
  );
}
