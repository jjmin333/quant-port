"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { ChevronDown, TrendingDown, TrendingUp } from "lucide-react";
import { Card } from "@/components/ui/card";
import { fetchStockDailyCloses } from "@/lib/daily-close-api";
import type { DailyCloseRow, DailyCloseSeries } from "@/lib/daily-close-types";
import { formatPercent } from "@/lib/format";
import { trendTextClass } from "@/lib/trend-colors";
import { SCORE_TONE_TEXT, scoreTone } from "@/lib/stock-score-api";
import { cn } from "@/lib/utils";
import { toUserMessage } from "@/lib/error-message";
import { PriceDisplay } from "@/components/ui/price-display";
import { PageNav } from "@/components/ui/page-nav";

type DailyCloseTabProps = {
  symbol: string;
  currency: "KRW" | "USD";
  compact?: boolean;
};

/** `'26년 8월 6일 (목)` — matches the dated-list styling used elsewhere. */
function formatTradeDate(tradeDate: string, weekday: string): { head: string; sub: string } {
  const [year, month, day] = tradeDate.split("-");
  if (!year || !month || !day) return { head: tradeDate, sub: weekday };
  return {
    head: `'${year.slice(2)}년 ${Number(month)}월`,
    sub: `${Number(day)}일${weekday ? ` (${weekday})` : ""}`,
  };
}

/*
날짜별 점수는 **종합 점수와 같은 값·같은 색**이다 (2026-09-03 §6 통일).

예전에는 이 목록만 자체 신호등(70/50/30)을 쓰고 값 앞에 `+`까지 붙였다. 위쪽
「종합 점수」 카드는 `scoreTone`(65/35)을 쓰는 0~100 점수라, 같은 날 같은 종목을
두고 **카드는 62, 이 목록은 +71** 같은 일이 생겼다 — 서버가 두 가지 점수를
같은 칸에 쓰고 있었기 때문이다(BE `daily_close_score.go` 참조). 이제 두 자리 모두
`stock_scores`의 한 값을 보므로, 화면 규칙도 하나로 맞춘다.
*/

/*
 * `compact`는 이 행에서 크기를 바꾸지 않는다 — 예전엔 `compact ? "text-[11px]" : "text-[11px]"`
 * 처럼 양쪽이 같은 값인 삼항이 세 군데 있었다. v1.0.0에서 계단을 정리하며 그 죽은 분기를
 * 걷어냈고, 좁은 칸이라고 일별 시세 행을 더 줄일 이유도 없다(이미 하한이다).
 * 프로퍼티는 호출부 시그니처 때문에 남긴다.
 */
function DailyCloseItem({
  row,
  currency,
}: {
  row: DailyCloseRow;
  currency: "KRW" | "USD";
  compact?: boolean;
}) {
  const [open, setOpen] = useState(false);
  const { head, sub } = formatTradeDate(row.tradeDate, row.weekday);
  const rate = row.changeRate;
  const hasSummary = row.aiSummary.trim() !== "";

  const body = (
    <>
      <div className="min-w-0 text-left">
        <p className="text-cell text-[var(--text-muted)]">{head}</p>
        <p className="text-note font-medium text-[var(--text-primary)]">{sub}</p>
      </div>

      <div className="text-right">
        {rate == null ? (
          <p className="text-note text-[var(--text-muted)]">—</p>
        ) : (
          <p
            className={cn(
              "flex items-center justify-end gap-1 text-note font-medium tabular-nums",
              trendTextClass(rate),
            )}
          >
            {rate >= 0 ? (
              <TrendingUp className="h-3.5 w-3.5 shrink-0" aria-hidden />
            ) : (
              <TrendingDown className="h-3.5 w-3.5 shrink-0" aria-hidden />
            )}
            {formatPercent(rate)}
          </p>
        )}
        <p className="text-cell tabular-nums text-[var(--text-muted)]">
          <PriceDisplay value={row.close} currency={currency} size="inherit" />
        </p>
      </div>

      <div className="w-12 text-right sm:w-16">
        {/* A missing score prints "-" — never 0, which would read as a real
            (and very bearish) reading. */}
        {row.score == null ? (
          <span className="text-section font-semibold text-[var(--text-muted)]">-</span>
        ) : (
          <span
            className={cn(
              "text-section font-semibold tabular-nums",
              SCORE_TONE_TEXT[scoreTone(row.score)],
            )}
          >
            {row.score}
          </span>
        )}
      </div>
    </>
  );

  if (!hasSummary) {
    return (
      <div className="grid grid-cols-[minmax(0,1fr)_auto_auto] items-center gap-3 border-b border-[var(--border)]/70 px-3 py-2.5 last:border-b-0">
        {body}
      </div>
    );
  }

  return (
    <div className="border-b border-[var(--border)]/70 last:border-b-0">
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        aria-expanded={open}
        className="grid w-full grid-cols-[minmax(0,1fr)_auto_auto] items-center gap-3 px-3 py-2.5 text-left transition-colors hover:bg-[var(--surface-elevated)]/50"
      >
        {body}
        <ChevronDown
          className={cn(
            "h-3.5 w-3.5 shrink-0 text-[var(--text-muted)] transition-transform",
            open && "rotate-180",
          )}
          aria-hidden
        />
      </button>
      {open && (
        <div className="px-3 pb-3">
          <p className="rounded-lg bg-[var(--surface-elevated)]/60 px-3 py-2 text-read text-[var(--text-secondary)]">
            {row.aiSummary}
          </p>
          {row.analysisSource === "rule" && (
            <p className="mt-1 text-micro text-[var(--text-muted)]">
              규칙 기반 요약 (LLM 미사용)
            </p>
          )}
        </div>
      )}
    </div>
  );
}

/**
 * 한 페이지 = **30행**(거래일 30일 — 주말을 빼면 약 6주).
 *
 * **달 단위로 끊지 않는다.** 달마다 거래일이 17~23일로 달라 페이지 길이가
 * 들쭉날쭉하고, 무엇보다 월초에 열면 첫 화면에 한두 줄만 남는다(9월 2일에 열면
 * 9월 페이지에 1행). 행 수로 고정하면 모든 페이지가 같은 분량이다.
 *
 * 번호 페이지네이션은 종목 뉴스 탭과 **같은 컴포넌트**를 쓴다(`PageNav`).
 *
 * **추가 요청은 없다.** 한 번에 받아 둔 120행을 자르기만 한다(서버 상한이 120).
 */
const PAGE_SIZE = 30;

type DayPage = { rows: DailyCloseRow[]; rangeLabel: string };

/** "26.07.21 ~ 26.09.02" — 이 페이지가 덮는 구간. 행은 최신순이다. */
function rangeLabelOf(rows: DailyCloseRow[]): string {
  if (rows.length === 0) return "";
  const short = (d: string) => d.slice(2).replace(/-/g, ".");
  const newest = short(rows[0].tradeDate);
  const oldest = short(rows[rows.length - 1].tradeDate);
  return newest === oldest ? newest : `${oldest} ~ ${newest}`;
}

function paginate(rows: DailyCloseRow[]): DayPage[] {
  const pages: DayPage[] = [];
  for (let i = 0; i < rows.length; i += PAGE_SIZE) {
    const slice = rows.slice(i, i + PAGE_SIZE);
    pages.push({ rows: slice, rangeLabel: rangeLabelOf(slice) });
  }
  return pages;
}

export function DailyCloseTab({
  symbol,
  currency,
  compact = false,
}: DailyCloseTabProps) {
  const [series, setSeries] = useState<DailyCloseSeries | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [pageIndex, setPageIndex] = useState(0);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      setSeries(await fetchStockDailyCloses(symbol, 120));
      // 종목이 바뀌면 첫 페이지(최근 30일)로 돌아온다 — 앞 종목에서 2페이지를
      // 보고 있었다고 다음 종목도 거기서 시작할 이유는 없다.
      setPageIndex(0);
    } catch (e) {
      setSeries(null);
      setError(toUserMessage(e, "일별 시세를 불러오지 못했습니다."));
    } finally {
      setLoading(false);
    }
  }, [symbol]);

  useEffect(() => {
    void load();
  }, [load]);

  const rows = useMemo(() => series?.rows ?? [], [series]);
  const pages = useMemo(() => paginate(rows), [rows]);
  const seriesCurrency = series?.currency ?? currency;

  // 데이터가 줄어 인덱스가 범위를 벗어나면 마지막 페이지로 붙인다(렌더 중 보정 —
  // 이펙트로 되돌리면 빈 화면이 한 프레임 지나간다).
  const safeIndex =
    pages.length === 0 ? 0 : Math.min(pageIndex, pages.length - 1);
  const page = pages[safeIndex] ?? null;
  const pageRows = page?.rows ?? [];
  const scored = pageRows.filter((r) => r.score != null).length;

  /*
    「마감했는데 왜 점수가 비어 있나」에 화면이 먼저 답한다 (2026-09-11 사용자 요청 3).

    점수는 종가와 **같은 시각에 생기지 않는다.** 종가는 마감 +20분에 적재되지만
    (`daily_close_*`), 점수는 하루 한 번 도는 `stock_scores`(매일 00:25 KST)가
    전 종목을 다시 계산해 이 행에 미러링한다. 그래서 15:30에 마감한 종목의 점수는
    그날 밤 자정을 넘겨서 채워진다 — 비어 있는 게 고장이 아니다.

    각주를 항상 적고, **가장 최근 줄이 실제로 비어 있을 때만** 색을 올린다. 언제나
    강조하면 정상인 날에도 경고처럼 읽히고, 아예 안 적으면 지금 이 질문이 남는다.
  */
  const latestUnscored = rows.length > 0 && rows[0].score == null;

  // 페이지를 넘기면 목록 맨 위로 돌려놓는다 — 한 페이지가 30줄이라 페이지 번호는
  // 목록 아래에 있고, 그대로 두면 새 페이지의 끝부터 보게 된다.
  const cardRef = useRef<HTMLDivElement>(null);
  const goToPage = (oneBased: number) => {
    setPageIndex(oneBased - 1);
    cardRef.current?.scrollIntoView({ block: "start", behavior: "smooth" });
  };

  return (
    // Card는 ref를 넘겨받지 않으므로 스크롤 기준점만 바깥 div가 잡는다.
    <div ref={cardRef} className="scroll-mt-4">
      <Card className="p-0">
        <div className="flex flex-wrap items-center justify-between gap-2 border-b border-[var(--border)] px-3 py-2.5">
          <div className="min-w-0">
            <h2 className="text-section font-semibold text-[var(--text-primary)]">
              일별 시세 · 종합 점수
            </h2>
            <p className="text-micro text-[var(--text-muted)]">
              정규장 종가 기준 · 점수 {scored}/{pageRows.length}일
              {page?.rangeLabel && (
                <span className="ml-1 tabular-nums">· {page.rangeLabel}</span>
              )}
            </p>
            <p
              className={cn(
                "mt-0.5 text-micro",
                latestUnscored ? "text-[var(--brand)]" : "text-[var(--text-muted)]",
              )}
            >
              종합 점수는 매일 새벽 00:25(KST)에 한 번 집계됩니다 — 당일 종가 점수는 그날
              밤 채워집니다.
            </p>
          </div>
        </div>

        <div className="grid grid-cols-[minmax(0,1fr)_auto_auto] gap-3 border-b border-[var(--border)] bg-[var(--surface-elevated)]/40 px-3 py-1.5 text-micro font-medium text-[var(--text-muted)]">
          <span>날짜</span>
          <span className="text-right">실제주가</span>
          <span className="w-12 text-right sm:w-16">종합 점수</span>
        </div>

        {loading ? (
          <p className="px-3 py-8 text-center text-cell text-[var(--text-muted)]">
            일별 시세 로딩 중…
          </p>
        ) : error ? (
          <p className="px-3 py-8 text-center text-cell text-[var(--text-muted)]">
            {error}
          </p>
        ) : !page ? (
          <p className="px-3 py-8 text-center text-cell text-[var(--text-muted)]">
            아직 적재된 일별 종가가 없습니다. 정규장 종가 이후 자동으로
            쌓입니다.
          </p>
        ) : (
          <>
            <div>
              {pageRows.map((row) => (
                <DailyCloseItem
                  key={row.tradeDate}
                  row={row}
                  currency={seriesCurrency}
                  compact={compact}
                />
              ))}
            </div>
            {/* 뉴스 탭과 같은 번호 페이지네이션 — 같은 컴포넌트를 쓴다. */}
            <PageNav
              page={safeIndex + 1}
              totalPages={pages.length}
              onChange={goToPage}
              label="일별 시세 페이지"
              className="px-3 pb-3 pt-3"
            />
          </>
        )}
      </Card>
    </div>
  );
}
