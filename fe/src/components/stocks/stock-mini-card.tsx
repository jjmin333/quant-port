"use client";

import { useMemo, useState } from "react";
import { PREDICTION_FEATURES_HIDDEN } from "@/lib/hidden-features";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { ArrowRight, Heart, NotepadText, Stamp } from "lucide-react";
import { PriceHistoryChart } from "@/components/charts/price-history-chart";
import { NewsGatedActionBar, type GatedAction } from "@/components/news/news-login-gate";
import { useAuth } from "@/components/providers/auth-provider";
import { StockAvatar } from "@/components/stocks/stock-avatar";
import { StockScoreChip } from "@/components/stocks/stock-score-chip";
import { useSparklineCandles } from "@/hooks/use-sparkline-candles";
import { useStockScore } from "@/hooks/use-stock-score";
import type { UpcomingEvent } from "@/hooks/use-upcoming-events";
import { analysisSymbolHref } from "@/lib/analysis-tabs";
import { formatPercent, formatPrice } from "@/lib/format";
import { addWatchlistClient } from "@/lib/market-api";
import type { QuoteSummary } from "@/lib/quotes-types";
import type { StockScoreData } from "@/lib/stock-score-api";
import { trendTextClass } from "@/lib/trend-colors";
import { cn } from "@/lib/utils";
import { PriceDisplay } from "@/components/ui/price-display";

/**
 * 종목 미니 카드 — 모달·패널 안에서 "이 종목 지금 어떤가"에 답하는 **한 가지** 모양.
 *
 * 뉴스 기사 모달의 관련 종목, 그리고 앞으로 종목 카드를 늘어놓는 자리는 전부 이걸
 * 쓴다. 같은 종목이 화면마다 다르게 생기면 읽는 법을 매번 다시 배워야 한다.
 *
 * 두 가지가 예전과 다르다 —
 *
 *  1. **액션이 카드 안에 있다.** 관심 추가·메모·스탬프는 예전에 모달 맨 아래 한 줄로
 *     있었고, 종목이 여러 개여도 **첫 번째 종목에만** 적용됐다. 아마존·애플 기사에서
 *     「관심 추가」를 누르면 애플만 담기는 셈이라 어느 종목에 대한 버튼인지 알 수
 *     없었다. 이제 버튼은 자기 카드의 종목에만 작용한다.
 *  2. **점수는 이름표를 달고 나온다**(`StockScoreChip`).
 *
 * 종목 이름을 누르면 **개요 탭**으로 간다(`analysisSymbolHref(symbol)`) —
 * 목록에서 종목을 눌러 처음 궁금한 건 호가창이 아니라 "이 종목이 지금 어떤가"다.
 * 예전에는 AI 분석 탭으로 보냈는데, 그 탭의 본문(LLM 글)은 일부 종목에만 있어서
 * 밖에서 들어온 사람이 빈 화면을 먼저 만났다. 개요는 거의 모든 종목에 내용이 있다
 * (2026-09-11 사용자 요청 1).
 */
export function StockMiniCard({
  symbol,
  quote,
  score: scoreProp,
  events,
  articleId,
  showActions = true,
  showChart = true,
  onNavigate,
  className,
}: {
  symbol: string;
  quote: QuoteSummary | undefined;
  /** 배치로 이미 받아 둔 점수. 주지 않으면 카드가 직접 부른다. */
  score?: StockScoreData | null;
  /** 이 종목의 예정된 실적·공시. 호출부가 여러 종목을 한 번에 받아 나눠 준다. */
  events?: UpcomingEvent[];
  /** 기사에서 열렸을 때 — 로그인 후 이 기사로 돌아온다. */
  articleId?: string;
  showActions?: boolean;
  showChart?: boolean;
  onNavigate?: () => void;
  className?: string;
}) {
  const { session } = useAuth();
  const router = useRouter();
  const [added, setAdded] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // 점수를 넘겨받았으면 다시 부르지 않는다 — 카드가 여러 장 뜨는 화면에서
  // 카드마다 요청하면 종목 수만큼 요청이 나간다.
  const { data: fetched } = useStockScore(scoreProp === undefined ? symbol : "");
  const score = scoreProp === undefined ? fetched : scoreProp;

  const { points, times } = useSparklineCandles(symbol, showChart);

  const name = quote?.name || symbol;
  const hasPrice = Boolean(quote && quote.currentPrice > 0);

  const actions = useMemo<GatedAction[]>(() => {
    const priceLine =
      quote && quote.currentPrice > 0
        ? `${name} ${formatPrice(quote.currentPrice, quote.currency)}`
        : name;
    const all: GatedAction[] = [
      {
        key: "watchlist",
        label: added ? "관심 추가됨" : "관심 추가",
        icon: <Heart className="size-3.5" />,
        title: `⭐ ${name} 관심 목록에 담기`,
        description: "5초마다 갱신되는 내 목록에 올려두고, 담은 뒤 수익률까지 함께 봅니다.",
        bullets: [priceLine, "담은 날짜와 그때의 가격"],
      },
      {
        key: "journal",
        label: "메모",
        icon: <NotepadText className="size-3.5" />,
        title: `🖋️ ${name}에 붙는 내 기록`,
        description: "왜 지금 이 종목이 눈에 들어왔는지 적어두고, 나중에 다시 읽습니다.",
        bullets: [priceLine, "오늘 날짜"],
      },
      {
        key: "stamp",
        label: "스탬프",
        icon: <Stamp className="size-3.5" />,
        title: `🔖 ${name} 스탬프 찍기`,
        description:
          "지금 이 순간의 가격·점수를 그대로 봉인해 두고, 나중에 내 판단이 맞았는지 확인합니다.",
        bullets: [priceLine, "오늘 날짜"],
      },
    ];
    // 목표가 예측(스탬프) 비노출 — lib/hidden-features (2026-10-10).
    return PREDICTION_FEATURES_HIDDEN ? all.filter((a) => a.key !== "stamp") : all;
  }, [name, quote, added]);

  const handleAction = (key: string) => {
    setError(null);
    if (key === "watchlist") {
      addWatchlistClient(symbol)
        .then(() => setAdded(true))
        .catch(() => setError("관심 목록에 담지 못했습니다."));
      return;
    }
    onNavigate?.();
    if (key === "journal") {
      // 메모 화면은 `?symbol=`을 읽지 않는다(목표가 예측만 읽는다) — 없는 파라미터를
      // 붙이면 다음 사람이 동작한다고 오해한다.
      router.push("/journal");
      return;
    }
    router.push(`/predictions?symbol=${encodeURIComponent(symbol)}`);
  };

  return (
    <div
      className={cn(
        "min-w-0 rounded-xl border border-[var(--border)] bg-[var(--surface)] p-2.5",
        className,
      )}
    >
      <div className="flex items-start justify-between gap-2">
        <Link
          href={analysisSymbolHref(symbol)}
          onClick={onNavigate}
          className="group flex min-w-0 items-center gap-1.5 text-cell font-semibold text-[var(--text-primary)]"
        >
          <StockAvatar
            symbol={symbol}
            name={quote?.name}
            currency={quote?.currency}
            assetType={quote?.assetType}
            market={quote?.market}
            logoUrl={quote?.logoUrl}
            size="xs"
            className="shrink-0"
          />
          <span className="truncate">{name}</span>
          <span className="shrink-0 font-num text-micro font-normal text-[var(--text-muted)]">
            {symbol}
          </span>
          <ArrowRight className="size-3 shrink-0 text-[var(--text-muted)] group-hover:text-[var(--brand)]" />
        </Link>
        <StockScoreChip
          symbol={symbol}
          score={score?.score}
          grade={score?.grade}
          tradeDate={score?.tradeDate}
          onNavigate={onNavigate}
        />
      </div>

      {showActions && (
        <>
          <NewsGatedActionBar
            actions={actions}
            signedIn={Boolean(session)}
            articleId={articleId}
            onAction={handleAction}
            placement="down"
            className="mt-1.5"
          />
          {error && <p className="mt-1 text-cell text-[var(--loss)]">{error}</p>}
          {added && (
            <p className="mt-1 text-cell text-[var(--brand)]">관심 목록에 담았습니다.</p>
          )}
        </>
      )}

      {hasPrice ? (
        <div className="mt-1.5 flex items-baseline gap-1.5">
          <span className="font-num text-lg font-bold tabular-nums text-[var(--text-primary)]">
            <PriceDisplay value={quote!.currentPrice} currency={quote!.currency} size="inherit" />
          </span>
          <span
            className={cn(
              "font-num text-cell font-semibold tabular-nums",
              trendTextClass(quote!.changeRate),
            )}
          >
            {formatPercent(quote!.changeRate)}
          </span>
        </div>
      ) : (
        <p className="mt-1.5 text-cell text-[var(--text-muted)]">시세를 불러오지 못했습니다</p>
      )}

      {showChart && points.length >= 2 && (
        <PriceHistoryChart
          points={points}
          times={times}
          currency={quote?.currency ?? "KRW"}
          title="최근 흐름"
          height={80}
          className="mt-1.5"
        />
      )}

      {events && events.length > 0 && (
        <div className="mt-2 border-t border-[var(--border)]/60 pt-1.5">
          <p className="mb-0.5 text-cell font-semibold text-[var(--text-muted)]">다가오는 일정</p>
          <ul className="min-w-0">
            {events.slice(0, 3).map((event, i) => (
              <li
                key={`${event.date}-${i}`}
                className="flex items-center gap-2 py-0.5 text-cell"
              >
                <span className="w-12 shrink-0 font-num tabular-nums text-[var(--text-muted)]">
                  {event.date.slice(5)}
                </span>
                <span className="min-w-0 flex-1 truncate">{event.label || "실적 발표"}</span>
              </li>
            ))}
          </ul>
        </div>
      )}
    </div>
  );
}
