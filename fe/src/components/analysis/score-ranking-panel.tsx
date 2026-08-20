"use client";

import { useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { ArrowDown, ArrowUp, Medal, Star } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Card } from "@/components/ui/card";
import { StockAvatar } from "@/components/stocks/stock-avatar";
import { MarketRadarSparkline } from "@/components/market-radar/market-radar-sparkline";
import { useScoreRankings, type EnrichedScoreItem } from "@/hooks/use-score-rankings";
import { analysisSymbolHref } from "@/lib/analysis-tabs";
import { fetchQuotesBatchClient } from "@/lib/market-api";
import type { QuoteSummary } from "@/lib/quotes-types";
import { formatPercent } from "@/lib/format";
import { SCORE_NEUTRAL_TEXT, type ScoreRankingMarket } from "@/lib/stock-score-api";
import { setRankingView, useRankingView, type RankingView } from "@/lib/ranking-view";
import { trendTextClass } from "@/lib/trend-colors";
import { cn } from "@/lib/utils";

const MARKET_TABS: { id: ScoreRankingMarket; label: string }[] = [
  { id: "domestic", label: "국내" },
  { id: "overseas", label: "해외" },
];

const MEDAL_CLASS: Record<1 | 2 | 3, string> = {
  1: "text-[#FFB300]",
  2: "text-[#9CA3AF]",
  3: "text-[#B36A3C]",
};

const MEDAL_LABEL: Record<1 | 2 | 3, string> = {
  1: "금메달",
  2: "은메달",
  3: "동메달",
};

type SortKey = "score" | "change" | "name";

/**
 * 등락 강도 배경 (옵션 6-C).
 *
 * 색은 기존 규칙 그대로 빨강(상승)·파랑(하락)을 쓰고, **농도로 크기**를 말한다.
 * ±2% 미만은 거의 칠하지 않아 평범한 날에 화면이 알록달록해지지 않게 한다.
 * 글자색이 아니라 배경만 건드리므로 숫자 자체의 가독성은 그대로다.
 */
function heatStyle(rate: number | undefined): React.CSSProperties | undefined {
  if (typeof rate !== "number" || rate === 0) return undefined;
  const abs = Math.abs(rate);
  const alpha = abs >= 5 ? "22%" : abs >= 2 ? "13%" : "6%";
  const base = rate > 0 ? "var(--profit)" : "var(--loss)";
  return { backgroundColor: `color-mix(in oklab, ${base} ${alpha}, transparent)` };
}

type ScoreRankingPanelProps = {
  activeSymbol?: string | null;
  className?: string;
};

/**
 * 오늘(배치 미실행 시 최근일) 종합 점수 기준 국내/해외 종목 랭킹.
 * 종목 분석 화면에서 검색 없이 상위 점수 종목을 둘러보고 바로 넘어가는 진입점.
 */
export function ScoreRankingPanel({ activeSymbol, className }: ScoreRankingPanelProps) {
  const [market, setMarket] = useState<ScoreRankingMarket>("domestic");
  const [sort, setSort] = useState<SortKey>("score");
  const [desc, setDesc] = useState(true);
  const { items, tradeDate, loading, error } = useScoreRankings(market, 20);

  // 보기 밀도는 `lib/ranking-view`가 쥔다 — 페이지(`PageColumns`)가 같은 값을 보고
  // 레일 폭을 넓히기 때문이다.
  const view = useRankingView();
  const quotes = useRankingQuotes(items, view === "wide");

  function toggleSort(key: SortKey) {
    if (sort === key) {
      setDesc((v) => !v);
      return;
    }
    setSort(key);
    // 이름은 가나다순이 자연스럽고, 숫자는 큰 값이 위가 자연스럽다.
    setDesc(key !== "name");
  }

  /*
   * 점수 랭킹 API는 순위·점수·등급만 준다 — 국내 종목은 `changeRate`·`closePrice`가
   * 아예 없다(실측). 그래서 넓게 보기의 등락 칸이 전부 `—`가 되고 등락 정렬도 무의미해진다.
   * 배치 시세로 그 두 값을 채운다. **넓게 보기일 때만** 부른다 — 간단 보기는 등락을
   * 쓰지 않으므로 요청할 이유가 없다.
   */
  const merged = useMemo(
    () =>
      items.map((item) => {
        const q = quotes[item.symbol.toUpperCase()];
        if (!q) return item;
        return {
          ...item,
          changeRate: q.changeRate,
          closePrice: q.currentPrice > 0 ? q.currentPrice : item.closePrice,
          currency: item.currency ?? q.currency,
        };
      }),
    [items, quotes],
  );

  const sorted = useMemo(() => {
    const rows = [...merged];
    const dir = desc ? -1 : 1;
    rows.sort((a, b) => {
      if (sort === "name") return dir * a.name.localeCompare(b.name, "ko");
      if (sort === "change") {
        // 등락률이 없는 종목은 정렬 방향과 무관하게 항상 아래로 보낸다.
        const av = a.changeRate;
        const bv = b.changeRate;
        if (typeof av !== "number" && typeof bv !== "number") return a.rank - b.rank;
        if (typeof av !== "number") return 1;
        if (typeof bv !== "number") return -1;
        return dir * (av - bv);
      }
      return dir * (a.score - b.score);
    });
    return rows;
  }, [merged, sort, desc]);

  return (
    // 레일에 들어가면 남는 높이를 다 쓰고, 좁은 화면에서 본문 위로 쌓이면 420px에서 멈춘다.
    <Card className={cn("flex min-h-0 flex-col p-0", className)}>
      <div className="flex flex-wrap items-center justify-between gap-2 border-b px-3 py-2.5">
        <h2 className="text-sm font-semibold">종목 랭킹</h2>
        <div className="flex items-center gap-1.5">
          <SegTabs
            options={MARKET_TABS}
            value={market}
            onChange={(id) => setMarket(id as ScoreRankingMarket)}
            label="시장 선택"
          />
          <SegTabs
            options={[
              { id: "compact", label: "간단" },
              { id: "wide", label: "넓게" },
            ]}
            value={view}
            onChange={(id) => setRankingView(id as RankingView)}
            label="보기 밀도"
          />
        </div>
      </div>

      {tradeDate && (
        <p className="border-b px-3 py-1 text-[10px] text-muted-foreground">
          {tradeDate} 기준 점수
        </p>
      )}

      {/* 정렬 헤더 (옵션 6-C) */}
      <div className="flex h-[26px] items-center gap-2 border-b px-3 text-[11px] text-muted-foreground">
        <span className="w-7 shrink-0" aria-hidden />
        <SortButton label="종목" active={sort === "name"} desc={desc} onClick={() => toggleSort("name")} className="flex-1 text-left" />
        {view === "wide" && (
          <SortButton
            label="등락"
            active={sort === "change"}
            desc={desc}
            onClick={() => toggleSort("change")}
            className="w-[62px] justify-end text-right"
          />
        )}
        <SortButton
          label="점수"
          active={sort === "score"}
          desc={desc}
          onClick={() => toggleSort("score")}
          className="w-10 justify-end text-right"
        />
      </div>

      <div className="min-h-0 flex-1 overflow-y-auto max-xl:max-h-[420px]">
        {loading && items.length === 0 ? (
          <p className="p-4 text-center text-xs text-muted-foreground">랭킹 로딩 중…</p>
        ) : error ? (
          <p className="p-4 text-center text-xs text-muted-foreground">{error}</p>
        ) : sorted.length === 0 ? (
          <p className="p-4 text-center text-xs text-muted-foreground">
            아직 산출된 점수가 없습니다.
          </p>
        ) : (
          sorted.map((item) => (
            <RankingRow
              key={item.symbol}
              item={item}
              view={view}
              /*
               * 메달은 "점수 1·2·3위"라는 뜻이다. 등락순·이름순으로 정렬한 목록에서도
               * 메달을 달면 3번째 줄이 아닌 곳에 금메달이 박혀 순위표처럼 읽힌다.
               * 기본 정렬(점수 내림차순)일 때만 보여준다.
               */
              showMedal={sort === "score" && desc}
              active={item.symbol === activeSymbol}
            />
          ))
        )}
      </div>
    </Card>
  );
}

/** 랭킹 종목의 현재 시세를 심볼로 색인해 돌려준다. `enabled=false`면 아무것도 안 부른다. */
function useRankingQuotes(
  items: EnrichedScoreItem[],
  enabled: boolean,
): Record<string, QuoteSummary> {
  const [bySymbol, setBySymbol] = useState<Record<string, QuoteSummary>>({});
  // 심볼 목록이 실제로 바뀔 때만 다시 부르기 위해 문자열로 고정한다.
  const key = enabled ? items.map((i) => i.symbol).join(",") : "";

  useEffect(() => {
    if (!key) return;
    let alive = true;
    fetchQuotesBatchClient(key.split(","))
      .then((rows) => {
        if (!alive) return;
        setBySymbol(Object.fromEntries(rows.map((q) => [q.symbol.toUpperCase(), q])));
      })
      .catch(() => {
        // 시세가 없어도 점수·등급은 그대로 보여준다.
      });
    return () => {
      alive = false;
    };
  }, [key]);

  return bySymbol;
}

function SegTabs({
  options,
  value,
  onChange,
  label,
}: {
  options: { id: string; label: string }[];
  value: string;
  onChange: (id: string) => void;
  label: string;
}) {
  return (
    <nav className="flex gap-0.5 rounded-lg bg-muted p-0.5" aria-label={label}>
      {options.map((tab) => (
        <button
          key={tab.id}
          type="button"
          onClick={() => onChange(tab.id)}
          aria-pressed={value === tab.id}
          className={cn(
            "rounded-md px-2 py-0.5 text-[11px] font-medium transition-colors",
            value === tab.id
              ? "bg-card text-foreground shadow-sm"
              : "text-muted-foreground hover:text-foreground",
          )}
        >
          {tab.label}
        </button>
      ))}
    </nav>
  );
}

function SortButton({
  label,
  active,
  desc,
  onClick,
  className,
}: {
  label: string;
  active: boolean;
  desc: boolean;
  onClick: () => void;
  className?: string;
}) {
  const Icon = desc ? ArrowDown : ArrowUp;
  return (
    <button
      type="button"
      onClick={onClick}
      // aria-sort는 columnheader 역할에만 유효하다. 여긴 표가 아니라 버튼 줄이라
      // 현재 정렬 상태를 제목으로 읽어준다.
      title={active ? `${label} ${desc ? "내림차순" : "오름차순"}` : `${label}순 정렬`}
      className={cn(
        "flex items-center gap-0.5 hover:text-foreground",
        active && "font-semibold text-foreground",
        className,
      )}
    >
      {label}
      {active && <Icon className="size-3" aria-hidden />}
    </button>
  );
}

function RankingRow({
  item,
  view,
  showMedal,
  active,
}: {
  item: EnrichedScoreItem;
  view: RankingView;
  showMedal: boolean;
  active: boolean;
}) {
  const medalRank = showMedal && item.rank <= 3 ? (item.rank as 1 | 2 | 3) : null;
  const currency = (item.currency ?? "KRW") as "KRW" | "USD";
  const priceText =
    item.closePrice != null
      ? currency === "USD"
        ? `$${item.closePrice.toLocaleString("en-US", { maximumFractionDigits: 2 })}`
        : `${Math.round(item.closePrice).toLocaleString("ko-KR")}원`
      : null;

  return (
    <Link
      href={analysisSymbolHref(item.symbol, "ai")}
      aria-current={active ? "true" : undefined}
      className={cn(
        "flex items-center gap-2 border-b border-border/70 px-3 transition-colors",
        view === "wide" ? "py-2.5" : "py-1.5",
        active ? "bg-[var(--brand-muted)]" : "hover:bg-accent",
      )}
    >
      <div className="flex w-7 shrink-0 items-center justify-center">
        {medalRank ? (
          <Medal
            className={cn("h-4 w-4", MEDAL_CLASS[medalRank])}
            fill="currentColor"
            fillOpacity={0.18}
            aria-label={`${item.rank}위 ${MEDAL_LABEL[medalRank]}`}
          />
        ) : (
          <span className="font-mono text-[12px] font-semibold text-muted-foreground">
            {item.rank}
          </span>
        )}
      </div>

      {view === "wide" && (
        <StockAvatar
          symbol={item.symbol}
          name={item.name}
          currency={currency}
          assetType={item.assetType as "domestic" | "us" | "etf" | undefined}
          size="sm"
          className="shrink-0"
        />
      )}

      <div className="min-w-0 flex-1">
        <div className="flex min-w-0 items-center gap-1">
          <span
            className={cn(
              "truncate font-medium leading-tight",
              view === "wide" ? "text-[13px]" : "text-xs",
            )}
          >
            {item.name}
          </span>
          {item.isHeld && (
            <Badge variant="info" className="shrink-0 px-1 py-0 text-[9px] leading-none">
              보유
            </Badge>
          )}
          {item.isWatchlisted && (
            <Star
              className="h-3 w-3 shrink-0 fill-amber-400 text-amber-400"
              aria-label="관심 종목"
            />
          )}
        </div>
        {/* 넓게 보기에서만 심볼·가격을 덧붙인다. 간단 보기는 종목과 점수만 본다. */}
        {view === "wide" && (
          <p className="truncate text-[11px] leading-tight text-muted-foreground">
            {item.symbol}
            {priceText && ` · ${priceText}`}
          </p>
        )}
      </div>

      {view === "wide" && (
        <>
          <MarketRadarSparkline
            symbol={item.symbol}
            changeRate={item.changeRate ?? 0}
            className="w-[54px] shrink-0"
          />
          <span
            style={heatStyle(item.changeRate)}
            className={cn(
              "w-[62px] shrink-0 rounded px-1 py-0.5 text-right font-mono text-[11px] tabular-nums",
              typeof item.changeRate === "number"
                ? trendTextClass(item.changeRate)
                : "text-muted-foreground",
            )}
          >
            {typeof item.changeRate === "number" ? formatPercent(item.changeRate) : "—"}
          </span>
        </>
      )}

      {/*
        목록의 점수는 신호등을 쓰지 않는다 (08-12 사용자 결정). 순위가 이미 높낮이를
        말하고 있어서, 스무 줄 전부에 색을 입히면 초록 벽이 될 뿐이다.
      */}
      <div className="w-10 shrink-0 text-right leading-tight">
        <span className={cn("font-mono text-[15px] font-bold tabular-nums", SCORE_NEUTRAL_TEXT)}>
          {item.score}
        </span>
        {item.grade && (
          <span className={cn("block text-[10px] font-medium", SCORE_NEUTRAL_TEXT)}>
            {item.grade}
          </span>
        )}
      </div>
    </Link>
  );
}
