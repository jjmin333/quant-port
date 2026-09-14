/**
 * 지수 상태 키워드 — 코드를 화면 문구로 옮긴다.
 *
 * BE(`be/internal/service/index_tags.go`)는 `high_52w`, `streak_up:4` 같은 **코드**만
 * 저장한다. 문구가 여기 있는 이유는 하나다 — 표현을 다듬자고 배치를 다시 돌리고
 * 하루치 데이터를 기다릴 이유가 없다.
 *
 * 이 칩들이 대신하는 것은 홈 스트립의 「AI 56」이었다. 그 숫자는 횡단면 랭킹이 아니라
 * 절대 지표로 만든 값이라 코스피 56과 나스닥 62를 비교할 수 없는데, 나란히 놓이면
 * 반드시 비교하게 된다. 여기 있는 건 전부 **셀 안의 스파크라인을 보면 그 자리에서
 * 확인되는 사실**이고, 그래서 설명이 필요 없다.
 */

/** 칩의 색 갈래. `up`/`down`은 상승·하락 색, `warn`은 주의, `neutral`은 무채색. */
export type IndexTagTone = "up" | "down" | "warn" | "neutral";

export type IndexTag = {
  code: string;
  label: string;
  tone: IndexTagTone;
  /** 마우스를 올렸을 때 나오는 근거 한 줄. */
  hint: string;
};


/*
  색은 **좋고 나쁨이 아니라 방향**을 뜻한다 — 오르면 빨강, 내리면 파랑, 그 밖은 회색.

  한때 원/달러·VIX·미 국채 10년만 무채색으로 뺐다. 「3일 연속↑」에 상승색을 칠하면
  좋은 소식처럼 읽힌다는 이유였는데, 그 걱정은 **이 앱에서 색이 이미 방향만 뜻한다**는
  사실과 어긋난다: 같은 셀 안의 등락률 숫자는 VIX가 −4.60%일 때 이미 파랗다
  (`trendTextClass`). 그래서 셋만 칩이 회색이면 오히려 **한 셀 안에서 색 규칙이 둘로
  갈리는** 꼴이었고, 실측 08-31에 VIX는 「3일 연속↓」과 「한 달 −15%」를 나란히 달고도
  둘 다 회색이라 무엇이 내렸다는 건지 눈에 들어오지 않았다.

  좋은 소식인지 나쁜 소식인지는 색이 아니라 지수 모달의 해설 문단이 답한다 — 거기서는
  방향이 아니라 국면을 말로 적는다.
*/

/** 숫자가 붙는 코드는 `코드:값` 꼴이다. */
function splitTagCode(code: string): [string, string] {
  const at = code.indexOf(":");
  return at < 0 ? [code, ""] : [code.slice(0, at), code.slice(at + 1)];
}

type TagSpec = {
  label: (arg: string) => string;
  tone: IndexTagTone;
  hint: string;
  /**
   * 이 키워드가 **오늘 하루에 대한 주장**인가.
   *
   * 아래 `toIndexTags`의 모순 필터가 이 값만 본다. 「3일 연속↑」은 오늘까지의 이야기라
   * 오늘 내리는 중이면 모순이지만, 「한 달 +8%」는 지난 20거래일에 대한 말이라 오늘
   * 내려도 여전히 참이다. 둘을 같이 걸렀더니 코스피·코스닥·비트코인의 칩이 하락일마다
   * 통째로 사라져, 없애려던 빈 칸이 그대로 돌아왔다(실측 08-31).
   */
  sameDay?: boolean;
  /**
   * 색이 **값에 따라 달라지는** 키워드를 위한 자리.
   *
   * 「한 달 +8%」와 「한 달 −8%」는 같은 코드(`month`)인데 색이 달라야 한다 — 부호를
   * 값에 넣은 이상(`month_up:0` 같은 자기모순을 없애려고 그렇게 했다) 고정 `tone`으로는
   * 표현할 수 없다. 이 함수가 있으면 `tone`보다 우선한다.
   */
  toneFor?: (arg: string) => IndexTagTone;
};

/** `+5` → up, `-2` → down, `+0`/`-0` → neutral(0%에 색을 칠할 이유가 없다). */
function signedTone(arg: string): IndexTagTone {
  const n = Number(arg);
  if (!Number.isFinite(n) || n === 0) return "neutral";
  return n > 0 ? "up" : "down";
}

const TAG_SPECS: Record<string, TagSpec> = {
  cross_golden: {
    label: () => "골든크로스",
    tone: "up",
    hint: "50일선이 200일선을 위로 지났습니다 — 오늘 일어난 변화입니다.",
    sameDay: true,
  },
  cross_dead: {
    label: () => "데드크로스",
    tone: "down",
    hint: "50일선이 200일선을 아래로 지났습니다 — 오늘 일어난 변화입니다.",
    sameDay: true,
  },
  high_52w: {
    label: () => "52주 최고",
    tone: "up",
    hint: "종가가 최근 1년 중 가장 높습니다.",
    sameDay: true,
  },
  low_52w: {
    label: () => "52주 최저",
    tone: "down",
    hint: "종가가 최근 1년 중 가장 낮습니다.",
    sameDay: true,
  },
  surge: {
    label: () => "급등",
    tone: "up",
    hint: "오늘 등락이 최근 60일 평소 진폭의 2배를 넘었습니다.",
    sameDay: true,
  },
  plunge: {
    label: () => "급락",
    tone: "down",
    hint: "오늘 등락이 최근 60일 평소 진폭의 2배를 넘었습니다.",
    sameDay: true,
  },
  sma20_recover: {
    label: () => "20일선 회복",
    tone: "up",
    hint: "어제까지 20일 이동평균 아래에 있다가 오늘 위로 올라섰습니다.",
    sameDay: true,
  },
  sma20_break: {
    label: () => "20일선 이탈",
    tone: "down",
    hint: "어제까지 20일 이동평균 위에 있다가 오늘 아래로 내려섰습니다.",
    sameDay: true,
  },
  streak_up: {
    label: (n) => `${n}일 연속↑`,
    tone: "up",
    hint: "종가가 며칠째 이어서 올랐는지입니다.",
    sameDay: true,
  },
  streak_down: {
    label: (n) => `${n}일 연속↓`,
    tone: "down",
    hint: "종가가 며칠째 이어서 내렸는지입니다.",
    sameDay: true,
  },
  month_up: {
    label: (n) => `한 달 +${n}%`,
    tone: "up",
    hint: "최근 20거래일 누적 등락입니다.",
  },
  month_down: {
    label: (n) => `한 달 −${n}%`,
    tone: "down",
    hint: "최근 20거래일 누적 등락입니다.",
  },
  overbought: {
    label: () => "과열",
    tone: "warn",
    hint: "RSI(14)가 70 이상입니다.",
  },
  oversold: {
    label: () => "과매도",
    tone: "warn",
    hint: "RSI(14)가 30 이하입니다.",
  },
  vol_expand: {
    label: () => "변동성 확대",
    tone: "warn",
    hint: "최근 20일 변동성이 100일 평균의 1.5배를 넘었습니다.",
  },
  range_bound: {
    label: () => "박스권",
    tone: "neutral",
    hint: "최근 20일 고저 폭이 평균 대비 3% 안입니다.",
  },
  /*
    아래 둘은 **다른 할 말이 없을 때 나오는 마지막 칩**이다(BE `index_tags.go` 10번).
    조용한 지수의 칸이 비어 있으면 "조용하다"가 아니라 **아직 안 불러왔다**로 읽히는데,
    실측 08-31에는 열세 칸 중 여덟이 그랬다.

    색은 **값의 부호를 따른다**(`toneFor`). 한때 무조건 회색으로 뒀는데, 그건 모순
    필터가 이 칩까지 지우던 시절의 우회였다 — 지금은 필터가 당일 주장(`sameDay`)에만
    걸리므로 색을 칠해도 사라지지 않는다. 옆 칸의 등락률 숫자와 같은 색 계열을 쓰므로
    한 셀 안에서 따로 놀지 않는다. 값은 부호를 달고 온다(`month:+5`, `week:-2`).
  */
  month: {
    label: (n) => `한 달 ${signedPercent(n)}`,
    tone: "neutral",
    toneFor: signedTone,
    hint: "최근 20거래일 누적 등락입니다.",
  },
  week: {
    label: (n) => `한 주 ${signedPercent(n)}`,
    tone: "neutral",
    toneFor: signedTone,
    hint: "최근 5거래일 누적 등락입니다. 20거래일이 아직 쌓이지 않은 지수입니다.",
  },
};

/** `+5` → `+5%`, `-2` → `−2%`. 빼기표는 하이픈이 아니라 −(U+2212)를 쓴다. */
function signedPercent(arg: string): string {
  return `${arg.replace("-", "−")}%`;
}

/**
 * 코드 배열을 칩으로 옮긴다.
 *
 * 모르는 코드는 **버린다**. BE가 새 키워드를 먼저 배포해도 화면에 `foo_bar`가
 * 그대로 뜨지 않게 한다 — 배포 순서에 화면이 끌려다니면 안 된다.
 *
 * `changeRate`를 받으면 **오늘 움직임과 어긋나는 키워드를 지운다**. 키워드는 마지막
 * 「마감된」 세션까지의 사실이고 값·등락률은 지금 이 순간이라, 장중에는 둘이 다른
 * 날을 말할 수 있다. 실측에서 코스피가 「3일 연속↑」 옆에 −1.43%를 달고 있었다 —
 * 어제까지 사흘 올랐고 오늘 내리는 중이니 둘 다 참이지만, 한 줄에 나란히 놓이면
 * 읽는 사람에게는 그냥 모순이다. 어제 끝난 이야기는 접는다.
 *
 * **다만 오늘에 대한 주장만 접는다**(`spec.sameDay`). 「한 달 +8%」는 지난 20거래일에
 * 대한 말이라 오늘 내려도 여전히 참인데, 이것까지 걸렀더니 하락일마다 코스피·코스닥·
 * 비트코인의 칩이 통째로 사라졌다 — 없애려던 빈 칸이 그대로 돌아온 셈이다(실측 08-31).
 *
 * 종가로 도는 지수(FRED 계열)는 등락률 자체가 그 세션의 것이라 이 규칙에 걸리지
 * 않는다 — 방향이 어긋날 수가 없다.
 */
export function toIndexTags(
  codes: string[] | undefined,
  changeRate?: number,
): IndexTag[] {
  if (!codes?.length) return [];

  const out: IndexTag[] = [];
  for (const code of codes) {
    const [name, arg] = splitTagCode(code);
    const spec = TAG_SPECS[name];
    if (!spec) continue;
    const tone = spec.toneFor ? spec.toneFor(arg) : spec.tone;
    if (spec.sameDay && changeRate != null && Number.isFinite(changeRate)) {
      if (tone === "up" && changeRate < 0) continue;
      if (tone === "down" && changeRate > 0) continue;
    }
    out.push({ code, label: spec.label(arg), tone, hint: spec.hint });
  }
  return out;
}

/**
 * 칩 색. 등락 색은 `--profit`/`--loss` 토큰을 그대로 쓴다 — 등락률 숫자와 같은
 * 계열이어야 한 셀 안에서 따로 놀지 않는다. 테두리 농도(`/40`)는 점수 배지가 쓰던
 * 값을 그대로 따랐다(`SCORE_TONE_BORDER`).
 */
export const INDEX_TAG_TONE_CLASS: Record<IndexTagTone, string> = {
  up: "border-[var(--profit)]/40 text-[var(--profit)]",
  down: "border-[var(--loss)]/40 text-[var(--loss)]",
  // 앱에 별도의 「주의」 토큰이 없다. `--score-mid`가 이미 그 자리의 주황이라
  // 새 색을 만들지 않고 빌려 쓴다.
  warn: "border-[var(--score-mid)]/40 text-[var(--score-mid)]",
  neutral: "border-[var(--border)] text-[var(--text-muted)]",
};
