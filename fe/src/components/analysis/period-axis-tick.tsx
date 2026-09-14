"use client";

import type { PeriodTick } from "@/lib/financial-period-label";

/**
 * recharts가 눈금에 넘기는 좌표는 타입상 `string | number`다(SVG 속성 그대로).
 * 실제로는 늘 숫자지만 좁혀서 받으면 `XAxis`의 tick 시그니처와 안 맞는다.
 */
type TickProps = {
  x?: number | string;
  y?: number | string;
  payload?: { value?: string | number };
};

/**
 * 분기 축의 두 줄 눈금 — 윗줄 "3월", 아랫줄 "26년"(연도가 바뀔 때만).
 *
 * recharts는 `tick`에 넘긴 요소를 눈금마다 복제하며 x·y·payload를 주입한다.
 * 여기서는 축이 들고 있는 값(`payload.value` = 고유 라벨)으로 표기를 되찾으므로,
 * 축의 카테고리는 고유하게 두면서 화면 글자만 줄일 수 있다.
 *
 * `interval={0}`과 함께 쓴다 — 라벨이 짧아 12분기가 모두 들어가고, 자동 솎아내기에
 * 맡기면 연도를 인 눈금이 지워져 그 해가 통째로 사라진다.
 */
export function PeriodAxisTick({
  index,
  ...props
}: TickProps & { index: Record<string, PeriodTick> }) {
  const x = Number(props.x ?? 0);
  const y = Number(props.y ?? 0);
  const { payload } = props;
  const key = String(payload?.value ?? "");
  const tick = index[key];
  const month = tick?.month ?? key;
  const year = tick?.year;

  return (
    <text x={x} y={y} textAnchor="middle" fill="var(--text-muted)" fontSize={11}>
      <tspan x={x} dy={12}>
        {month}
      </tspan>
      {year && (
        <tspan x={x} dy={11}>
          {year}
        </tspan>
      )}
    </text>
  );
}

/** `<XAxis tick={periodTickRenderer(index)} …>` 형태로 쓴다. */
export function periodTickRenderer(index: Record<string, PeriodTick>) {
  const render = (props: TickProps) => <PeriodAxisTick {...props} index={index} />;
  return render;
}

/** 두 줄 눈금이 잘리지 않는 축 높이. */
export const PERIOD_AXIS_HEIGHT = 32;

/**
 * 12분기(3년)까지는 눈금을 전부 그린다 — 두 줄로 줄인 라벨이 모바일 카드 폭에도
 * 들어간다. 상세 모달의 5년(20분기)은 그 폭을 넘기므로 자동 솎아내기에 맡긴다:
 * 그때 연도를 인 눈금이 지워질 수 있지만, 모달에는 기간을 그대로 적는 표가 아래에
 * 함께 있다.
 */
export function periodTickInterval(count: number): 0 | "preserveStartEnd" {
  return count <= 12 ? 0 : "preserveStartEnd";
}
