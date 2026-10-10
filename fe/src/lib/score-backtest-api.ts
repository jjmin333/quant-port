import { apiRequest } from "@/lib/api-client";
import { cachedGet } from "@/lib/fetch-cache";
import type { ScoreRankingAxis } from "@/lib/score-ranking-config";
import type { ScoreRankingMarket } from "@/lib/stock-score-api";

/**
 * AI 랭킹 백테스트 — `be/internal/model/score_backtest.go`의 거울.
 *
 * # 이 화면이 답하는 질문
 *
 * 「이 랭킹을 믿고 샀으면 어떻게 됐나」. 점수는 매일 나오는데 그 점수가 맞았는지는
 * 지금까지 앱 어디에서도 알 수 없었다.
 *
 * # 읽는 쪽이 알아야 할 것 셋
 *
 *   · **미래를 보지 않는다** — D일 랭킹의 수익은 D+1일부터 센다(서버가 그렇게 잰다).
 *   · **표본이 짧으면 서버가 비운다** — `annualizedReturn`·`sharpe`가 없는 것은
 *     오류가 아니라 「아직 말할 수 없다」이고, 화면은 그 자리를 0으로 채우면 안 된다.
 *   · **벤치마크마다 잰 날 수가 다르다** — 지수는 휴장일이 달라서, `days`가 포트폴리오의
 *     거래일 수와 어긋날 수 있다. 어긋나면 화면이 그 사실을 적는다.
 */

export type BacktestRebalance = "daily" | "weekly" | "monthly";

export const BACKTEST_REBALANCE_OPTIONS: { id: BacktestRebalance; label: string }[] = [
  { id: "daily", label: "매일" },
  { id: "weekly", label: "매주" },
  { id: "monthly", label: "매월" },
];

export type BacktestMetrics = {
  totalReturn: number;
  /** 표본이 40거래일 미만이면 없다 — 짧은 창을 연율화하면 외삽이 된다. */
  annualizedReturn?: number;
  mdd: number;
  volatility?: number;
  /** 연 환산과 같은 문턱(40거래일). 없으면 비워 둔다. */
  sharpe?: number;
  winRate?: number;
  bestDay?: number;
  worstDay?: number;
};

export type BacktestBenchmark = {
  id: string;
  label: string;
  primary: boolean;
  metrics: BacktestMetrics;
  /** 포트폴리오 − 이 지수 (%p). */
  alpha: number;
  /** 이 지수가 실제로 잰 날 수. 휴장일이 달라 포트폴리오와 어긋날 수 있다. */
  days: number;
};

export type BacktestPoint = {
  date: string;
  /** 시작일 = 100 기준. */
  portfolio: number;
  /** 지수 id → 같은 100 기준 값. 그날 휴장인 지수는 키가 없다. */
  benchmarks?: Record<string, number>;
  holdings: number;
};

export type BacktestCoverage = {
  rankingDates: number;
  tradingDays: number;
  rebalances: number;
  avgHoldings: number;
  missingPrices: number;
  turnover?: number;
  /** false면 화면은 숫자를 성적이 아니라 **관찰**로 적어야 한다. */
  sufficient: boolean;
  note?: string;
};

export type BacktestResult = {
  request: {
    market: ScoreRankingMarket;
    axis: ScoreRankingAxis;
    topN: number;
    rebalance: BacktestRebalance;
    benchmarks?: string[];
    costBps: number;
  };
  /** 요청한 창이 아니라 **실제로 가진 창**이다. */
  fromDate: string;
  toDate: string;
  portfolio: BacktestMetrics;
  benchmarks: BacktestBenchmark[];
  /** 머리줄에 쓰는 기준 벤치마크의 id와 초과수익. */
  benchmarkId?: string;
  alpha?: number;
  curve: BacktestPoint[];
  coverage: BacktestCoverage;
};

export type BacktestQuery = {
  market: ScoreRankingMarket;
  axis: ScoreRankingAxis;
  topN?: number;
  rebalance?: BacktestRebalance;
  benchmarks?: string[];
};

/**
 * 한 번의 백테스트. 실측 0.24초 · 2.9KB.
 *
 * 같은 손잡이 조합이면 답이 같다(하루 한 번 도는 배치의 산물) — 목록 툴바를 오가며
 * 같은 조합으로 돌아오는 자리라 캐시를 둔다.
 *
 * **이력이 없으면 `null`이다.** 이 기능이 켜진 첫날이나 요청한 창에 스냅샷이 없는
 * 경우이고, 오류가 아니다 — 화면은 0%짜리 곡선 대신 「아직 쌓이지 않았습니다」를 그린다.
 */
export function fetchScoreBacktestClient(q: BacktestQuery): Promise<BacktestResult | null> {
  const params = new URLSearchParams({ market: q.market, axis: q.axis });
  if (q.topN) params.set("topN", String(q.topN));
  if (q.rebalance) params.set("rebalance", q.rebalance);
  if (q.benchmarks?.length) params.set("benchmarks", q.benchmarks.join(","));
  const path = `/api/v1/stocks/scores/backtest?${params.toString()}`;
  return cachedGet(
    path,
    () => apiRequest<BacktestResult | null>(path, undefined, "백테스트 조회 실패"),
    60_000,
  );
}

export type BacktestBenchmarkCatalog = {
  catalog: Record<string, { id: string; label: string }[]>;
  defaults: Record<string, string[]>;
};

/**
 * 고를 수 있는 지수 목록.
 *
 * 화면이 한국어 이름을 하드코딩하지 않게 하는 입구다 — 카탈로그가 늘면 목록만 길어진다.
 * 거의 바뀌지 않으므로 캐시를 길게 둔다.
 */
export function fetchBacktestBenchmarksClient(): Promise<BacktestBenchmarkCatalog | null> {
  const path = "/api/v1/stocks/scores/backtest/benchmarks";
  return cachedGet(
    path,
    () => apiRequest<BacktestBenchmarkCatalog | null>(path, undefined, "벤치마크 목록 조회 실패"),
    30 * 60_000,
  );
}

/* ------------------------------------------------------------------------- */
/* 표시 규칙                                                                  */
/* ------------------------------------------------------------------------- */

/** +7.30% / −1.59% / 0% — 부호를 항상 붙인다. */
export function formatPct(value: number | undefined, digits = 2, suffix = "%"): string {
  if (typeof value !== "number") return "—";
  const v = Number(value.toFixed(digits));
  if (v === 0) return `0${suffix}`;
  return `${v > 0 ? "+" : "−"}${Math.abs(v).toFixed(digits)}${suffix}`;
}

/** MDD는 언제나 음수이거나 0이다 — 부호를 붙이지 않고 그대로 적는다. */
export function formatDrawdown(value: number | undefined): string {
  if (typeof value !== "number") return "—";
  return `${value.toFixed(2)}%`;
}

/**
 * 지수 곡선의 선 모양.
 *
 * 색으로 가르지 않는다 — 이 앱에서 빨강·파랑은 상승·하락이고, 벤치마크를 그 색으로
 * 그리면 「KOSPI는 빨간 시장인가」가 된다. 포트폴리오만 색을 갖고 지수는 **점선의
 * 간격**으로 구분한다.
 */
export const BENCHMARK_DASH: string[] = ["5 3", "2 3", "8 3 2 3"];
