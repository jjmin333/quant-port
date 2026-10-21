"use client";

import Link from "next/link";
import { useCallback, useEffect, useMemo, useRef, useState, useSyncExternalStore } from "react";
import { useAuth } from "@/components/providers/auth-provider";
import { SectionCard, SectionSkeleton } from "@/components/ui/section-card";
import { QuoteCell } from "@/components/ui/quote-cell";
import { StockAvatar } from "@/components/stocks/stock-avatar";
import { LoadErrorState } from "@/components/common/load-error-state";
import { useLiveQuotes, type LiveQuote } from "@/hooks/use-live-quotes";
import { daysUntil, useUpcomingEvents } from "@/hooks/use-upcoming-events";
import { useVisibleInterval } from "@/hooks/use-visible-interval";
import { fetchMyToday, type MyToday, type MyTodayItem } from "@/lib/my-today-api";
import { SCREEN_SLOW_REFRESH_MS } from "@/lib/screen-refresh";
import { SCORE_TONE_TEXT, scoreTone } from "@/lib/stock-score-api";
import {
  getWatchlistSymbols,
  isWatchlistLoaded,
  subscribeWatchlist,
} from "@/lib/watchlist-store";
import { toUserMessage } from "@/lib/error-message";
import { cn } from "@/lib/utils";

/** 카드가 그리는 줄 수. 넓은 화면에서는 두 칸으로 네 줄씩 선다. */
const MY_STOCKS_ROWS = 8;
/** 실적 일정은 이 안쪽만 배지로 단다 — 한 달 뒤 일정은 「오늘」의 일이 아니다. */
const EARNINGS_SOON_DAYS = 14;
const EMPTY_SET: Set<string> = new Set();

/**
 * 홈 맨 위 「내 종목」 (2026-09-26 사용자 결정 — 재방문 개선 C).
 *
 * 가입하고 종목을 담아도 홈은 비로그인 화면과 똑같았다. 다시 들어올 이유는 「내 종목에
 * 오늘 무슨 일이 있었나」인데 그걸 보려면 관심·보유·변화·뉴스 화면을 따로 돌아야 했다.
 * 여기서 한 줄에 모은다: 지금 가격 · AI 점수와 어제 대비 · 그리고 **사건**(오늘의 변화
 * 한 줄 · 실적 D-n · 24시간 기사 수).
 *
 * 줄 순서는 **사건이 있는 종목 먼저**, 그다음 서버 순서(보유 → 관심 목록 순서)다.
 * 등락률로 줄을 세우지 않는다 — 장중에는 틱마다 순서가 바뀌어 누르려던 줄이 도망간다.
 *
 * 이 카드가 예전 「내 종목 변화」 띠(`MyChangesStrip`)를 대신한다 — 띠가 하던 변화 한 줄이
 * 여기 줄마다 붙는다. 비로그인이면 아무것도 그리지 않는다(홈의 첫인상을 바꾸지 않는다).
 */
export function MyStocksCard({ onPick }: { onPick: (symbol: string) => void }) {
  const { session, loading: authLoading } = useAuth();
  const loggedIn = Boolean(session);
  const [data, setData] = useState<MyToday | null>(null);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async (isCancelled: () => boolean = () => false) => {
    try {
      const res = await fetchMyToday();
      if (isCancelled()) return;
      setData(res);
      setError(null);
    } catch (e) {
      if (isCancelled()) return;
      // 한 번 받은 값이 있으면 그대로 둔다 — 갱신 실패로 카드가 비면 안 된다.
      setError(toUserMessage(e, "내 종목을 불러오지 못했습니다."));
    }
  }, []);

  useEffect(() => {
    if (!loggedIn) return;
    let cancelled = false;
    const first = async () => {
      await load(() => cancelled);
    };
    void first();
    return () => {
      cancelled = true;
    };
  }, [loggedIn, load]);

  useVisibleInterval(() => void load(), SCREEN_SLOW_REFRESH_MS, loggedIn);

  /*
    다른 카드(랭킹의 하트 등)에서 관심을 담거나 빼면 이 카드도 따라 바뀌어야 한다.
    관심 집합은 앱에 하나다(`watchlist-store`). 첫 적재로 집합이 채워지는 변화는
    건너뛴다 — 그건 사용자가 바꾼 것이 아니라 알게 된 것이다.
  */
  const watchSet = useSyncExternalStore(subscribeWatchlist, getWatchlistSymbols, () => EMPTY_SET);
  const lastWatch = useRef<{ set: Set<string>; loaded: boolean } | null>(null);
  useEffect(() => {
    const prev = lastWatch.current;
    lastWatch.current = { set: watchSet, loaded: isWatchlistLoaded() };
    if (!loggedIn || !prev || !prev.loaded || prev.set === watchSet) return;
    const t = window.setTimeout(() => void load(), 400);
    return () => window.clearTimeout(t);
  }, [watchSet, loggedIn, load]);

  const items = useMemo(() => data?.items ?? [], [data]);
  const symbols = useMemo(() => items.map((i) => i.symbol), [items]);
  const live = useLiveQuotes(symbols);
  const eventTargets = useMemo(
    () => items.map((i) => ({ symbol: i.symbol, name: i.name })),
    [items],
  );
  const { bySymbol: upcoming } = useUpcomingEvents(eventTargets, 20);

  const rows = useMemo(() => {
    const decorated = items.map((item, order) => {
      const next = upcoming.get(item.symbol)?.[0];
      const dday = next ? daysUntil(next.date) : null;
      const earningsSoon = dday !== null && dday <= EARNINGS_SOON_DAYS ? dday : null;
      const weight =
        (item.change ? 4 : 0) +
        (earningsSoon !== null ? 2 : 0) +
        (item.news24h > 0 ? 1 : 0) +
        (item.scoreDelta !== undefined && Math.abs(item.scoreDelta) >= 5 ? 1 : 0);
      return { item, earningsSoon, weight, order };
    });
    decorated.sort((a, b) => b.weight - a.weight || a.order - b.order);
    return decorated;
  }, [items, upcoming]);

  if (authLoading || !loggedIn) return null;

  const heldCount = items.filter((i) => i.held).length;
  const watchCount = items.filter((i) => i.watched && !i.held).length;
  const shown = rows.slice(0, MY_STOCKS_ROWS);
  const rest = rows.length - shown.length;

  return (
    <SectionCard
      title={
        <span className="flex items-baseline gap-2">
          내 종목
          {items.length > 0 && (
            <span className="text-micro font-normal text-muted-foreground">
              {[heldCount > 0 && `보유 ${heldCount}`, watchCount > 0 && `관심 ${watchCount}`]
                .filter(Boolean)
                .join(" · ")}
            </span>
          )}
        </span>
      }
      href={items.length > 0 ? "/changes" : undefined}
      hrefLabel="변화 모두 보기"
      footer={
        items.length > 0 ? (
          <span className="flex w-full items-center justify-between gap-2">
            <span>
              {data?.scoreDate ? `AI 점수 ${data.scoreDate.slice(5).replace("-", "/")} 기준 · ` : ""}
              뉴스는 최근 24시간
            </span>
            <Link href="/watchlist" className="shrink-0 hover:text-foreground">
              {rest > 0 ? `외 ${rest}종목 · ` : ""}관심 종목 관리 ›
            </Link>
          </span>
        ) : undefined
      }
    >
      {data === null && error ? (
        <LoadErrorState compact message={error} onRetry={() => void load()} />
      ) : data === null ? (
        <SectionSkeleton rows={4} />
      ) : items.length === 0 ? (
        <MyStocksEmpty />
      ) : (
        <ul className="@container grid min-w-0 gap-x-6 lg:grid-cols-2">
          {shown.map(({ item, earningsSoon }) => (
            <MyStockRow
              key={item.symbol}
              item={item}
              earningsSoon={earningsSoon}
              quote={live.get(item.symbol.toUpperCase())}
              onPick={onPick}
            />
          ))}
        </ul>
      )}
    </SectionCard>
  );
}

function MyStockRow({
  item,
  earningsSoon,
  quote,
  onPick,
}: {
  item: MyTodayItem;
  earningsSoon: number | null;
  quote: LiveQuote | undefined;
  onPick: (symbol: string) => void;
}) {
  const tone = item.score !== undefined ? scoreTone(item.score) : null;
  const currency = (quote?.currency ?? (item.assetType === "domestic" ? "KRW" : "USD")) as
    | "KRW"
    | "USD";
  return (
    <li className="min-w-0 overflow-hidden">
      {/*
        세로 배치를 flex가 아니라 **한 칸짜리 grid(minmax(0,1fr))** 로 둔다 (2026-09-27 제보 2).
        WebKit(iOS Safari)은 <button>을 flex 컨테이너로 쓸 때 자식의 min-width:0을 믿을 수 없어,
        뉴스 제목 줄이 말줄임 없이 카드 밖으로 밀려났다. grid 트랙은 엔진과 무관하게 폭을 가둔다.
      */}
      <button
        type="button"
        onClick={() => onPick(item.symbol)}
        className="grid w-full min-w-0 grid-cols-[minmax(0,1fr)] gap-0.5 overflow-hidden rounded-md px-1 py-1.5 text-left text-cell hover:bg-accent"
      >
        <span className="flex w-full min-w-0 items-center gap-2">
          <StockAvatar
            symbol={item.symbol}
            name={item.name}
            currency={currency}
            assetType={item.assetType as "domestic" | "us" | "etf" | undefined}
            size="xs"
            className="shrink-0"
          />
          <span className="min-w-0 flex-1 truncate font-medium">
            {item.name || item.symbol}
            {item.held && (
              <span className="ml-1.5 rounded bg-[var(--brand)]/10 px-1 text-micro font-normal text-[var(--brand)]">
                보유
              </span>
            )}
          </span>
          <QuoteCell
            price={quote?.livePrice ?? 0}
            changeRate={quote?.liveChangeRate ?? 0}
            currency={currency}
            symbol={item.symbol}
          />
          <span
            className="w-16 shrink-0 text-right font-num tabular-nums"
            title="AI 점수 · 전 점수일 대비"
          >
            {item.score !== undefined && tone ? (
              <>
                <span className={cn("font-bold", SCORE_TONE_TEXT[tone])}>{item.score}</span>
                <ScoreDelta delta={item.scoreDelta} />
              </>
            ) : (
              <span className="text-muted-foreground">—</span>
            )}
          </span>
        </span>
        <Signals item={item} earningsSoon={earningsSoon} />
      </button>
    </li>
  );
}

function ScoreDelta({ delta }: { delta?: number }) {
  if (delta === undefined || delta === 0) return null;
  return (
    <span
      className={cn(
        "ml-1 text-micro",
        delta > 0 ? "text-[var(--profit)]" : "text-[var(--loss)]",
      )}
    >
      {delta > 0 ? `▲${delta}` : `▼${-delta}`}
    </span>
  );
}

/**
 * 줄 아래 사건들. 없으면 줄을 그리지 않는다 — 「특이사항 없음」을 여덟 번 적는 것은
 * 안내가 아니라 잡음이다.
 */
function Signals({ item, earningsSoon }: { item: MyTodayItem; earningsSoon: number | null }) {
  const hasAny = item.change || earningsSoon !== null || item.news24h > 0;
  if (!hasAny) return null;
  return (
    <span className="flex w-full min-w-0 max-w-full items-center gap-1.5 overflow-hidden pl-7 text-micro text-muted-foreground">
      {earningsSoon !== null && (
        <span className="shrink-0 rounded bg-amber-500/15 px-1 text-amber-700 dark:text-amber-400">
          실적 {earningsSoon === 0 ? "오늘" : `D-${earningsSoon}`}
        </span>
      )}
      {item.news24h > 0 && <span className="shrink-0">뉴스 {item.news24h}</span>}
      {item.change && (
        <span
          className={cn(
            "block min-w-0 flex-1 truncate",
            item.change.direction > 0
              ? "text-[var(--profit)]"
              : item.change.direction < 0
                ? "text-[var(--loss)]"
                : undefined,
          )}
        >
          {item.change.title}
          {item.change.unexplained && (
            <span className="ml-1 rounded bg-amber-500/15 px-1 text-amber-700 dark:text-amber-400">
              설명 불가
            </span>
          )}
        </span>
      )}
    </span>
  );
}

/**
 * 담은 종목이 없을 때. 가입만 하고 떠나는 사람이 가장 많이 보는 자리라, 「무엇을 해
 * 주는가」를 먼저 적고 버튼은 둘만 둔다(고르기 · 보유 등록).
 */
function MyStocksEmpty() {
  return (
    <div className="flex flex-col gap-3 px-1 py-2 sm:flex-row sm:items-center sm:justify-between">
      <p className="text-cell text-muted-foreground">
        관심 종목을 담으면 여기서 매일{" "}
        <b className="text-[var(--text-primary)]">등락 · AI 점수 변화 · 실적 일정 · 새 뉴스</b>를
        한눈에 모아 드립니다.
      </p>
      <span className="flex shrink-0 gap-2">
        <Link
          href="/welcome"
          className="rounded-md bg-[var(--brand)] px-3 py-1.5 text-cell font-semibold text-[var(--brand-foreground)] hover:opacity-90"
        >
          관심 종목 고르기
        </Link>
        <Link
          href="/holdings"
          className="rounded-md border border-[var(--border)] px-3 py-1.5 text-cell hover:bg-accent"
        >
          보유 종목 등록
        </Link>
      </span>
    </div>
  );
}
