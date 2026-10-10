"use client";

import { RedditMoodCard } from "@/components/dashboard/reddit-mood-card";
import { RateTopCard, useHomeAssets } from "@/components/dashboard/asset-pulse-cards";
import { ReWatchHomeCard } from "@/components/wealth/re-watch";
import { ASSET_FEATURES_HIDDEN } from "@/lib/hidden-features";
import { useQuickLook } from "@/components/providers/quick-look-provider";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { QuoteCell } from "@/components/ui/quote-cell";
import { useLiveQuotes } from "@/hooks/use-live-quotes";
import { useVisibleInterval } from "@/hooks/use-visible-interval";
import {
  SCREEN_REFRESH_MS,
  SCREEN_SLOW_REFRESH_MS,
} from "@/lib/screen-refresh";
import { MiniToggle } from "@/components/ui/mini-toggle";
import { DASHBOARD_RANK_ROWS } from "@/lib/dashboard-rows";
import { useMarketClosed, useOpenMarket } from "@/hooks/use-open-market";
import { LoadErrorState } from "@/components/common/load-error-state";
import { toUserMessage } from "@/lib/error-message";
import { tradeDateLabel } from "@/lib/data-freshness";
import type { ServerScoreSeed } from "@/lib/home-api.server";
import {
  SectionCard,
  SectionEmpty,
  SectionSkeleton,
} from "@/components/ui/section-card";
import { useNewsArticleModal } from "@/components/news/news-article-modal-host";
import { StockAvatar } from "@/components/stocks/stock-avatar";
import { DashboardEventsCard } from "@/components/dashboard/dashboard-events-card";
import { MyStocksCard } from "@/components/dashboard/my-stocks-card";
import {
  LatestNewsCard,
} from "@/components/dashboard/dashboard-news-cards";
import {
  MarketBriefingCard,
  useMarketBriefings,
} from "@/components/dashboard/market-briefing-card";
import { newsCardSharesBriefingRow } from "@/lib/dashboard-news-layout";
import { fetchQuotesBatchClient, fetchRankingsClient } from "@/lib/market-api";
import {
  SCORE_TONE_TEXT,
  fetchScoreRankingsClient,
  priceAsOfLabel,
  scoreTone,
} from "@/lib/stock-score-api";

import { MARKET_RADAR_DEFAULT_COUNT, marketRadarHref } from "@/lib/market-radar-config";
import type { MarketCountry } from "@/lib/market-radar-types";
import type { NewsArticle } from "@/lib/news-types";
import type { QuoteSummary } from "@/lib/quotes-types";
import type { ScoreRankingMarket } from "@/lib/stock-score-api";
import { cn } from "@/lib/utils";

/**
 * 홈 종합 대시보드 — 여러 메뉴의 핵심만 한 화면에 모은다.
 *
 * 각 블록은 **독립적으로 실패한다**. 마켓 레이더가 죽어도 뉴스는 떠야 하고, 로그인이
 * 없어 리포트가 401이어도 나머지는 보여야 한다. 그래서 블록마다 자기 fetch·로딩·빈
 * 상태를 따로 가진다 — 하나의 에러가 화면 전체를 빈 화면으로 만들지 않게.
 *
 * 시장 토글(국내/해외)은 블록마다 따로 둔다. 하나로 묶으면 "국내 점수를 보면서 해외
 * 거래대금을 보는" 조합이 불가능해지는데, 그 조합이야말로 대시보드에서 하고 싶은 일이다.
 *
 * **누른 것은 홈에서 열린다** (08-16). 예전에는 네 블록이 전부 다른 화면으로 떠났다 —
 * 종목은 `/analysis`, 뉴스는 `/news/domestic`, 리포트는 `/reports/{id}`. 홈에 온 사람이
 * 하는 일은 하나를 정독하는 게 아니라 **여럿을 견주는 것**이라, 셋을 비교하려면 뒤로가기를
 * 세 번 눌러야 했다. 관심종목·보유·뉴스 화면이 이미 쓰던 흐름(모달로 훑고 마음에 들 때
 * 넘어간다)을 첫 화면에도 똑같이 둔다. 모달 셋은 앱에 이미 있던 것을 연결만 했다.
 */

/**
 * 카드 한 장의 상태.
 *
 * **`error`가 2026-09-20에 생겼다.** 그전에는 셋(`loading`·`ready`·`empty`)뿐이라
 * **첫 적재의 실패가 `empty`로 떨어졌다** — 화면에는 「아직 산출된 점수가 없습니다」가
 * 떴고, 그건 사실이 아니었다. 서버는 멀쩡히 점수를 주고 있는데 우리가 못 받은
 * 것이었고, 사용자는 「없다」를 읽고 기다리기를 그만둔다.
 *
 * 「없다」와 「못 받았다」는 **사용자가 할 일이 다르다**: 앞은 기다리는 것이고
 * 뒤는 다시 눌러 보는 것이다(2026-09-20 점검 요청 6).
 */
type Block<T> = {
  status: "loading" | "ready" | "empty" | "error";
  items: T[];
  /** `status === "error"`일 때의 사용자 문구. */
  error?: string;
};

const LOADING: Block<never> = { status: "loading", items: [] };


/*
국내/해외의 **이름이 카드마다 다르다** — 랭킹은 domestic/overseas, 거래대금은 KR/US다.
`useOpenMarket`은 「열린 쪽」만 판정하고 이름은 이 표에서 읽는다. 모듈 상수로 두는
이유는 훅의 의존성이 값이라서다 — 렌더마다 새 객체를 주면 매 렌더 다시 묻는다.
*/
const SCORE_MARKET_PAIR = { domestic: "domestic", us: "overseas" } as const satisfies {
  domestic: ScoreRankingMarket;
  us: ScoreRankingMarket;
};
const MOVERS_MARKET_PAIR = { domestic: "KR", us: "US" } as const satisfies {
  domestic: MarketCountry;
  us: MarketCountry;
};

function settled<T>(items: T[]): Block<T> {
  return { status: items.length ? "ready" : "empty", items };
}

/**
 * 「장이 열리면 표시됩니다」 뒤에 붙는 다음 개장 시각 — ` · 월 09:00`.
 *
 * 시각을 **함께** 적는 이유는 「기다리라」는 말만으로는 얼마나 기다릴지 모르기
 * 때문이다. 금요일 저녁에 이 문구를 본 사람과 월요일 아침 8시에 본 사람이
 * 해야 할 일이 다르다.
 *
 * 값이 없으면 아무것도 덧붙이지 않는다 — 구버전 서버는 `nextSessionAt`을 안 준다.
 */
function nextSessionLabel(nextSessionAt: string | null): string {
  if (!nextSessionAt) return "";
  const at = new Date(nextSessionAt);
  if (Number.isNaN(at.getTime())) return "";
  return ` · ${at.toLocaleString("ko-KR", {
    timeZone: "Asia/Seoul",
    weekday: "short",
    hour: "2-digit",
    minute: "2-digit",
  })}`;
}

/**
 * 카드 한 장의 적재 규칙 — **받고 · 주기적으로 다시 받고 · 갱신 실패는 화면을 지킨다**
 * (2026-09-16 사용자 요청, 주기는 `lib/screen-refresh`).
 *
 * 홈의 카드는 저마다 한 번 받고 그대로 서 있었다. 가격은 스트림이 밀어 줘서 움직였지만
 * **순위와 목록은 마운트 시각에 묶여** 있었고, 열어 둔 채 하루가 지나면 어제의 거래대금
 * 상위가 「오늘」인 척 서 있었다.
 *
 * 두 카드가 같은 실수를 두 번 하지 않게 한 이름으로 묶는다. 다른 곳(뉴스·브리핑)은
 * 반환값 모양이 달라 각자 같은 규칙을 편다.
 *
 * @param load 없으면(`null`) 아직 무엇을 받을지 모르는 것 — 첫 적재도 주기도 걸지 않는다.
 */
function useRefreshingBlock<T>(
  load: (() => Promise<T[]>) | null,
  refreshMs: number,
  /**
   * 서버가 미리 받아 둔 첫 목록 (`lib/home-api.server.ts`).
   *
   * 있으면 **첫 렌더부터 `ready`**다 — 스켈레톤을 한 프레임도 거치지 않고,
   * 서버 HTML에도 이 목록이 실린다. 없으면 예전처럼 `loading`으로 시작한다.
   *
   * 씨앗이 있어도 아래의 첫 적재는 **그대로 돈다.** 씨앗은 최대 60초 캐시된
   * 값이고, 클라이언트가 받는 것이 더 새것이다 — 씨앗의 일은 그 왕복이
   * 끝나기 전까지 화면을 채워 두는 것뿐이다.
   */
  seed?: readonly T[],
): Block<T> & { retry: () => void } {
  const [block, setBlock] = useState<Block<T>>(() =>
    seed && seed.length > 0 ? { status: "ready", items: [...seed] } : LOADING,
  );
  /** 언마운트 뒤 도착한 응답을 버린다 — 요청을 거는 자리가 첫 적재와 주기 둘이다. */
  const aliveRef = useRef(true);
  /**
   * 한 번이라도 받아 본 적이 있는가 — 첫 적재의 실패와 갱신의 실패를 가른다.
   *
   * **씨앗이 있으면 이미 받아 본 것으로 친다.** 서버가 준 목록이 화면에 떠 있는데
   * 클라이언트의 첫 왕복이 실패했다고 그 목록을 오류 화면으로 바꾸면, 보고 있던
   * 것을 없애는 일이 된다 — 갱신 실패와 같은 상황이다.
   */
  const loadedRef = useRef(seed != null && seed.length > 0);
  /** 「다시 시도」가 올리는 값. `run`을 한 번 더 돌린다. */
  const [retryKey, setRetryKey] = useState(0);
  useEffect(() => {
    aliveRef.current = true;
    return () => {
      aliveRef.current = false;
    };
  }, []);

  const run = useCallback(() => {
    if (!load) return;
    void load()
      .then((items) => {
        if (!aliveRef.current) return;
        loadedRef.current = true;
        setBlock(settled(items));
      })
      .catch((err) => {
        // 갱신이 실패하면 보고 있던 목록을 지우지 않는다. 첫 적재만 오류로 떨어진다.
        if (!aliveRef.current || loadedRef.current) return;
        /*
          **`settled([])`가 아니다** (2026-09-20 점검 요청 6). 예전에는 여기서 빈
          목록으로 떨어뜨렸고, 그래서 못 받은 것이 「없다」로 보였다 — 서버는
          멀쩡히 주고 있는데 화면이 「아직 산출된 점수가 없습니다」를 그렸다.
        */
        setBlock({
          status: "error",
          items: [],
          error: toUserMessage(err, "목록을 불러오지 못했습니다."),
        });
      });
  }, [load]);

  useEffect(() => {
    run();
    // `retryKey`는 이펙트를 한 번 더 돌리기 위한 의존성이다 — 값 자체는 안 쓴다.
  }, [run, retryKey]);

  useVisibleInterval(run, refreshMs, load != null);

  return {
    ...block,
    retry: () => {
      setBlock((prev) => (prev.status === "error" ? LOADING : prev));
      setRetryKey((k) => k + 1);
    },
  };
}

/* ── AI 추천 랭킹 TOP ──────────────────────────────────────────────────────── */

function ScoreTop({
  seed,
  onPick,
}: {
  seed?: ServerScoreSeed;
  onPick: (symbol: string) => void;
}) {
  // 기본값은 **지금 열려 있는 시장**이다 (2026-09-11 사용자 요청 9) — 둘 다 열렸거나
  // 둘 다 닫혔으면 국내. 정해지기 전에는 null이고 본문이 스켈레톤을 지킨다.
  const [market, setMarket] = useOpenMarket(SCORE_MARKET_PAIR);
  /*
   * 시장을 바꾸면 `key`로 본문만 다시 마운트한다. 이펙트 안에서 로딩으로 되돌리면
   * 렌더가 한 번 더 돌고(react-hooks/set-state-in-effect), 그 한 프레임 동안 **국내
   * 순위가 "해외" 탭 밑에 남는다**. 토글 자체는 이 컴포넌트에 있어 다시 마운트되지 않는다.
   */
  return (
    <ScoreTopBody
      key={market}
      market={market}
      seed={seed}
      onMarketChange={setMarket}
      onPick={onPick}
    />
  );
}

function ScoreTopBody({
  market,
  seed,
  onMarketChange,
  onPick,
}: {
  market: ScoreRankingMarket | null;
  seed?: ServerScoreSeed;
  onMarketChange: (m: ScoreRankingMarket) => void;
  onPick: (symbol: string) => void;
}) {
  const load = useMemo(
    () =>
      market == null
        ? null // 어느 시장을 받을지 아직 모른다
        : // 홈 카드는 언제나 **종합 점수 · 최신 배치일**이다 — 축·기간은 랭킹 화면의 손잡이다.
          () =>
            fetchScoreRankingsClient({ market, limit: DASHBOARD_RANK_ROWS }).then(
              ({ items }) => items,
            ),
    [market],
  );
  /*
    점수는 **하루 한 번 도는 배치**가 만든다 — 5분마다 물어도 스물아홉 번은 같은 답이라
    느린 주기를 쓴다(`SCREEN_SLOW_REFRESH_MS`). 그래도 0이 아닌 이유는 홈을 열어 둔
    채 날짜가 바뀌는 경우다. 등락률은 아래 `useLiveQuotes`가 따로 밀어 준다.
  */
  /*
    **씨앗은 해외 탭에만 안 쓴다.** 서버가 미리 받아 온 것은 국내 랭킹 하나이므로
    (`app/(main)/page.tsx`), 「해외」라고 적힌 카드 밑에 국내 종목을 깔 수는 없다.

    `market == null`도 받는 이유가 요점이다. 서버 렌더에서 이 값은 **언제나
    null**이다 — 열린 시장은 `/market/clocks`가 답하는데 그건 마운트 뒤에야 온다
    (`useOpenMarket`의 「왜 null로 시작하나」). null일 때 씨앗을 막으면 서버 HTML에
    랭킹이 한 줄도 안 실리고, 그러면 이 선반입은 아무 일도 안 한 것이 된다 —
    처음 그렇게 짰다가 실측에서 잡았다.

    **치르는 값**: 해외장만 열린 시간대(대략 23:30~06:00 KST)에는 토글이 정해지기
    전까지 국내 목록이 잠깐 보였다가 해외로 갈린다. 그 창에서 예전에는 스켈레톤이
    떴다 — 둘 다 최종 목록은 아니고, 이쪽은 적어도 진짜 데이터다. 토글에는 그동안
    아무 쪽도 선택돼 있지 않다. 없애려면 서버가 `/market/clocks`를 한 번 더 불러
    열린 시장을 정해야 하는데, 그건 이 카드가 아니라 `useOpenMarket`의 설계를
    바꾸는 일이라 따로 다룬다.
  */
  const block = useRefreshingBlock(
    load,
    SCREEN_SLOW_REFRESH_MS,
    market === "overseas" ? undefined : seed?.items,
  );

  /*
    카드 바닥에 적을 기준. 점수 랭킹의 기준은 **거래일**이지 벽시계가 아니라
    「낡음」을 판정하지 않는다 — 일요일에 금요일 종가인 것은 정상이다
    (`lib/data-freshness.ts` 머리글의 ⓐ/ⓑ 구분).
  */
  const first = block.items[0] ?? seed?.items[0];
  /*
    「종가 기준」에는 **가격의 날짜**(`priceAsOf`)를 적는다. `tradeDate`는 점수 배치의
    날짜라 휴장일에도 넘어간다 — 추석 연휴(2026-09-24·25 국내 휴장)에 홈이
    「09-24 종가 기준」이라고 적었는데 그날 국내 종가는 없었다(마지막 거래일 09-23).
    `priceAsOf`가 없거나 날짜가 아니면(`live`) 예전처럼 점수 날짜로 적는다.
  */
  const asOf = tradeDateLabel(
    first?.priceAsOf && /^d{4}-d{2}-d{2}/.test(first.priceAsOf)
      ? first.priceAsOf
      : (first?.tradeDate ?? seed?.tradeDate),
  );

  /*
    점수는 **배치가 하루 한 번** 만든 값이고 가격은 **지금**이다 (2026-09-04 사용자 요청 6).

    이 카드는 그동안 등락률만 그렸고 그 값도 배치 응답에 실려 온 종가 기준이라
    화면을 열어 둔 채로는 움직이지 않았다. 이제 가격은 옆 「거래대금 상위」와 같은
    경로로 받는다 — 그래서 제목도 「실시간」이 붙는다. 붙이는 것은 이름만이 아니라
    **그 이름이 참이 되게 하는 데이터**다.
  */
  const liveSymbols = useMemo(() => block.items.map((i) => i.symbol), [block.items]);
  const live = useLiveQuotes(liveSymbols);

  return (
    <SectionCard
      /*
        제목 옆에 **배치일을 붙이지 않는다** (2026-09-05 사용자 결정).

        점수가 언제 것인지 말해 주는 값이었지만, 홈의 좁은 칸(4열)에서 제목을 두 줄로
        접어 카드가 12px 높아졌다 — 옆 「거래대금 상위」와 높이를 맞추려고 줄 수를
        일곱으로 통일한 그 작업의 마지막 12px이 이것이었다. 날짜는 「더보기」가 가는
        `/analysis`에 그대로 있다.
      */
      title="실시간 AI 점수 순위 TOP"
      action={
        <MiniToggle
          options={[
            { id: "domestic" as const, label: "국내" },
            { id: "overseas" as const, label: "해외" },
          ]}
          value={market}
          onChange={onMarketChange}
        />
      }
      href="/analysis"
      /*
       * 링크 문구는 기본값(「더보기」)을 쓴다. 「AI 추천 랭킹」이라고 적어 두었었는데
       * 바로 왼쪽 제목과 같은 말이라, 좁은 홈 칸에서 제목이 「AI 추 / 천 랭 / 킹 TOP」
       * 세 줄로 접히는 값을 치르고 아무 정보도 더하지 않았다.
       */
      footer={asOf ? <span>{asOf}</span> : undefined}
    >
      {block.status === "loading" ? (
        <SectionSkeleton rows={DASHBOARD_RANK_ROWS} />
      ) : block.status === "error" ? (
        /*
          **「없다」가 아니라 「못 받았다」로 말한다** (2026-09-20 점검 요청 6).
          예전에는 이 경우가 아래의 빈 상태로 떨어져, 서버가 멀쩡히 주고 있는
          점수를 화면이 「아직 산출된 점수가 없습니다」라고 말했다.
        */
        <LoadErrorState
          compact
          message={block.error ?? "랭킹을 불러오지 못했습니다."}
          onRetry={block.retry}
        />
      ) : block.items.length === 0 ? (
        <SectionEmpty text="아직 산출된 점수가 없습니다" />
      ) : (
        <ul className="@container min-w-0">
          {block.items.map((item) => {
            const tone = scoreTone(item.score);
            const q = live.get(item.symbol.toUpperCase());
            return (
              <li key={item.symbol}>
                <button
                  type="button"
                  onClick={() => onPick(item.symbol)}
                  className="flex h-[32px] w-full min-w-0 items-center gap-2 rounded-md px-1 text-left text-cell hover:bg-accent"
                >
                  <span className="w-3 shrink-0 text-right text-cell font-medium text-muted-foreground">
                    {item.rank}
                  </span>
                  <StockAvatar
                    symbol={item.symbol}
                    name={item.name}
                    currency={(item.currency as "KRW" | "USD") ?? "KRW"}
                    assetType={item.assetType as "domestic" | "us" | "etf" | undefined}
                    size="xs"
                    className="shrink-0"
                  />
                  <span className="min-w-0 flex-1 truncate">{item.name || item.symbol}</span>
                  {/*
                    가격 · 등락률은 `QuoteCell`이 그린다 — 「거래대금 상위처럼」이
                    사용자가 요청 6에서 말한 기준이고, 그 카드도 같은 부품을 쓴다.
                    색 규칙(0.00%에는 색이 없다)과 「좁으면 가격 생략」도 그쪽에 있다.

                    툴팁의 `priceAsOf`는 남긴다 — 스트림이 아직 안 붙은 동안 보이는
                    값이 언제 것인지는 여전히 질문이 된다.
                  */}
                  <span title={priceAsOfLabel(item.priceAsOf)} className="shrink-0">
                    <QuoteCell
                      price={q?.livePrice ?? item.closePrice ?? 0}
                      changeRate={q?.liveChangeRate ?? item.changeRate ?? 0}
                      currency={(q?.currency ?? item.currency ?? "KRW") as "KRW" | "USD"}
                      symbol={item.symbol}
                    />
                  </span>
                  <span
                    className={cn(
                      "w-10 shrink-0 text-right font-num text-cell font-bold tabular-nums",
                      SCORE_TONE_TEXT[tone],
                    )}
                  >
                    {item.score}
                    {item.grade && (
                      <span className="ml-0.5 text-micro font-medium">{item.grade}</span>
                    )}
                  </span>
                </button>
              </li>
            );
          })}
        </ul>
      )}
    </SectionCard>
  );
}

/* ── 거래대금 상위 ─────────────────────────────────────────────────────────── */

function TopMovers({
  onPick,
}: {
  onPick: (quote: QuoteSummary) => void;
}) {
  // ScoreTop과 같은 규칙 — 열려 있는 시장이 기본값이다.
  const [market, setMarket] = useOpenMarket(MOVERS_MARKET_PAIR);
  // ScoreTop과 같은 이유로 본문만 key로 다시 마운트한다.
  return (
    <TopMoversBody
      key={market}
      market={market}
      onMarketChange={setMarket}
      onPick={onPick}
    />
  );
}

function TopMoversBody({
  market,
  onMarketChange,
  onPick,
}: {
  market: MarketCountry | null;
  onMarketChange: (m: MarketCountry) => void;
  onPick: (quote: QuoteSummary) => void;
}) {
  const load = useMemo(
    () =>
      market == null
        ? null // 어느 시장을 받을지 아직 모른다
        : async () => {
            /*
              **일곱 줄을 채우려고 아홉을 받는다.** 랭킹에는 시세 배치가 못 채우는 종목이
              섞인다(ETN·신규 상장 등) — 그런 줄은 종목코드만 남아 화면에 세울 수 없어
              아래에서 버려진다. 딱 일곱만 받으면 그날의 카드가 여섯 줄이 되고, 옆
              점수 카드와 높이를 맞추자던 이 작업이 되돌아간다.
            */
            const res = await fetchRankingsClient({
              type: "MARKET_TRADING_AMOUNT",
              marketCountry: market,
              duration: "1d",
              count: DASHBOARD_RANK_ROWS + 2,
            });
            const symbols = res.rankings.map((r) => r.symbol).slice(0, DASHBOARD_RANK_ROWS + 2);
            // 랭킹 응답에는 종목명이 없다. 코드만 보여주면 훑을 수 없으므로 배치 시세로 채운다.
            const quotes = symbols.length ? await fetchQuotesBatchClient(symbols) : [];
            const bySymbol = new Map(quotes.map((q) => [q.symbol, q]));
            return symbols
              .map((s) => bySymbol.get(s))
              .filter((q): q is QuoteSummary => Boolean(q))
              .slice(0, DASHBOARD_RANK_ROWS);
          },
    [market],
  );
  /*
    **순위 자체를 5분마다 다시 받는다** (2026-09-16 사용자 요청). 값은 스트림이 밀어
    주고 있었지만 순서는 마운트 시각의 것이라, 홈을 열어 둔 채 하루가 지나면 어제
    거래대금 상위가 그대로 서 있었다. 서버 랭킹 스냅샷이 5분 주기라 그보다 자주 물어야
    할 이유도 없다.
  */
  const block = useRefreshingBlock(load, SCREEN_REFRESH_MS);
  /*
    빈 목록이 **휴장인지 데이터 없음인지** 가르는 값. 거래대금 랭킹은 장이 닫히면
    서버가 정상적으로 0건을 준다(`useMarketClosed` 주석의 실측). 토글의 이름이
    KR/US라 세션 쪽 이름으로 바꿔 묻는다.
  */
  const { closed, nextSessionAt } = useMarketClosed(
    market == null ? null : market === "KR" ? "domestic" : "us",
  );

  // 이 카드도 값이 살아 있어야 한다 — 랭킹 순위는 5분 스냅샷이지만 가격은 지금이다.
  const liveSymbols = useMemo(() => block.items.map((q) => q.symbol), [block.items]);
  const live = useLiveQuotes(liveSymbols);

  return (
    <SectionCard
      title="거래대금 상위"
      /* 시장이 정해지기 전에는 「더보기」가 국내를 가리킨다 — 목적지 링크는 잠깐 뒤
         갱신되고, 그 사이 링크가 없는 것보다 낫다. */
      href={marketRadarHref({
        market: market ?? "KR",
        type: "MARKET_TRADING_AMOUNT",
        duration: "realtime",
        count: MARKET_RADAR_DEFAULT_COUNT,
      })}
      hrefLabel="실시간 차트"
      action={
        <MiniToggle
          options={[
            { id: "KR" as const, label: "국내" },
            { id: "US" as const, label: "해외" },
          ]}
          value={market}
          onChange={onMarketChange}
        />
      }
    >
      {block.status === "loading" ? (
        <SectionSkeleton rows={DASHBOARD_RANK_ROWS} />
      ) : block.status === "error" ? (
        <LoadErrorState
          compact
          message={block.error ?? "랭킹을 불러오지 못했습니다."}
          onRetry={block.retry}
        />
      ) : block.items.length === 0 ? (
        /*
          **빈 목록의 세 가지 뜻을 가른다** (2026-09-20 점검 요청 6).

          이 자리는 원래 한 문장이었다 — 「랭킹 데이터를 불러오지 못했습니다」.
          그런데 장이 닫히면 서버가 **성공적으로** 빈 목록을 준다(실측: 일요일에
          200 · 60바이트 · 0건). 그래서 주말과 장 마감 뒤 내내, 아무 문제 없는
          화면이 문제가 있다고 말하고 있었다.

          이제 실패는 위에서 잡히고, 여기 남는 것은 「받았는데 없다」뿐이다.
        */
        <SectionEmpty
          text={
            closed
              ? `장이 열리면 표시됩니다${nextSessionLabel(nextSessionAt)}`
              : "표시할 랭킹이 없습니다"
          }
        />
      ) : (
        <ul className="@container min-w-0">
          {block.items.map((q, i) => (
            <li key={q.symbol}>
              <button
                type="button"
                onClick={() => onPick(q)}
                className="flex h-[32px] w-full min-w-0 items-center gap-2 rounded-md px-1 text-left text-cell hover:bg-accent"
              >
                <span className="w-3 shrink-0 text-right text-cell font-medium text-muted-foreground">
                  {i + 1}
                </span>
                <StockAvatar
                  symbol={q.symbol}
                  name={q.name}
                  currency={q.currency}
                  assetType={q.assetType}
                  size="xs"
                  className="shrink-0"
                />
                <span className="min-w-0 flex-1 truncate">{q.name || q.symbol}</span>
                {/*
                  값은 **스트림이 밀어 준 것**을 먼저 쓴다 (2026-09-04 사용자 요청 9).
                  이 카드는 랭킹 응답과 배치 시세를 한 번씩 받고 그대로 서 있었다 —
                  홈을 열어 둔 채로는 숫자가 하루 종일 안 움직였다.
                */}
                <QuoteCell
                  price={live.get(q.symbol.toUpperCase())?.livePrice ?? q.currentPrice}
                  changeRate={live.get(q.symbol.toUpperCase())?.liveChangeRate ?? q.changeRate}
                  currency={q.currency}
                  symbol={q.symbol}
                />
              </button>
            </li>
          ))}
        </ul>
      )}
    </SectionCard>
  );
}

/* ── 조립 ────────────────────────────────────────────────────────────────── */

export function DashboardOverview({ scoreSeed }: { scoreSeed?: ServerScoreSeed }) {
  /*
   * 종목은 **전역 퀵룩 하나**로 연다(`QuickLookProvider`, 2026-10-09 요청 1 — 같은 역할의 화면을 하나로).
   * 예전에는 이 화면이 자기 모달을 한 벌 더 들고 있어, 브리핑 카드의 것까지 종목 창이 세 벌이었다.
   * 거래대금 블록은 시세를 **이미 들고 있어** `seed`로 넘긴다(요청 0).
   */
  const quickLook = useQuickLook();
  const openSymbol = (symbol: string, seed?: QuoteSummary | null) => quickLook?.openSymbol(symbol, seed);
  // 금리 TOP · 부동산·금리 탭 · 공매가 나눠 쓰는 한 요청(`/home/assets`).
  const assets = useHomeAssets();
  const [newsPool, setNewsPool] = useState<NewsArticle[]>([]);

  /*
   * 기사는 뉴스 화면과 **같은 방식**으로 연다(`useNewsArticleModal`). 예전에는 여기서
   * iframe 셸 하나를 문맥 없이 띄워, 홈에서 연 기사에는 관련 종목·지수 카드가 아예
   * 없었다 — 같은 기사가 뉴스 메뉴에서는 미니 차트까지 달고 열리는데도.
   *
   * `pool`은 실시간 뉴스 카드가 지금 보여 주는 시장이다. 국내 키워드용 목록을 넘기면
   * 해외 기사 모달에 국내 「같은 이슈」가 붙는다.
   */
  const newsModal = useNewsArticleModal({
    pool: newsPool,
  });

  const briefings = useMarketBriefings();
  const newsSharesRow = newsCardSharesBriefingRow({
    loading: briefings.loading,
    hasDomestic: Boolean(briefings.domestic),
    hasUs: Boolean(briefings.us),
  });

  return (
    <div className="space-y-3">
      {/*
        읽는 것(브리핑·뉴스)을 위에, 훑는 것(점수·거래대금·일정·키워드)을 아래에 둔다.
        브리핑이 아직 생성 전이면 뉴스가 전폭으로 늘어난다 — 빈 칸을 두고 "준비 중"이라
        적는 것보다 있는 것으로 채우는 편이 낫다.
      */}
      {/*
        **한 행에 설 때 행 높이는 브리핑이 정하고, 뉴스가 그 높이를 채운다.**

        실측 1440px: 브리핑 535px · 뉴스 478px라 오른쪽 아래가 57px 비어 있었다. 그렇다고
        뉴스 줄만 늘리면 이번엔 뉴스가 797px이 되어 **브리핑 아래가 262px 비는** 반대
        증상이 된다(실제로 그렇게 나왔다) — 08-28에 고쳤던 「짧은 카드가 늘어나 안이
        비는」 문제 그대로다.

        그래서 뉴스 칸은 **행 높이 계산에 참여하지 않게** 한다: 칸은 자리만 잡고(`relative`)
        카드를 그 안에 절대 배치한다. 절대 배치된 자식은 부모의 내용 높이에 기여하지
        않으므로 행 높이는 브리핑 혼자 정하고, 늘어난 칸을 카드가 채운 뒤 넘치는 기사는
        카드 안에서 스크롤된다.

        `lg:min-h-[22rem]`은 바닥이다 — 브리핑이 유난히 짧은 날 뉴스가 두어 줄로 눌리지
        않게 한다. 모바일은 위아래로 쌓여 맞출 상대가 없으므로 전부 자연 높이 그대로다.
      */}
      {/*
        **행을 나눠 쓰는가**는 세 자리(그리드·칸·카드)가 같은 답을 써야 한다 —
        2026-09-05에 그리드만 「전폭」으로 가고 칸·카드는 절대 배치를 그대로 둬서,
        브리핑이 없는 날 PC 뉴스가 352px 안에 갇혀 **다섯 줄만** 보였다.
        판단은 `newsCardSharesBriefingRow` 한 곳에 있다.
      */}
      <div
        className={cn(
          "grid min-w-0 items-start gap-3",
          newsSharesRow
            ? "lg:grid-cols-[minmax(0,2fr)_minmax(0,1fr)] lg:items-stretch"
            : "grid-cols-1",
        )}
      >
        <MarketBriefingCard briefings={briefings} assets={assets} className="min-w-0" />
        <div className={cn("min-w-0", newsSharesRow && "lg:relative lg:min-h-[22rem]")}>
          <LatestNewsCard
            onPick={newsModal.open}
            onPoolChange={setNewsPool}
            fillRow={newsSharesRow}
            className={cn(newsSharesRow && "lg:absolute lg:inset-0")}
          />
        </div>
      </div>

      {/*
        내 종목은 **브리핑·뉴스 아래**다 (2026-10-07 사용자 요청 2 — 09-26엔 맨 위였다).
        비로그인이면 이 카드는 아무것도 그리지 않아 홈의 첫인상은 그대로다. 예전 「내 종목
        변화」 띠는 이 카드의 줄마다 붙는 변화 한 줄로 옮겼다.
      */}
      <MyStocksCard onPick={openSymbol} />

      {/* 한 행의 카드는 **높이를 맞춘다**(그리드 기본 stretch, 2026-10-07 요청 4) — 짧은 카드 밑으로 배경이 비쳐 보였다. */}
      <div
        className={cn(
          "grid min-w-0 gap-3 sm:grid-cols-2",
          // 금리 TOP이 빠지면(부동산·금융상품 비노출) 세 칸이 한 줄을 채운다.
          ASSET_FEATURES_HIDDEN ? "xl:grid-cols-3" : "xl:grid-cols-4",
        )}
      >
        <ScoreTop seed={scoreSeed} onPick={openSymbol} />
        <TopMovers onPick={(quote) => openSymbol(quote.symbol, quote)} />
        <DashboardEventsCard onPick={openSymbol} />
        {/*
          「오늘의 키워드」 자리 → 「금리 TOP」 (2026-10-09 사용자 요청 7). 키워드는 뉴스 화면 레일에 그대로 있다.
          자료가 없으면 아무것도 그리지 않아 세 칸으로 남는다.
        */}
        {!ASSET_FEATURES_HIDDEN && <RateTopCard d={assets} />}
      </div>

      {/* 관심 단지 새 거래(공매 마감 임박은 10-09 요청 5로 내렸다). 부동산·금리 브리핑은 위 브리핑 카드의 탭으로 갔다(10-09 요청 6). */}
      {!ASSET_FEATURES_HIDDEN && (
        <div className="grid min-w-0 gap-3 empty:hidden lg:grid-cols-2">
          <ReWatchHomeCard />
        </div>
      )}

      {/* 해외 Reddit 커뮤니티 (2026-10-01 요청 3, 10-09 요청 8에서 개편). 자료가 없으면 아무것도 그리지 않는다. */}
      <RedditMoodCard onPick={openSymbol} />

      {/*
        「최신 투자 리포트」 카드는 뺐다(2026-09-03 사용자 결정). 홈에서 전폭 한 줄을
        통째로 쓰면서, 위 네 카드처럼 훑고 지나가는 물건이 아니라 하나를 정독하는
        물건이라 그 자리 값을 못 했다. 리포트는 `/reports`가 제 자리다.
      */}
      {newsModal.modal}
    </div>
  );
}
