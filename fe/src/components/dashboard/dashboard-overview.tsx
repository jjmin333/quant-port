"use client";

import { useEffect, useState } from "react";
import { PriceDisplay } from "@/components/ui/price-display";
import { MiniToggle } from "@/components/ui/mini-toggle";
import {
  SectionCard,
  SectionEmpty,
  SectionSkeleton,
} from "@/components/ui/section-card";
import { useNewsArticleModal } from "@/components/news/news-article-modal-host";
import { ResearchQuickLookModal } from "@/components/research/research-quick-look-modal";
import { StockAvatar } from "@/components/stocks/stock-avatar";
import { SymbolQuickLookModal } from "@/components/stocks/symbol-quick-look-modal";
import { DashboardEventsCard } from "@/components/dashboard/dashboard-events-card";
import {
  LatestNewsCard,
  TodayKeywordsCard,
} from "@/components/dashboard/dashboard-news-cards";
import { LatestReportsCard } from "@/components/dashboard/latest-reports-card";
import {
  MarketBriefingCard,
  useMarketBriefings,
} from "@/components/dashboard/market-briefing-card";
import { fetchDashboardNews } from "@/lib/news-api";
import { fetchQuotesBatchClient, fetchRankingsClient } from "@/lib/market-api";
import {
  SCORE_TONE_TEXT,
  fetchScoreRankingsClient,
  scoreTone,
} from "@/lib/stock-score-api";
import { formatPercent } from "@/lib/format";

import type { MarketCountry } from "@/lib/market-radar-types";
import type { NewsArticle } from "@/lib/news-types";
import type { QuoteSummary } from "@/lib/quotes-types";
import type { ResearchCall } from "@/lib/research-types";
import type { ScoreRankingMarket, StockScoreListItem } from "@/lib/stock-score-api";
import { trendTextClass } from "@/lib/trend-colors";
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

function settled<T>(items: T[]): Block<T> {
  return { status: items.length ? "ready" : "empty", items };
}

/* ── 오늘의 점수 TOP ───────────────────────────────────────────────────────── */

function ScoreTop({ onPick }: { onPick: (symbol: string) => void }) {
  const [market, setMarket] = useState<ScoreRankingMarket>("domestic");
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
  market: ScoreRankingMarket;
  onMarketChange: (m: ScoreRankingMarket) => void;
  onPick: (symbol: string) => void;
}) {
  const [block, setBlock] = useState<Block<StockScoreListItem>>(LOADING);
  const [tradeDate, setTradeDate] = useState("");

  useEffect(() => {
    let alive = true;
    fetchScoreRankingsClient(market, 8)
      .then(({ items, tradeDate: date }) => {
        if (!alive) return;
        setBlock(settled(items));
        setTradeDate(date);
      })
      .catch(() => {
        if (alive) setBlock(settled([]));
      });
    return () => {
      alive = false;
    };
  }, [market]);

  return (
    <SectionCard
      title={
        <span className="flex items-baseline gap-1.5">
          오늘의 점수 TOP
          {tradeDate && (
            <span className="text-[10px] font-normal text-muted-foreground">{tradeDate}</span>
          )}
        </span>
      }
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
    >
      {block.status === "loading" ? (
        <SectionSkeleton rows={8} />
      ) : block.items.length === 0 ? (
        <SectionEmpty text="아직 산출된 점수가 없습니다" />
      ) : (
        <ul className="min-w-0">
          {block.items.map((item) => {
            const tone = scoreTone(item.score);
            return (
              <li key={item.symbol}>
                <button
                  type="button"
                  onClick={() => onPick(item.symbol)}
                  className="flex h-[32px] w-full min-w-0 items-center gap-2 rounded-md px-1 text-left text-xs hover:bg-accent"
                >
                  <span className="w-3 shrink-0 text-right text-[11px] font-medium text-muted-foreground">
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
                  {typeof item.changeRate === "number" && (
                    <span
                      className={cn(
                        "shrink-0 font-mono text-[11px] tabular-nums",
                        trendTextClass(item.changeRate),
                      )}
                    >
                      {formatPercent(item.changeRate)}
                    </span>
                  )}
                  <span
                    className={cn(
                      "w-10 shrink-0 text-right font-mono text-xs font-bold tabular-nums",
                      SCORE_TONE_TEXT[tone],
                    )}
                  >
                    {item.score}
                    {item.grade && (
                      <span className="ml-0.5 text-[10px] font-medium">{item.grade}</span>
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
  onSymbols,
  onLoaded,
}: {
  onPick: (quote: QuoteSummary) => void;
  /**
   * 랭킹이 오는 **즉시** 종목 코드만 위로 올린다. 「실적·공시」 칸이 물어보는 데
   * 필요한 건 코드뿐인데, 예전에는 시세까지 기다리느라 3단으로 줄을 섰다.
   */
  onSymbols: (symbols: string[]) => void;
  /** 「실적·공시」가 종목명을 채우도록 받아 온 시세를 위로 올린다. */
  onLoaded: (quotes: QuoteSummary[]) => void;
}) {
  const [market, setMarket] = useState<MarketCountry>("KR");
  // ScoreTop과 같은 이유로 본문만 key로 다시 마운트한다.
  return (
    <TopMoversBody
      key={market}
      market={market}
      onMarketChange={setMarket}
      onPick={onPick}
      onSymbols={onSymbols}
      onLoaded={onLoaded}
    />
  );
}

function TopMoversBody({
  market,
  onMarketChange,
  onPick,
  onSymbols,
  onLoaded,
}: {
  market: MarketCountry;
  onMarketChange: (m: MarketCountry) => void;
  onPick: (quote: QuoteSummary) => void;
  onSymbols: (symbols: string[]) => void;
  onLoaded: (quotes: QuoteSummary[]) => void;
}) {
  const [block, setBlock] = useState<Block<QuoteSummary>>(LOADING);

  useEffect(() => {
    let alive = true;
    void (async () => {
      try {
        const res = await fetchRankingsClient({
          type: "MARKET_TRADING_AMOUNT",
          marketCountry: market,
          duration: "1d",
          count: 8,
        });
        const symbols = res.rankings.map((r) => r.symbol).slice(0, 8);
        if (!alive) return;
        /*
         * 종목 코드를 **시세보다 먼저** 올린다. 「실적·공시」 칸은 코드만 있으면
         * 물어볼 수 있어, 이 한 줄로 시세와 일정이 나란히 나간다 — 예전에는
         * 랭킹 → 시세 → 일정 3단이었다.
         */
        onSymbols(symbols);
        // 랭킹 응답에는 종목명이 없다. 코드만 보여주면 훑을 수 없으므로 배치 시세로 채운다.
        const quotes = symbols.length ? await fetchQuotesBatchClient(symbols) : [];
        const bySymbol = new Map(quotes.map((q) => [q.symbol, q]));
        if (!alive) return;
        const ordered = symbols
          .map((s) => bySymbol.get(s))
          .filter((q): q is QuoteSummary => Boolean(q));
        setBlock(settled(ordered));
        onLoaded(ordered);
      } catch {
        if (alive) {
          setBlock(settled([]));
          onSymbols([]);
          onLoaded([]);
        }
      }
    })();
    return () => {
      alive = false;
    };
    // `onSymbols`·`onLoaded`는 부모의 setState라 렌더마다 같은 참조다. 의존성에
    // 넣으면 시장이 바뀌지 않았는데도 이펙트가 다시 도는 길만 열린다.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [market]);

  return (
    <SectionCard
      title="거래대금 상위"
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
        <SectionSkeleton rows={8} />
      ) : block.items.length === 0 ? (
        <SectionEmpty text="랭킹 데이터를 불러오지 못했습니다" />
      ) : (
        <ul className="min-w-0">
          {block.items.map((q, i) => (
            <li key={q.symbol}>
              <button
                type="button"
                onClick={() => onPick(q)}
                className="flex h-[32px] w-full min-w-0 items-center gap-2 rounded-md px-1 text-left text-xs hover:bg-accent"
              >
                <span className="w-3 shrink-0 text-right text-[11px] font-medium text-muted-foreground">
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
                <PriceDisplay
                  value={q.currentPrice}
                  currency={q.currency}
                  className="shrink-0 font-mono text-[11px]"
                />
                <span
                  className={cn(
                    "w-14 shrink-0 text-right font-mono text-xs tabular-nums",
                    trendTextClass(q.changeRate),
                  )}
                >
                  {formatPercent(q.changeRate)}
                </span>
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
  const [call, setCall] = useState<ResearchCall | null>(null);

  /*
   * 기사는 **여기서 한 번만** 받아 목록 카드와 키워드 카드에 내려준다. 둘이 각자 부르면
   * 같은 화면의 「기사 100건 중」과 목록이 서로 다른 스냅샷이 된다(그 사이에 기사가 들어온다).
   *
   * 홈 전용 엔드포인트를 쓴다 — 뉴스 목록과 **건수는 같지만**(키워드 집계와 속보 수가
   * 100건 전체를 세야 한다) 화면에 그리지 않는 92건은 제목·태그·키워드만 실려 온다.
   * 98KB(gzip 29KB) → 35KB(gzip 10KB).
   */
  const [articles, setArticles] = useState<NewsArticle[] | null>(null);
  // 앞 `fullCount`건만 본문·링크가 있다. 그 뒤 기사로 모달을 열면 빈 창이 뜬다.
  const [fullCount, setFullCount] = useState(0);
  useEffect(() => {
    let alive = true;
    fetchDashboardNews("domestic")
      .then((res) => {
        if (!alive) return;
        setArticles(res.articles);
        setFullCount(res.fullCount);
      })
      .catch(() => {
        if (alive) setArticles([]);
      });
    return () => {
      alive = false;
    };
  }, []);

  /*
   * 「다가오는 일정」은 거래대금 상위 종목을 그대로 쓴다. 종목을 따로 고르면 "왜 이
   * 종목의 일정인지"를 설명할 길이 없는데, 바로 옆 칸이 방금 그 목록을 보여줬으므로
   * 이유가 화면에 있다. 이름도 저 목록의 시세에서 오므로 마스터를 다시 부르지 않는다.
   */
  const [topSymbols, setTopSymbols] = useState<string[] | null>(null);
  const [topQuotes, setTopQuotes] = useState<QuoteSummary[] | null>(null);

  /*
   * 기사는 뉴스 화면과 **같은 방식**으로 연다(`useNewsArticleModal`). 예전에는 여기서
   * iframe 셸 하나를 문맥 없이 띄워, 홈에서 연 기사에는 관련 종목·지수 카드가 아예
   * 없었다 — 같은 기사가 뉴스 메뉴에서는 미니 차트까지 달고 열리는데도.
   *
   * `pool`에는 **본문이 온전한 앞부분만** 넘긴다. 뒤쪽 요약본까지 넘기면 「같은 이슈」
   * 목록에 링크 없는 기사가 섞여, 눌렀을 때 아무것도 없는 창이 뜬다.
   */
  const newsModal = useNewsArticleModal({
    pool: articles ? articles.slice(0, fullCount) : undefined,
  });

  const briefings = useMarketBriefings();

  return (
    <div className="space-y-3">
      {/*
        읽는 것(브리핑·뉴스)을 위에, 훑는 것(점수·거래대금·일정·키워드)을 아래에 둔다.
        브리핑이 아직 생성 전이면 뉴스가 전폭으로 늘어난다 — 빈 칸을 두고 "준비 중"이라
        적는 것보다 있는 것으로 채우는 편이 낫다.
      */}
      <div
        className={cn(
          "grid min-w-0 gap-3",
          !briefings.loading && !briefings.domestic && !briefings.us
            ? "grid-cols-1"
            : "lg:grid-cols-[minmax(0,2fr)_minmax(0,1fr)]",
        )}
      >
        <MarketBriefingCard briefings={briefings} className="min-w-0" />
        {/*
          목록 카드에는 **본문이 온전한 것만** 준다. 키워드 카드는 반대로 전체를
          받아야 한다 — 「기사 100건 중 2회 이상」이라는 문장이 100건을 세서 나온다.
        */}
        <LatestNewsCard
          articles={articles ? articles.slice(0, fullCount) : null}
          onPick={newsModal.open}
        />
      </div>

      <div className="grid min-w-0 gap-3 sm:grid-cols-2 xl:grid-cols-4">
        <ScoreTop onPick={(symbol) => setPicked({ symbol, seed: null })} />
        <TopMovers
          onPick={(quote) => setPicked({ symbol: quote.symbol, seed: quote })}
          onSymbols={setTopSymbols}
          onLoaded={setTopQuotes}
        />
        <DashboardEventsCard symbols={topSymbols} quotes={topQuotes} />
        <TodayKeywordsCard articles={articles} />
      </div>

      <LatestReportsCard onPick={setCall} />

      <SymbolQuickLookModal
        symbol={picked?.symbol ?? null}
        seed={picked?.seed}
        onClose={() => setPicked(null)}
      />
      {newsModal.modal}
      <ResearchQuickLookModal call={call} open={call != null} onClose={() => setCall(null)} />
    </div>
  );
}
