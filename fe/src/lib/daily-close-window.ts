/**
 * 일별 종가 차트의 **기간 버튼** — 지수 모달과 종목 모달이 함께 쓴다.
 *
 * # 왜 거래일 수로 말하는가
 *
 * 「1개월」이 아니라 「20일」이다. 실측(2026-08-16): 코스피에 `limit=66`을 줘도 31행만
 * 온다. 일별 종가는 이제 막 쌓이는 중이고 종목마다 깊이가 다르다. 캘린더 기간으로
 * 이름을 붙이면 「3개월」과 「1년」이 **똑같은 차트**를 그리게 되고, 화면이 없는 기간을
 * 있다고 말한다.
 *
 * # 왜 함수로 뺐는가 (2026-09-05 사용자 요청 7-1)
 *
 * 지수 모달에만 있던 이 규칙을 종목 모달도 쓰게 됐다. 복사해 두면 한쪽만 고쳐지고,
 * 그 차이는 「지수는 전체가 뜨는데 종목은 안 뜬다」로 나타난다.
 *
 * # 「전체」의 뜻을 바로잡았다
 *
 * 예전 구현은 `[20, 60, 250]`을 훑으며 **실제 봉 수가 겹치는 단계만 걸러** 냈는데,
 * 라벨은 여전히 그 단계의 이름을 썼다. 그래서 31행짜리 지수가 「20일 / 60일」로
 * 떴다 — 60일 버튼이 그리는 것은 31봉인데도. 같은 파일의 주석은 「20일과 전체 둘만
 * 뜬다」고 적어 두었으니, 코드가 아니라 주석 쪽이 맞았다.
 *
 * 지금은 **가진 것을 다 그리는 단계는 「전체」로 부른다**. 「60일」은 정말 60봉이
 * 있을 때만 그렇게 적힌다.
 */

/** 한 번에 받아 두는 최대 봉 수. 토글은 이 안에서 자른다 — 누를 때마다 요청하지 않는다. */
export const DAILY_CLOSE_MAX_BARS = 250;

const BAR_STEPS = [20, 60, DAILY_CLOSE_MAX_BARS] as const;

/** 차트를 그릴 수 있는 최소 봉 수. 점 하나로는 선이 되지 않는다. */
const MIN_BARS = 2;

export type BarStep = {
  /** 자를 봉 수(`rows.slice(-step)`). */
  step: number;
  /** 버튼에 적을 말. */
  label: string;
};

/**
 * `rowCount`개의 봉으로 실제로 **서로 다른 차트**를 그리는 단계들.
 *
 * 하나뿐이면 부르는 쪽이 토글 자체를 그리지 않는다 — 누를 곳이 하나인 토글은
 * 자리만 차지한다.
 */
export function availableBarSteps(rowCount: number): BarStep[] {
  const out: BarStep[] = [];
  const seen = new Set<number>();
  for (const step of BAR_STEPS) {
    const effective = Math.min(step, rowCount);
    if (effective < MIN_BARS || seen.has(effective)) continue;
    seen.add(effective);
    // 가진 것을 다 그리는 단계는 「전체」다 — 「60일」이라 적어 놓고 31봉을 그리지 않는다.
    out.push({ step, label: effective < step || step >= DAILY_CLOSE_MAX_BARS ? "전체" : `${step}일` });
  }
  return out;
}

/**
 * 고른 단계가 아직 유효한가 — 아니면 남은 것 중 **가장 긴 것**.
 *
 * 종목·지수를 바꾸면 이력 깊이가 달라져 고른 단계가 사라질 수 있다. 그때 아무것도
 * 고르지 않은 상태로 두면 차트가 빈 채로 남는다.
 */
export function resolveBarStep(steps: BarStep[], picked: number): number {
  if (steps.some((s) => s.step === picked)) return picked;
  return steps[steps.length - 1]?.step ?? DAILY_CLOSE_MAX_BARS;
}
