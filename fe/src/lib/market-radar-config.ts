import type {
  MarketCountry,
  MarketRadarParams,
  RankingDuration,
  RankingType,
} from "@/lib/market-radar-types";
import { PRICE_POLL_MS } from "@/lib/price-cadence";

export const MARKET_RADAR_DEFAULT_COUNT = 100;

/** FE fallback refresh interval before `market_data_settings` loads; actual cadence is server-configured (see useMarketDataSettings). */
export const MARKET_RADAR_POLL_MS = 60_000;

/** Quote overlay for list price/change — same API path as detail panel. */
export const MARKET_RADAR_QUOTE_POLL_MS = PRICE_POLL_MS;

/*
랭킹 목록의 가격·등락률 갱신 주기.

**순위 폴링과 다른 주기다.** 순위 스냅샷은 서버가 5분마다 갱신하므로 그쪽을 자주
불러 봐야 같은 답을 다시 받는다. 움직이는 것은 가격뿐이라 `/rankings/prices`만
따로 당겨온다.

1.5초가 안전한 근거는 요청당 비용이다. 이 경로를 세 번 손봤고 그때마다 같은
방법으로 쟀다(`Com_select` 전역 카운터, 10회 중 최솟값 — 배경 배치 버스트를
걸러내려고):

	/rankings·/rankings/prices 100종목 (US 장중)
	  캐시 없음                     658 SELECT
	  자산구분·전일종가 캐시 후       43 SELECT
	  목록에서 체결·호가 조회 제거     4 SELECT   ← 지금

	요청당 Redis 명령: 244 → 91 (같은 요청 안에서 같은 키를 두 번 안 읽는다)
	서버 처리 시간: 7~18ms

**주기를 서버 폴러에 맞춘다.** 화면 값의 신선도 하한을 정하는 것은 서버 폴러이고,
그 주기를 실측하면 종목이 두 갈래로 갈린다(`market:price:v1`의 updatedAt 간격,
0.2초 간격 표본):

	009150 (일반 랭킹 종목)      median 999ms  (932~1054, 매우 균일)
	005930·000660 (웹소켓 구독)  median 578ms  (체결이 같은 키에 쓰인다)

위 표의 median 999ms가 그대로 `PRICE_POLL_MS`(1초)의 근거다. 09-04에 2초로 갔다가
같은 날 1초로 돌아왔는데, **자기 숫자를 다시 쓰지 않고 공용 상수를 import한다** —
숫자가 여기 있으면 다음에 또 한쪽만 바뀐다(그 사고가 이 파일에서 두 번 났다).

**비용은 뷰어 수에 정비례한다** — Toss 호출과 달리 캐시로 흡수할 수 없다(요청
수 자체가 느는 것이라서). 요청당 서버 작업은 4 SELECT · Redis 91명령 · 7~18ms
(실측 2026-09-04에도 4 SELECT · 80ms로 그대로다).

**그래서 지금 이 상수는 보조 경로다.** 값이 실제로 흐르는 길은 `useQuoteStream`
(다심볼 SSE)이고, 스트림이 붙어 있는 동안 훅은 `PRICE_POLL_MS_WHILE_STREAMING`
(30초)으로 물러난다. 이 1초가 실제로 쓰이는 것은 **스트림이 아직 안 붙었거나
끊긴 동안**뿐이라, 뷰어당 0.5 req/s가 1 req/s가 되는 것은 그 짧은 구간에 한한다.

마감 세션에서는 서버가 빈 배열을 주므로 사실상 공짜다 — 하루의 대부분이 그 상태다.
*/
export const MARKET_RADAR_PRICE_POLL_MS = PRICE_POLL_MS;

/** List sparklines cost ~1 candle API each — off by default for faster load. */
export const MARKET_RADAR_SHOW_SPARKLINES = false;

export const RANKING_TYPE_OPTIONS: { id: RankingType; label: string }[] = [
  { id: "MARKET_TRADING_AMOUNT", label: "거래대금" },
  { id: "MARKET_TRADING_VOLUME", label: "거래량" },
];

export const RANKING_DURATION_OPTIONS: { id: RankingDuration; label: string }[] = [
  { id: "realtime", label: "실시간" },
  { id: "1d", label: "1일" },
  { id: "1w", label: "1주" },
  { id: "1mo", label: "1개월" },
  { id: "3mo", label: "3개월" },
  { id: "6mo", label: "6개월" },
  { id: "1y", label: "1년" },
];

const RANKING_DURATION_SET = new Set(RANKING_DURATION_OPTIONS.map((o) => o.id));

export function parseMarketRadarParams(searchParams: URLSearchParams): MarketRadarParams {
  const market = searchParams.get("market") === "US" ? "US" : "KR";
  const type = parseRankingType(searchParams.get("type"));
  const duration = parseRankingDuration(searchParams.get("duration"));

  return { market, type, duration, count: MARKET_RADAR_DEFAULT_COUNT };
}

export function marketRadarSearchParams(params: MarketRadarParams): URLSearchParams {
  const q = new URLSearchParams();
  q.set("market", params.market);
  q.set("type", params.type);
  if (params.duration !== "realtime") {
    q.set("duration", params.duration);
  }
  return q;
}

function parseRankingType(value: string | null): RankingType {
  if (value && RANKING_TYPE_OPTIONS.some((o) => o.id === value)) {
    return value as RankingType;
  }
  return "MARKET_TRADING_AMOUNT";
}

function parseRankingDuration(value: string | null): RankingDuration {
  if (value && RANKING_DURATION_SET.has(value as RankingDuration)) {
    return value as RankingDuration;
  }
  return "realtime";
}

export function rankingMetricLabel(type: RankingType): "amount" | "volume" {
  if (type.includes("VOLUME")) return "volume";
  return "amount";
}

export function changeRateHint(duration: RankingDuration): string {
  if (duration === "realtime") {
    return "전일 대비 등락률 · 실시간 거래대금/거래량 순";
  }
  const label = RANKING_DURATION_OPTIONS.find((o) => o.id === duration)?.label ?? duration;
  return `${label} 누적 거래대금/거래량 순 · 등락률은 전일 대비`;
}

export function rankingRefreshHint(duration: RankingDuration, realtimeMinutes = 5): string {
  if (duration === "realtime") return `${realtimeMinutes}분 주기`;
  if (duration === "1d" || duration === "1w") return "15분 주기";
  return "1시간 주기";
}

export function parseRankingNumber(raw: string | null | undefined): number {
  if (!raw) return 0;
  const n = Number.parseFloat(raw.replace(/,/g, ""));
  return Number.isFinite(n) ? n : 0;
}

/** Toss ranking changeRate uses basePrice (전일 정규장 기준가). Never trust raw changeRate alone. */
export function parseRankingChangeRate(
  raw: string | null | undefined,
  lastPrice?: string | null,
  basePrice?: string | null,
): number {
  const last = parseRankingNumber(lastPrice ?? undefined);
  const base = parseRankingNumber(basePrice ?? undefined);
  if (last > 0 && base > 0) {
    return ((last - base) / base) * 100;
  }
  return 0;
}

export function marketCountryToMarketField(country: MarketCountry): string {
  return country === "US" ? "US" : "KR";
}

export function marketRadarParamsKey(params: MarketRadarParams): string {
  return `${params.market}|${params.type}|${params.duration}`;
}

export function parseMarketRadarSymbol(searchParams: URLSearchParams): string {
  return searchParams.get("symbol")?.trim() ?? "";
}

/** Guess KR/US for market radar toolbar from stock metadata. */
export function inferMarketCountryFromStock(stock: {
  currency?: string;
  market?: string;
}): MarketCountry {
  if (stock.currency === "USD") return "US";
  const market = (stock.market ?? "").toUpperCase();
  if (
    market.includes("NASDAQ") ||
    market.includes("NYSE") ||
    market.includes("AMEX") ||
    market.includes("US") ||
    market.includes("ADR")
  ) {
    return "US";
  }
  return "KR";
}

export function marketRadarHref(
  params: MarketRadarParams,
  options?: { symbol?: string },
): string {
  const q = marketRadarSearchParams(params);
  if (options?.symbol) q.set("symbol", options.symbol);
  return `/market?${q.toString()}`;
}

export function marketRadarSymbolHref(symbol: string, market?: MarketCountry): string {
  return marketRadarHref(
    {
      market: market ?? "KR",
      type: "MARKET_TRADING_AMOUNT",
      duration: "realtime",
      count: MARKET_RADAR_DEFAULT_COUNT,
    },
    { symbol },
  );
}
