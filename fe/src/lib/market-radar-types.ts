import type { QuoteAssetType } from "@/lib/quotes-types";

export type MarketCountry = "KR" | "US";

export type RankingType = "MARKET_TRADING_AMOUNT" | "MARKET_TRADING_VOLUME";

export type RankingDuration =
  | "realtime"
  | "1d"
  | "1w"
  | "1mo"
  | "3mo"
  | "6mo"
  | "1y";

export type RankingPrice = {
  lastPrice: string;
  basePrice: string;
  changeRate: string | null;
};

export type RankingItem = {
  rank: number;
  symbol: string;
  currency: "KRW" | "USD";
  price: RankingPrice;
  tradingVolume: string;
  tradingAmount: string;
  /**
   * 종목명 — **서버가 같이 준다.**
   *
   * 예전에는 화면이 심볼만 받고 이름을 두 번째 요청으로 채웠다. 그 호출이
   * `requestIdleCallback` 뒤에 나가서 목록이 종목코드로 먼저 그려졌다가 이름으로
   * 바뀌었고, 배경 탭에서는 idle 콜백이 안 돌아 코드가 그대로 남았다.
   *
   * 카탈로그에 없는 종목은 비어서 온다 — 그때만 화면이 심볼로 되돌린다.
   */
  name?: string;
  assetType?: QuoteAssetType;
  logoUrl?: string;
};

/** `/rankings/prices` 한 줄 — 5초마다 숫자만 덮어쓰기 위한 최소 정보. */
export type RankingPriceRow = {
  symbol: string;
  lastPrice: number;
  changeRate: number;
  currency?: "KRW" | "USD";
};

export type RankingsResponse = {
  rankedAt: string | null;
  /** BE poller refresh time. Prefer over rankedAt for display. */
  refreshedAt?: string | null;
  /**
   * 이 목록이 속한 시장의 세션 — 행이 아니라 응답에 한 번 붙는다(목록 전체가
   * 한 거래소 달력을 쓴다). 서버가 판정한다: `regular` · `pre` · `after` ·
   * `day` · `closed`.
   */
  marketSession?: string;
  /**
   * 마감 세션에서만 오는 두 시각(ISO).
   *
   * `sessionClosedAt` — 이 목록의 가격이 멈춘 시각. 툴바 시계가 이것을 날짜와
   * 함께 찍는다(`lib/market-radar-clock`).
   * `nextSessionAt` — 다음 창이 열리는 시각. 화면은 그때까지 `/rankings/prices`
   * 폴링을 멈춘다 — 가격이 바뀔 수 없는 구간이다.
   */
  sessionClosedAt?: string | null;
  nextSessionAt?: string | null;
  rankings: RankingItem[];
};

export type MarketRadarParams = {
  market: MarketCountry;
  type: RankingType;
  duration: RankingDuration;
  count: number;
};

export type EnrichedRankingItem = RankingItem & {
  name: string;
  market: string;
  assetType?: QuoteAssetType;
  logoUrl?: string;
  rankDelta: number | null;
  isWatchlisted: boolean;
  isHeld: boolean;
  /**
   * 5초 갱신에서 이 행의 가격이 **실제로 바뀐 횟수**. 값이 그대로면 증가하지 않는다.
   *
   * 화면은 이 숫자가 바뀌는 것만 보고 플래시를 준다 — 가격 문자열을 비교하면
   * 리렌더마다 애니메이션이 다시 돌아 목록이 계속 깜빡인다.
   */
  priceTick?: number;
  /** 마지막 변동의 방향. 플래시 색(상승 빨강·하락 파랑)을 고른다. */
  priceDirection?: "up" | "down";
};
