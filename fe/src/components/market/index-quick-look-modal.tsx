"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { ArrowRight } from "lucide-react";
import { PriceHistoryChart } from "@/components/charts/price-history-chart";
import { useNewsArticleModal } from "@/components/news/news-article-modal-host";
import { QuickLookSection } from "@/components/stocks/stock-quick-look-modal";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { useIndexScores, type IndexScoreSummary } from "@/hooks/use-index-scores";
import { fetchIndexDailyCloses } from "@/lib/daily-close-api";
import type { DailyCloseRow } from "@/lib/daily-close-types";
import { formatPercent, formatRelativeTime } from "@/lib/format";
import { fetchDomesticNews, fetchUsNews } from "@/lib/news-api";
import {
  articleMatchesDashboardIndex,
  DASHBOARD_INDEX_IDS,
  type DashboardIndexId,
} from "@/lib/news-dashboard-indices";
import type { NewsArticle } from "@/lib/news-types";
import type { MarketIndex } from "@/lib/quotes-types";
import { SCORE_TONE_BORDER, SCORE_TONE_TEXT, scoreTone } from "@/lib/stock-score-api";
import { trendTextClass } from "@/lib/trend-colors";
import { cn } from "@/lib/utils";

/**
 * 지수 퀵룩 — 앱에서 **모달이 없던 유일한 대상**.
 *
 * 지수는 홈에서 가장 많이 눌리지만 가장 설명이 없었다. 카드가 말하는 것은 값과 등락률
 * 둘뿐이고, 누르면 `/market`으로 화면을 떠난다. 정작 서버는 이미 더 많은 것을 주고
 * 있었다 — 추세 점수와 그 근거 문장은 `title` 툴팁에 갇혀 있어 **모바일에서는 존재
 * 자체가 안 보였고**, 60거래일 일별 종가(`/indices/{id}/daily-closes`)는 FE가 한 번도
 * 부른 적이 없다.
 *
 * 표에 **그날의 점수**를 함께 세우는 게 이 모달의 핵심이다. "추세 점수 63"이 좋은
 * 건지는 어제 61, 그제 57을 봐야 알 수 있는데, 그 값은 일별 종가 응답에 **이미 실려
 * 온다**(추가 요청 0). 지금까지는 오늘 점수 하나만 보여 주고 비교 대상은 주지 않았다.
 *
 * 열 때 나가는 요청은 둘이다 — 일별 종가 1, 뉴스 1. 목록에 지수가 열 개여도 연 것 하나뿐.
 */

/**
 * 기간 버튼은 **거래일 수**로 말한다 — 「1개월」이 아니라 「20일」.
 *
 * 실측(08-16): 코스피에 `limit=66`을 줘도 31행만 온다(07-02부터). 지수 일별 종가는
 * 이제 막 쌓이는 중이고 종목마다 깊이가 다르다. 캘린더 기간으로 이름을 붙이면
 * 「3개월」과 「1년」이 **똑같은 차트**를 그리게 되고, 화면이 없는 기간을 있다고 말한다.
 *
 * 그래서 ① 한 번에 최대치를 받아 **클라이언트에서 자르고**(토글할 때마다 요청이 나가지
 * 않는다) ② 앞 버튼보다 실제로 더 많은 봉을 보여주는 버튼만 남긴다. 31행짜리 지수에는
 * 「20일」과 「전체」 둘만 뜨고, 하나뿐이면 토글 자체를 그리지 않는다.
 * 실제 구간은 차트 아래 양옆 날짜가 답한다.
 */
const MAX_BARS = 250;
const BAR_STEPS = [20, 60, MAX_BARS];

function barStepLabel(step: number): string {
  return step >= MAX_BARS ? "전체" : `${step}일`;
}

/**
 * 이 지수 이야기가 실리는 뉴스 목록.
 *
 * 국내·해외 목록은 서로 다른 기사 풀이라, 나스닥을 국내 목록에서 찾으면 몇 건 나오지도
 * 않는다. 금·비트코인은 두 목록 모두에 걸리지만 국내 쪽이 더 촘촘해 국내로 둔다.
 */
const US_INDEX_IDS = new Set<string>(["nasdaq", "nasdaq100", "nq100", "sp500", "dow", "vix", "sox", "us10y"]);

function indexNewsMarket(indexId: string): "domestic" | "us" {
  return US_INDEX_IDS.has(indexId) ? "us" : "domestic";
}

function isDashboardIndexId(id: string): id is DashboardIndexId {
  return (DASHBOARD_INDEX_IDS as readonly string[]).includes(id);
}

function formatIndexValue(value: number): string {
  return value.toLocaleString("ko-KR", { maximumFractionDigits: 2 });
}

export function IndexQuickLookModal({
  index,
  score,
  allIndices,
  open,
  onClose,
}: {
  index: MarketIndex | null;
  score?: IndexScoreSummary;
  /** 「같은 날 다른 지수」 칩. 이미 홈이 받아 둔 목록을 그대로 넘긴다. */
  allIndices: MarketIndex[];
  open: boolean;
  onClose: () => void;
}) {
  const indexId = index?.id ?? "";
  const market = indexNewsMarket(indexId);

  const [bars, setBars] = useState(60);
  const [closes, setCloses] = useState<{ key: string; rows: DailyCloseRow[] } | null>(null);
  const [newsState, setNewsState] = useState<{ key: string; rows: NewsArticle[] } | null>(null);

  useEffect(() => {
    if (!open || !indexId) return;
    let alive = true;
    fetchIndexDailyCloses(indexId, MAX_BARS)
      .then((series) => {
        if (alive) setCloses({ key: indexId, rows: series.rows ?? [] });
      })
      .catch(() => {
        if (alive) setCloses({ key: indexId, rows: [] });
      });
    return () => {
      alive = false;
    };
  }, [open, indexId]);

  useEffect(() => {
    if (!open || !indexId) return;
    let alive = true;
    const load = market === "us" ? fetchUsNews : fetchDomesticNews;
    load({ market, sort: "latest" })
      .then(({ articles }) => {
        if (!alive) return;
        const matched = isDashboardIndexId(indexId)
          ? articles.filter((a) => articleMatchesDashboardIndex(a, indexId))
          : [];
        setNewsState({ key: indexId, rows: matched });
      })
      .catch(() => {
        if (alive) setNewsState({ key: indexId, rows: [] });
      });
    return () => {
      alive = false;
    };
  }, [open, indexId, market]);

  const rows = closes?.key === indexId ? closes.rows : null;
  const news = newsState?.key === indexId ? newsState.rows : null;

  /*
   * 기사는 앱 전체와 같은 방식으로 연다(`useNewsArticleModal`). 이 지수를 다룬 기사라
   * 열면 그 안에 이 지수 카드가 다시 서고, 해외 지수면 번역·원문 셸로 열린다.
   */
  const newsModal = useNewsArticleModal({ pool: news ?? undefined });

  /*
   * 부모가 모달을 닫으면 열려 있던 기사도 접는다. 이펙트가 아니라 렌더 중 조정인 이유는
   * 종목 퀵룩과 같다 — 이펙트로 하면 닫힌 프레임이 한 번 더 그려진다.
   */
  const [wasOpen, setWasOpen] = useState(open);
  if (wasOpen !== open) {
    setWasOpen(open);
    if (!open && newsModal.article) newsModal.close();
  }

  /*
   * 일별 종가는 **최신순**으로 온다(BE가 전일비를 다음 행에서 계산한다). 차트는 왼쪽이
   * 과거여야 하므로 뒤집는다 — 정렬 없이 자르면 가장 오래된 구간을 거꾸로 그리게 된다.
   */
  const chrono = useMemo(
    () => (rows ? [...rows].reverse().filter((r) => r.close > 0) : []),
    [rows],
  );

  /*
   * 원/달러·금·비트코인은 일별 종가 이력이 아직 얕다(오늘부터 배치로 쌓는 중). 비어
   * 있으면 지수 응답이 들고 오는 20포인트 스파크라인으로 대신하고, 기간이 다르다는 것을
   * 제목에 적는다 — 「3개월」이라고 써 놓고 최근 흐름을 그리면 그건 다른 이야기가 된다.
   */
  const usingSparkFallback = rows != null && chrono.length < 2;

  /* 앞 버튼보다 실제로 더 많은 봉을 그리는 단계만 남긴다. */
  const steps = useMemo(() => {
    const seen = new Set<number>();
    const out: number[] = [];
    for (const step of BAR_STEPS) {
      const effective = Math.min(step, chrono.length);
      if (effective < 2 || seen.has(effective)) continue;
      seen.add(effective);
      out.push(step);
    }
    return out;
  }, [chrono.length]);

  // 고른 단계가 사라졌으면(지수를 바꿔 이력이 얕아졌다) 남은 것 중 가장 긴 것으로 떨어진다.
  const activeStep = steps.includes(bars) ? bars : (steps[steps.length - 1] ?? MAX_BARS);

  const windowed = chrono.slice(-activeStep);
  const chartPoints = usingSparkFallback
    ? (index?.sparkline ?? []).filter((v) => Number.isFinite(v) && v > 0)
    : windowed.map((r) => r.close);
  const chartTimes = usingSparkFallback ? undefined : windowed.map((r) => r.tradeDate);

  const peers = useMemo(
    () => allIndices.filter((i) => i.id !== indexId && i.value > 0).slice(0, 6),
    [allIndices, indexId],
  );

  const tone = score?.score != null ? scoreTone(score.score) : null;

  return (
    <>
      {/* 기사를 열면 이 모달을 닫는다 — 겹쳐 띄우면 백드롭 클릭이 둘 다 닫는다. */}
      <Dialog open={open && !newsModal.article} onOpenChange={(next) => !next && onClose()}>
        <DialogContent className="flex max-h-[88dvh] flex-col gap-0 overflow-hidden p-0 sm:max-w-lg">
          {index ? (
            <>
              <DialogHeader className="shrink-0 border-b px-4 py-3 pr-10 text-left">
                <DialogTitle className="flex min-w-0 items-center gap-2 text-sm">
                  <span className="min-w-0 flex-1 truncate">{index.name}</span>
                  <span className="shrink-0 rounded-md bg-muted px-1.5 py-0.5 text-[10px] font-semibold text-muted-foreground">
                    지수
                  </span>
                  <span className="shrink-0 text-right">
                    <span className="block font-mono text-base font-bold tabular-nums">
                      {index.value > 0 ? formatIndexValue(index.value) : "—"}
                    </span>
                    {index.value > 0 && (
                      <span
                        className={cn(
                          "block font-mono text-[11px] tabular-nums",
                          trendTextClass(index.changeRate),
                        )}
                      >
                        {index.change >= 0 ? "+" : ""}
                        {formatIndexValue(index.change)} ({formatPercent(index.changeRate)})
                      </span>
                    )}
                  </span>
                </DialogTitle>
                <DialogDescription className="text-[11px]">
                  {index.dataKind === "eod" ? "최근 거래일 종가" : "실시간"}
                  {index.updatedAt
                    ? ` · ${new Date(index.updatedAt).toLocaleTimeString("ko-KR", {
                        timeZone: "Asia/Seoul",
                        hour: "2-digit",
                        minute: "2-digit",
                      })} 기준`
                    : ""}
                  {index.badge ? ` · ${index.badge}` : ""}
                </DialogDescription>
              </DialogHeader>

              <div className="min-h-0 flex-1 overflow-y-auto px-4 py-3">
                <QuickLookSection
                  title={usingSparkFallback ? "최근 흐름" : "일별 종가"}
                  action={
                    steps.length > 1 && (
                      <div className="flex shrink-0 gap-0.5 rounded-lg bg-muted p-0.5">
                        {steps.map((step) => (
                          <button
                            key={step}
                            type="button"
                            onClick={() => setBars(step)}
                            className={cn(
                              "rounded-md px-2 py-0.5 text-[11px] font-medium transition-colors",
                              activeStep === step
                                ? "bg-card text-foreground shadow-sm"
                                : "text-muted-foreground hover:text-foreground",
                            )}
                          >
                            {barStepLabel(step)}
                          </button>
                        ))}
                      </div>
                    )
                  }
                >
                  {rows === null ? (
                    <div className="h-[96px] animate-pulse rounded bg-muted" />
                  ) : (
                    <>
                      <PriceHistoryChart
                        points={chartPoints}
                        times={chartTimes}
                        currency={index.currency}
                        valueFormat="plain"
                        title=""
                      />
                      {usingSparkFallback && (
                        <p className="mt-1 text-[10px] leading-relaxed text-muted-foreground">
                          이 지수는 일별 종가 이력이 아직 쌓이는 중입니다 — 실시간 스냅샷으로
                          그린 최근 흐름입니다.
                        </p>
                      )}
                    </>
                  )}
                </QuickLookSection>

                <QuickLookSection
                  title={
                    <span className="flex items-center gap-1.5">
                      AI 추세 점수
                      {score?.score != null && tone && (
                        <span
                          className={cn(
                            "rounded-full border px-1.5 text-[10px] font-semibold tabular-nums",
                            SCORE_TONE_TEXT[tone],
                            SCORE_TONE_BORDER[tone],
                          )}
                        >
                          {score.score}
                          {score.grade ? ` ${score.grade}` : ""}
                        </span>
                      )}
                    </span>
                  }
                >
                  {/*
                    금·비트코인은 점수가 `null`인데 **요약 문장은 온다**(20일선·RSI를 쓰는
                    점수 공식이 이 둘에는 적용되지 않는다). 점수 유무로 블록을 통째로
                    감추면 있는 문장까지 같이 사라진다.
                  */}
                  {score?.aiSummary ? (
                    <div className="rounded-lg border px-2.5 py-2">
                      <p className="text-[11px] leading-relaxed">{score.aiSummary}</p>
                      {/*
                        「AI」라고 부르되 무엇으로 만든 값인지는 정확히 적는다 — 이 점수와
                        위 문장은 20일선·RSI·최근 등락으로 계산한 것이지 LLM이 쓴 것이
                        아니다(`be/internal/service/index_score.go`: "no LLM").
                      */}
                      <p className="mt-1.5 text-[10px] leading-relaxed text-muted-foreground">
                        {score.tradeDate} 기준 ·{" "}
                        {score.score == null
                          ? "이 지수는 추세 점수가 산출되지 않습니다."
                          : "지수에는 재무제표가 없어 20일선·RSI 등 추세만 봅니다."}{" "}
                        정보이지 매매 신호가 아닙니다.
                      </p>
                    </div>
                  ) : (
                    <p className="py-2 text-center text-[11px] text-muted-foreground">
                      산출된 점수가 없습니다
                    </p>
                  )}
                </QuickLookSection>

                {rows !== null && rows.length > 0 && (
                  <QuickLookSection title="최근 일별">
                    <div className="flex h-[22px] items-center gap-2 border-b px-1 text-[10px] text-muted-foreground">
                      <span className="flex-1">날짜</span>
                      <span className="w-[76px] text-right">종가</span>
                      <span className="w-[56px] text-right">전일비</span>
                      <span className="w-[34px] text-right">점수</span>
                    </div>
                    <ul className="min-w-0">
                      {rows.slice(0, 6).map((r) => (
                        <li
                          key={r.tradeDate}
                          className="flex h-[26px] items-center gap-2 px-1 text-[11px]"
                        >
                          <span className="min-w-0 flex-1 truncate">
                            {r.tradeDate.slice(5)}{" "}
                            <span className="text-[10px] text-muted-foreground">({r.weekday})</span>
                          </span>
                          <span className="w-[76px] text-right font-mono tabular-nums">
                            {formatIndexValue(r.close)}
                          </span>
                          <span
                            className={cn(
                              "w-[56px] text-right font-mono tabular-nums",
                              r.changeRate == null
                                ? "text-muted-foreground"
                                : trendTextClass(r.changeRate),
                            )}
                          >
                            {r.changeRate == null ? "—" : formatPercent(r.changeRate)}
                          </span>
                          {/* 점수가 없는 날은 "—". 0으로 적으면 그날이 바닥이었다는 없는 사실이 된다. */}
                          <span
                            className={cn(
                              "w-[34px] text-right font-mono tabular-nums",
                              r.score == null
                                ? "text-muted-foreground"
                                : SCORE_TONE_TEXT[scoreTone(r.score)],
                            )}
                          >
                            {r.score ?? "—"}
                          </span>
                        </li>
                      ))}
                    </ul>
                  </QuickLookSection>
                )}

                {peers.length > 0 && (
                  <QuickLookSection title="같은 시각 다른 지수">
                    <div className="flex flex-wrap gap-1.5">
                      {peers.map((p) => (
                        <span
                          key={p.id}
                          className="flex items-center gap-1 rounded-full bg-muted px-2 py-0.5 text-[10px]"
                        >
                          <span className="text-muted-foreground">{p.name}</span>
                          <b className={cn("font-mono tabular-nums", trendTextClass(p.changeRate))}>
                            {formatPercent(p.changeRate)}
                          </b>
                        </span>
                      ))}
                    </div>
                  </QuickLookSection>
                )}

                <QuickLookSection
                  title={
                    <span className="flex items-center gap-1.5">
                      이 지수를 다룬 뉴스
                      {news != null && news.length > 0 && (
                        <span className="rounded bg-muted px-1 text-[10px] font-normal text-muted-foreground">
                          {news.length}건
                        </span>
                      )}
                    </span>
                  }
                >
                  {news === null ? (
                    <div className="space-y-1.5">
                      {Array.from({ length: 3 }).map((_, i) => (
                        <div key={i} className="h-8 animate-pulse rounded bg-muted" />
                      ))}
                    </div>
                  ) : news.length === 0 ? (
                    <p className="py-2 text-center text-[11px] text-muted-foreground">
                      이 지수를 다룬 기사가 아직 없습니다
                    </p>
                  ) : (
                    <ul>
                      {news.slice(0, 5).map((a) => (
                        <li key={a.id} className="border-b py-1 last:border-0">
                          <button
                            type="button"
                            onClick={() => newsModal.open(a)}
                            className="block w-full rounded-md px-1 py-0.5 text-left hover:bg-accent"
                          >
                            <p className="line-clamp-2 text-[11px] leading-snug">
                              {a.headlineKo || a.headline}
                            </p>
                            <p className="mt-0.5 flex items-center gap-1.5 text-[10px] text-muted-foreground">
                              <span className={a.publisher.badgeClass}>{a.publisher.name}</span>
                              {formatRelativeTime(a.publishedAt)}
                            </p>
                          </button>
                        </li>
                      ))}
                    </ul>
                  )}
                </QuickLookSection>
              </div>

              <div className="flex shrink-0 flex-wrap items-center gap-2 border-t px-4 py-3">
                <Button asChild size="sm" className="min-w-32 flex-1">
                  <Link href={market === "us" ? "/news/us" : "/news/domestic"} onClick={onClose}>
                    관련 뉴스 더보기
                    <ArrowRight className="size-3.5" />
                  </Link>
                </Button>
                <Button asChild size="sm" variant="outline" className="shrink-0">
                  <Link href="/market" onClick={onClose}>
                    실시간 차트
                  </Link>
                </Button>
              </div>
            </>
          ) : (
            <div className="p-6">
              <DialogHeader>
                <DialogTitle className="text-sm">지수</DialogTitle>
                <DialogDescription className="text-xs">지수를 불러오는 중…</DialogDescription>
              </DialogHeader>
            </div>
          )}
        </DialogContent>
      </Dialog>

      {newsModal.modal}
    </>
  );
}

/**
 * 지수를 여는 **한 가지** 방법 — 홈 티커 스트립 밖의 화면들이 공유한다.
 *
 * 레일의 「지수 흐름」은 지금까지 카드 제목 옆 「마켓 레이더」 링크 하나만 있고 다섯 줄
 * 자체는 눌리지 않았다. 홈 스트립에서는 같은 코스피 셀이 점수·일별 종가·관련 뉴스를
 * 펼쳐 주는데, 관심·보유 화면에서는 숫자 두 개로 끝났다.
 *
 * 점수는 이 훅이 배치로 한 번 받는다(`useIndexScores`) — 레일 카드가 열 때마다 부르면
 * 화면마다 같은 요청이 다시 나간다.
 */
export function useIndexQuickLook(allIndices: MarketIndex[]): {
  index: MarketIndex | null;
  open: (index: MarketIndex) => void;
  close: () => void;
  modal: React.ReactNode;
} {
  const [picked, setPicked] = useState<MarketIndex | null>(null);
  const scoresById = useIndexScores();

  const open = useCallback((index: MarketIndex) => setPicked(index), []);
  const close = useCallback(() => setPicked(null), []);

  return {
    index: picked,
    open,
    close,
    modal: (
      <IndexQuickLookModal
        index={picked}
        score={picked ? scoresById[picked.id] : undefined}
        allIndices={allIndices}
        open={picked != null}
        onClose={close}
      />
    ),
  };
}
