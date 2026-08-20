export type EarningsCalendarKind = "upcoming" | "filed";

/**
 * 발표일의 세 상태. **화면에서 절대 같은 모양으로 그리지 않는다.**
 *
 * - `confirmed` 이미 발표됐다
 * - `scheduled` 소스가 준 예정일 (해외 Finnhub만)
 * - `estimated` 작년 동기 발표일로 **우리가 역산한** 추정일 (국내만)
 *
 * 국내에는 실적발표 예정일을 주는 소스가 존재하지 않는다 — DART에 예정 API가 없고
 * FnGuide 캘린더도 미래 달은 빈 배열이다. 그래서 국내 미래 일정은 전부 추정이며,
 * 이것을 회사가 공시한 날짜처럼 보이게 만드는 것이 이 기능의 유일한 거짓말이다.
 */
export type EarningsDateStatus = "confirmed" | "scheduled" | "estimated";

/** 발표 시각대. 해외만 — 국내 소스는 시각을 주지 않는다. */
export type EarningsSession = "bmo" | "amc" | "dmh";

export type EarningsSurpriseMetric = "opIncome" | "eps" | "revenue";

/**
 * 컨센서스 대비 실적.
 *
 * `pct`가 없을 수 있다 — 기준치(estimate)가 0 이하면 퍼센트가 무의미해서
 * 서버가 `label`("흑자전환"·"적자전환"·"적자지속")로 대신 답한다. 그때 숫자 칸을
 * `—`로 비우면 데이터가 없는 것처럼 보이므로 **라벨을 그 자리에 넣는다.**
 */
export type EarningsSurprise = {
  metric: EarningsSurpriseMetric;
  estimate: number;
  actual: number;
  pct?: number;
  label?: string;
};

/** 기대치 / 발표치 / (국내) 전분기 대비. */
export type EarningsMetricPair = {
  estimate?: number;
  actual?: number;
  /** FnGuide가 주는 전분기 대비 증감률(%). YoY가 아니라 QoQ다. */
  qoq?: number;
};

/**
 * 발표 후 첫 세션의 주가 반응.
 *
 * **없을 수 있고, 없음은 0%가 아니다.** 인접 거래일 종가가 없으면 서버가 값을
 * 내지 않는다(최근 세션의 종가 커버리지가 얇다) — 그때 0%로 그리면 「반응이
 * 없었다」는 거짓말이 되므로 화면은 「집계 중」으로 말한다.
 *
 * `basis`가 `sameDay`면 장전 발표라 **발표 당일** 종가가 첫 반응이고,
 * `nextDay`면 장후·국내라 **다음 거래일**이 첫 반응이다. 이 구분을 안 하면
 * 반응을 하루 잘못 읽는다.
 */
export type EarningsReaction = {
  baseClose: number;
  baseDate: string;
  close: number;
  date: string;
  pct: number;
  basis: "sameDay" | "nextDay";
};

/** 이 회사가 컨센서스를 습관적으로 넘기는지. 한 번은 우연, 5분기 중 4번은 습관. */
export type EarningsHitHistory = {
  quarters: number;
  beats: number;
  misses: number;
  avgPct?: number;
  medianPct?: number;
};

/** 상단 스코어보드 — 이 구간 시장 전체의 온도. */
export type EarningsStats = {
  total: number;
  confirmed: number;
  scheduled: number;
  estimated: number;
  /** 서프라이즈를 계산할 수 있었던 건수. */
  compared: number;
  beats: number;
  misses: number;
  beatRate?: number;
  /**
   * 평균과 중위를 **둘 다** 쓴다. 실측에서 하루 평균이 −0.19%인데 중위는
   * +14.5%였다 — 이마트 −193% 한 건이 평균을 끌어내린 것이다. 하나만 보여주면
   * 그날 시장을 반대로 읽는다.
   */
  avgPct?: number;
  medianPct?: number;
  best?: EarningsStatsPick;
  worst?: EarningsStatsPick;
};

export type EarningsStatsPick = { symbol: string; name: string; pct: number };

/** 캘린더 한 줄 = 한 종목의 한 회계기간 발표. */
export type EarningsEvent = {
  symbol: string;
  name?: string;
  market: "domestic" | "us";
  date: string;
  dateStatus: EarningsDateStatus;
  session?: EarningsSession;
  /** 정정 공시가 있었던 날. 날짜는 원 발표일, 수치는 정정본이라는 뜻. */
  revisedAt?: string;
  fiscalYear: number;
  /** "1Q".."4Q" | "FY" */
  fiscalPeriod: string;
  periodEnd?: string;
  /** "연결" | "개별" (국내) */
  basis?: string;
  currency: string;
  source: string;
  revenue: EarningsMetricPair;
  opIncome: EarningsMetricPair;
  netIncome: EarningsMetricPair;
  eps: EarningsMetricPair;
  surprise?: EarningsSurprise;
  /**
   * 매출 서프라이즈 — 헤드라인과 **별개 열**이다. 이익이 기대를 넘었는데 매출이
   * 밑돌면 비용 절감이지 성장이 아니라는 뜻이고, 둘을 나란히 두어야 그게 보인다.
   */
  revenueSurprise?: EarningsSurprise;
  reaction?: EarningsReaction;
  /**
   * 발행주식수 × 최근 종가. 표에서 「무엇이 큰 발표인가」로 줄을 세우는 기준이다.
   * 실시간 시세가 아니라 **규모 구간**으로 쓴다 — 현재가 열과 몇 %는 다를 수 있고,
   * 정렬 목적에는 그 차이가 무의미하다.
   */
  marketCap?: number;
  /**
   * 같은 (종목·회계기간)의 **다른 기준** 발표(주로 개별).
   *
   * FnGuide가 연결·개별을 별 행으로 주고 컨센서스가 어긋나 붙어, 한 회사가 같은
   * 날 +54.6%(연결)와 −60.8%(개별)를 동시에 말한다(실측 338건). 서버가 연결을
   * 대표로 세우고 나머지를 여기 접어 준다 — 지우지 않는 이유는 지주회사처럼
   * 개별이 의미 있는 경우가 있기 때문이다.
   */
  alternates?: EarningsEvent[];
  /** 추정 행에만: "작년 2Q는 8/3 발표". 근거 없는 추정은 그냥 오답이다. */
  estimateBasis?: string;
};

/** 월 히트맵 한 칸. 상세 행을 세지 않고 서버가 집계해 준다. */
export type EarningsDaySummary = {
  date: string;
  confirmed: number;
  scheduled: number;
  estimated: number;
  /** 그날 컨센서스와 견줄 수 있었던 발표와 그중 상회한 수 — 달력 칸 색의 재료. */
  compared: number;
  beats: number;
};

export type EarningsCalendarEvent = {
  date: string;
  label: string;
  source: "finnhub" | "dart" | "fnguide";
  kind: EarningsCalendarKind;
  dateStatus?: EarningsDateStatus;
  session?: EarningsSession;
  surprise?: EarningsSurprise;
  estimateBasis?: string;
};

/**
 * 정기보고서(DART/EDGAR) 한 기간.
 *
 * ⚠️ **이 수치는 `EarningsEvent`의 발표 잠정치와 같은 기준이 아니다.** DART
 * 정기보고서는 1분기를 뺀 나머지가 누적이다(반기보고서 = 상반기 합계). 발표
 * 잠정치는 분기 단독이다. 그래서 두 표를 한 줄로 합치지 않는다 — 합치면
 * 「2Q 컨센서스(분기)」 옆에 「2Q 실적(반기 누적)」이 놓여 서프라이즈가 두 배가 된다.
 */
export type EarningsQuarter = {
  fiscalYear: number;
  period: string;
  reportCode?: string;
  periodEnd?: string;
  revenue?: number;
  operatingIncome?: number;
  netIncome?: number;
  epsBasic?: number;
};

export type EarningsConsensus = {
  available: boolean;
  period?: string;
  surprise?: EarningsSurprise;
  /** 없을 때만 이유가 온다. */
  message?: string;
};

export type EarningsTabData = {
  symbol: string;
  assetType: string;
  calendar: EarningsCalendarEvent[];
  /** 발표 잠정치 + 서프라이즈 (분기 단독 기준). */
  announcements: EarningsEvent[];
  quarterly: EarningsQuarter[];
  consensus: EarningsConsensus;
  hitHistory?: EarningsHitHistory;
};

export type EarningsMarketFilter = "all" | "domestic" | "us";

/**
 * 화면 하나가 맡는 시장. 사이드바가 국내·해외 두 항목으로 갈렸으므로 화면 안에는
 * 시장 토글이 없다 — 지표(연결/개별 · 장전/장후)와 데이터 성질이 달라 한 목록에
 * 섞으면 한쪽이 항상 이상해 보인다.
 */
export type EarningsMarket = "domestic" | "us";

/** 본문 보기 단위. 실적은 주 단위로 몰려서 「주」가 실제로 쓸모 있다. */
export type EarningsRangeMode = "month" | "week" | "day";

/** 한 회사가 컨센서스를 습관적으로 넘기는지 — 레일 「꾸준히 기대를 넘는 기업」. */
export type EarningsConsistency = {
  symbol: string;
  name: string;
  market: "domestic" | "us";
  history: EarningsHitHistory;
};

/**
 * 우측 레일 3종을 한 응답으로.
 *
 * 예전 레일(상회/하회 TOP)은 **서프라이즈 탭이 이미 더 길게** 보여주는 목록이라
 * 자리값을 못 했다. 셋 다 표에 없는 것만 담는다 — 표는 날짜순이라 「무엇이 큰
 * 발표인가」를 못 말하고, 한 행만 봐서는 「숫자와 주가가 엇갈렸다」와 「이 회사가
 * 늘 기대를 넘는다」를 알 수 없다.
 */
export type EarningsHighlights = {
  big: EarningsEvent[];
  divergent: EarningsEvent[];
  consistent: EarningsConsistency[];
};
