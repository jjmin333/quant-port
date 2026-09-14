"use client";

import { useEffect, useMemo, useState } from "react";
import { QuoteCell } from "@/components/ui/quote-cell";
import { useLiveQuotes } from "@/hooks/use-live-quotes";
import { MiniToggle } from "@/components/ui/mini-toggle";
import { useOpenMarket } from "@/hooks/use-open-market";
import {
  SectionCard,
  SectionEmpty,
  SectionSkeleton,
} from "@/components/ui/section-card";
import { useNewsArticleModal } from "@/components/news/news-article-modal-host";
import { StockAvatar } from "@/components/stocks/stock-avatar";
import { SymbolQuickLookModal } from "@/components/stocks/symbol-quick-look-modal";
import { DashboardEventsCard } from "@/components/dashboard/dashboard-events-card";
import {
  DASHBOARD_NEWS_ROWS,
  LatestNewsCard,
} from "@/components/dashboard/dashboard-news-cards";
import { TrendingKeywordsCard } from "@/components/news/trending-keywords-card";
import {
  MarketBriefingCard,
  useMarketBriefings,
} from "@/components/dashboard/market-briefing-card";
import { newsCardSharesBriefingRow } from "@/lib/dashboard-news-layout";
import { fetchDashboardNews } from "@/lib/news-api";
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
import type { ScoreRankingMarket, StockScoreListItem } from "@/lib/stock-score-api";
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

type Block<T> = { status: "loading" | "ready" | "empty"; items: T[] };

const LOADING: Block<never> = { status: "loading", items: [] };

/**
 * 「실시간 AI 추천 랭킹 TOP」과 「거래대금 상위」가 **같은 줄 수**로 선다
 * (2026-09-05 사용자 요청 2).
 *
 * 둘은 같은 행의 이웃인데 점수는 여덟 줄, 거래대금은 일곱 줄이라 카드 높이가
 * 44px 어긋나 있었다(실측 322px · 278px). 거래대금이 하나 모자랐던 것은 여덟을
 * 받아 놓고 **시세가 없는 한 종목을 버렸기** 때문이다 — 그날은 386380(ETN)이라
 * `/quotes/batch`가 일곱 건만 돌려줬다. 그래서 낮은 쪽에 맞춘다.
 */
const DASHBOARD_RANK_ROWS = 7;

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

/* ── AI 추천 랭킹 TOP ──────────────────────────────────────────────────────── */

function ScoreTop({ onPick }: { onPick: (symbol: string) => void }) {
  // 기본값은 **지금 열려 있는 시장**이다 (2026-09-11 사용자 요청 9) — 둘 다 열렸거나
  // 둘 다 닫혔으면 국내. 정해지기 전에는 null이고 본문이 스켈레톤을 지킨다.
  const [market, setMarket] = useOpenMarket(SCORE_MARKET_PAIR);
  /*
   * 시장을 바꾸면 `key`로 본문만 다시 마운트한다. 이펙트 안에서 로딩으로 되돌리면
   * 렌더가 한 번 더 돌고(react-hooks/set-state-in-effect), 그 한 프레임 동안 **국내
   * 순위가 "해외" 탭 밑에 남는다**. 토글 자체는 이 컴포넌트에 있어 다시 마운트되지 않는다.
   */
  return (
    <ScoreTopBody key={market} market={market} onMarketChange={setMarket} onPick={onPick} />
  );
}

function ScoreTopBody({
  market,
  onMarketChange,
  onPick,
}: {
  market: ScoreRankingMarket | null;
  onMarketChange: (m: ScoreRankingMarket) => void;
  onPick: (symbol: string) => void;
}) {
  const [block, setBlock] = useState<Block<StockScoreListItem>>(LOADING);

  useEffect(() => {
    if (market == null) return; // 어느 시장을 받을지 아직 모른다
    let alive = true;
    // 홈 카드는 언제나 **종합 점수 · 최신 배치일**이다 — 축·기간은 랭킹 화면의 손잡이다.
    fetchScoreRankingsClient({ market, limit: DASHBOARD_RANK_ROWS })
      .then(({ items }) => {
        if (!alive) return;
        setBlock(settled(items));
      })
      .catch(() => {
        if (alive) setBlock(settled([]));
      });
    return () => {
      alive = false;
    };
  }, [market]);

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
      title="실시간 AI 추천 랭킹 TOP"
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
    >
      {block.status === "loading" ? (
        <SectionSkeleton rows={DASHBOARD_RANK_ROWS} />
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
  const [block, setBlock] = useState<Block<QuoteSummary>>(LOADING);

  useEffect(() => {
    if (market == null) return; // 어느 시장을 받을지 아직 모른다
    let alive = true;
    void (async () => {
      try {
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
        if (!alive) return;
        // 랭킹 응답에는 종목명이 없다. 코드만 보여주면 훑을 수 없으므로 배치 시세로 채운다.
        const quotes = symbols.length ? await fetchQuotesBatchClient(symbols) : [];
        const bySymbol = new Map(quotes.map((q) => [q.symbol, q]));
        if (!alive) return;
        const ordered = symbols
          .map((s) => bySymbol.get(s))
          .filter((q): q is QuoteSummary => Boolean(q))
          .slice(0, DASHBOARD_RANK_ROWS);
        setBlock(settled(ordered));
      } catch {
        if (alive) setBlock(settled([]));
      }
    })();
    return () => {
      alive = false;
    };
  }, [market]);

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
      ) : block.items.length === 0 ? (
        <SectionEmpty text="랭킹 데이터를 불러오지 못했습니다" />
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

export function DashboardOverview() {
  /*
   * 모달 상태를 여기 한곳에 둔다. 블록마다 자기 모달을 들면 종목 모달이 두 벌 뜨고,
   * 점수 TOP에서 하나를 연 채 거래대금에서 또 누르면 둘이 겹친다.
   *
   * 거래대금 블록은 시세를 **이미 들고 있어** 그대로 넘긴다(요청 0). 점수 랭킹은 종가만
   * 알아서 심볼만 넘기고, 모달이 열릴 때 배치 시세를 한 번 부른다.
   */
  const [picked, setPicked] = useState<{ symbol: string; seed: QuoteSummary | null } | null>(null);

  /*
   * 키워드 카드는 국내 기사 전체를 받는다 — 「기사 100건 중 2회 이상」이 100건을
   * 세야 한다. 실시간 뉴스 카드는 자기 시장을 따로 받아, 해외로 바꿔도 키워드가
   * 따라가지 않는다.
   *
   * 홈 전용 엔드포인트를 쓴다 — 뉴스 목록과 **건수는 같지만** 화면에 그리지 않는
   * 92건은 제목·태그·키워드만 실려 온다. 98KB(gzip 29KB) → 35KB(gzip 10KB).
   */
  const [articles, setArticles] = useState<NewsArticle[] | null>(null);
  const [newsPool, setNewsPool] = useState<NewsArticle[]>([]);
  useEffect(() => {
    let alive = true;
    // 옆의 뉴스 카드와 **같은 수**로 부른다 — 그래야 둘이 한 요청을 나눠 쓴다
    // (`DASHBOARD_NEWS_ROWS` 주석 참조). 받는 목록의 길이는 어느 쪽이든 같다.
    fetchDashboardNews("domestic", DASHBOARD_NEWS_ROWS)
      .then((res) => {
        if (alive) setArticles(res.articles);
      })
      .catch(() => {
        if (alive) setArticles([]);
      });
    return () => {
      alive = false;
    };
  }, []);

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
        <MarketBriefingCard briefings={briefings} className="min-w-0" />
        <div className={cn("min-w-0", newsSharesRow && "lg:relative lg:min-h-[22rem]")}>
          <LatestNewsCard
            onPick={newsModal.open}
            onPoolChange={setNewsPool}
            fillRow={newsSharesRow}
            className={cn(newsSharesRow && "lg:absolute lg:inset-0")}
          />
        </div>
      </div>

      <div className="grid min-w-0 items-start gap-3 sm:grid-cols-2 xl:grid-cols-4">
        <ScoreTop onPick={(symbol) => setPicked({ symbol, seed: null })} />
        <TopMovers onPick={(quote) => setPicked({ symbol: quote.symbol, seed: quote })} />
        <DashboardEventsCard onPick={(symbol) => setPicked({ symbol, seed: null })} />
        {/*
          뉴스 레일과 **같은 부품**이다 (2026-09-05 사용자 요청 7).
          홈에서는 접힘·각주를 달고(`variant="full"` 기본), 누르면 뉴스 화면으로
          검색어를 싣고 나간다 — 홈에는 볼 기사가 열넷뿐이라 좁히는 일은 목록이
          다 있는 쪽에서 하는 게 맞다.
        */}
        <TrendingKeywordsCard market="domestic" articles={articles} />
      </div>

      {/*
        「최신 투자 리포트」 카드는 뺐다(2026-09-03 사용자 결정). 홈에서 전폭 한 줄을
        통째로 쓰면서, 위 네 카드처럼 훑고 지나가는 물건이 아니라 하나를 정독하는
        물건이라 그 자리 값을 못 했다. 리포트는 `/reports`가 제 자리다.
      */}
      <SymbolQuickLookModal
        symbol={picked?.symbol ?? null}
        seed={picked?.seed}
        onClose={() => setPicked(null)}
      />
      {newsModal.modal}
    </div>
  );
}
