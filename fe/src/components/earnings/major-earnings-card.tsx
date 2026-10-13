"use client";

import { useMemo } from "react";
import {
  DATE_STATUS_META,
  DateStatusBadge,
  earningsEventKey,
  eventName,
  periodLabel,
} from "@/components/earnings/earnings-shared";
import { StockAvatar } from "@/components/stocks/stock-avatar";
import { QuoteCell } from "@/components/ui/quote-cell";
import { SectionCard, SectionEmpty, SectionSkeleton } from "@/components/ui/section-card";
import { useLiveQuotes } from "@/hooks/use-live-quotes";
import { DASHBOARD_RANK_ROWS } from "@/lib/dashboard-rows";
import { earningsDaysFromToday, formatEarningsDateLabel } from "@/lib/earnings-date";
import type { EarningsDateStatus, EarningsEvent } from "@/lib/earnings-types";
import { formatFinancialAmount } from "@/lib/format";
import { cn } from "@/lib/utils";

/*
「큰 발표가 다가온다」를 그리는 카드 — **한 벌** (2026-09-05 사용자 요청 7).

# 무엇이 두 벌이었나

홈의 「주요 실적 일정」(`dashboard-events-card`)과 실적 화면 레일의 「다가오는 대형
발표」(`earnings-side-rail`)가 **같은 응답의 같은 칸**을 그렸다 —
`GET /earnings/highlights`의 `big`. 고르는 기준(시가총액 상위)도, 국내 예정일이
추정치라는 각주도 같았다.

다른 것은 셋이었다:

	홈   : 날짜 오름차순 · 오른쪽에 지금 시세 · 시장 토글을 자기가 가짐
	레일 : 서버가 준 시가총액 순 · 오른쪽에 확정 배지·날짜·시가총액 · 시장은 화면이 정함

앞의 둘은 **자리의 차이**다. 홈에서 이 카드는 시간축을 가진 유일한 칸이라 「무엇이
먼저 오는가」를 말해야 하고, 실적 화면에서는 표가 이미 날짜순이라 레일이 답할 것은
「무엇이 큰 발표인가」다. 그래서 정렬과 오른쪽 칸만 `variant`로 가른다.

# 고르는 기준과 늘어놓는 기준은 다르다

서버는 다가오는 발표 중 시가총액 상위 여섯을 골라 준다 — 「주요」를 정하는 건 크기가
맞다. 그런데 홈에서 받은 그대로 그리면 D-44, D-59, D-59, D-43처럼 날짜가 뒤죽박죽
서서, 시간축을 가진 유일한 칸이 정작 「무엇이 먼저 오는가」를 못 말한다.

그래서 **고르기는 서버(시총), 늘어놓기는 여기(날짜)**로 나눈다. 정렬을 서버로 옮기면
상위 여섯을 고르는 기준까지 날짜가 되어 대형주가 목록에서 빠진다.

# 가져오는 것은 이 카드가 하지 않는다

`events`를 받는다. 레일은 하이라이트 한 응답에서 카드 셋을 그리므로 이미 받아 둔 것을
넘기고, 홈은 `useEarningsHighlights`로 받아 넘긴다 — 카드가 스스로 부르면 레일 한 줄에
왕복이 셋이 된다.
*/

export type MajorEarningsVariant = "home" | "rail";

type MajorEarningsCardProps = {
  /** 아직 받는 중이면 `null`, 받았는데 없으면 `[]`. 둘을 같은 화면으로 그리면 안 된다. */
  events: EarningsEvent[] | null;
  /**
   * 최근 발표된 대형 실적 — 홈만. 다가오는 발표가 일곱 줄에 못 미치면 그 아래를 가까운 순으로
   * 채운다(2026-10-13 사용자 요청 — 옆 카드와 줄 수가 같아야 카드 밑이 비지 않는다).
   * 날짜 칸이 「3일 전」으로 적혀 다가오는 줄과 갈린다.
   */
  recent?: EarningsEvent[];
  variant: MajorEarningsVariant;
  /** 줄을 눌렀을 때. 홈은 종목 퀵룩을, 실적 화면은 발표 상세를 연다. */
  onPick: (event: EarningsEvent) => void;
  /** 제목 오른쪽 — 홈의 시장 토글이 여기로 들어온다. */
  action?: React.ReactNode;
  /** 카드 오른쪽 위의 「실적 →」. 레일에서는 그 화면이 이미 실적이라 안 붙인다. */
  href?: string;
};

/**
 * 날짜 확정 여부를 **한 마디**로. 홈에서는 배지 대신 날짜의 툴팁이 이 말을 한다 —
 * 배지가 있던 자리를 시세에 내줬기 때문이고(2026-09-04 사용자 요청 7), 국내 일정
 * 대부분이 작년 발표일로 역산한 추정치라 구분 자체는 계속 필요하다.
 */
function dateStatusLabel(status: EarningsDateStatus): string {
  return DATE_STATUS_META[status]?.title ?? "";
}

export function MajorEarningsCard({
  events,
  recent,
  variant,
  onPick,
  action,
  href,
}: MajorEarningsCardProps) {
  const home = variant === "home";

  const rows = useMemo(() => {
    if (events === null) return null;
    // 홈만 날짜순으로 다시 세운다 — 레일은 「무엇이 큰가」를 말하는 자리라 시총 순이다.
    if (!home) return events;
    const upcoming = [...events].sort((a, b) => a.date.localeCompare(b.date));
    const seen = new Set(upcoming.map(earningsEventKey));
    const past = (recent ?? [])
      .filter((e) => !seen.has(earningsEventKey(e)))
      .sort((a, b) => b.date.localeCompare(a.date));
    return [...upcoming, ...past].slice(0, Math.max(DASHBOARD_RANK_ROWS, upcoming.length));
  }, [events, recent, home]);

  /*
    발표를 앞둔 종목의 **지금 시세**. 홈에서만 붙인다 — 레일의 그 자리는 시가총액이
    쓰고 있고, 「무엇이 큰 발표인가」에 답하는 값이 거기서는 그쪽이다.

    빈 배열을 넘기면 `useLiveQuotes`가 아무것도 부르지 않는다.
  */
  const liveSymbols = useMemo(
    () => (home && rows ? rows.map((e) => e.symbol) : []),
    [home, rows],
  );
  const live = useLiveQuotes(liveSymbols);

  return (
    <SectionCard
      title={home ? "주요 실적 일정" : "다가오는 대형 발표"}
      href={href}
      action={action}
    >
      {rows === null ? (
        <SectionSkeleton rows={home ? 7 : 4} />
      ) : rows.length === 0 ? (
        <SectionEmpty text="예정된 발표가 없습니다" />
      ) : (
        <ul className="@container min-w-0">
          {rows.map((event) => {
            const name = eventName(event);
            const days = earningsDaysFromToday(event.date);
            /* 확정이 아닌데 사흘 안이면 날짜를 굵게 — 「곧인데 아직 추정」이 가장 흔들리는 값이다. */
            const soon =
              event.dateStatus !== "confirmed" && days != null && days >= 0 && days <= 3;
            const currency = event.currency === "USD" ? "USD" : "KRW";

            return (
              <li key={earningsEventKey(event)}>
                <button
                  type="button"
                  onClick={() => onPick(event)}
                  className={cn(
                    "flex w-full min-w-0 items-center gap-1.5 px-1 text-left text-cell",
                    home ? "h-[32px] rounded-md hover:bg-accent" : "h-[34px] hover:bg-muted/50",
                  )}
                >
                  {home && (
                    <span
                      className={cn(
                        "w-10 shrink-0 text-right font-num text-cell tabular-nums",
                        soon ? "font-bold text-[var(--brand)]" : "text-muted-foreground",
                      )}
                      title={`${periodLabel(event)} · ${dateStatusLabel(event.dateStatus)}`}
                    >
                      {formatEarningsDateLabel(event.date)}
                    </span>
                  )}

                  <StockAvatar
                    symbol={event.symbol}
                    name={name}
                    currency={currency}
                    market={event.market}
                    size="xs"
                    className="shrink-0"
                  />
                  <span className={cn("min-w-0 truncate", home ? "max-w-[45%]" : "flex-1")}>
                    {name}
                  </span>

                  {home ? (
                    /*
                      **오른쪽 끝은 가격 · 등락률이다** (2026-09-04 사용자 요청 7).

                      전에는 「예정」 배지와 「2027 1Q」가 그 자리를 썼다. 둘 다 이 카드가
                      이미 말하고 있는 것을 되풀이한다 — 카드 이름이 「주요 **실적 일정**」
                      이고 왼쪽 끝에 날짜가 있으니 「예정」은 새로운 정보가 아니고, 분기
                      표기는 그 날짜에서 따라 나온다.
                    */
                    <span className="ml-auto flex min-w-0 items-center justify-end gap-1">
                      <QuoteCell
                        price={live.get(event.symbol.toUpperCase())?.livePrice ?? 0}
                        changeRate={live.get(event.symbol.toUpperCase())?.liveChangeRate ?? 0}
                        currency={currency}
                        symbol={event.symbol}
                      />
                    </span>
                  ) : (
                    <>
                      <DateStatusBadge status={event.dateStatus} />
                      <span className="shrink-0 font-num text-micro tabular-nums text-muted-foreground">
                        {formatEarningsDateLabel(event.date)}
                      </span>
                      <span className="shrink-0 font-num text-micro tabular-nums">
                        {event.marketCap != null
                          ? formatFinancialAmount(event.marketCap, currency)
                          : "—"}
                      </span>
                    </>
                  )}
                </button>
              </li>
            );
          })}
        </ul>
      )}

      {/* 정렬·추정 풀이 문단은 걷었다(2026-10-09 요청 9) — 추정일은 줄의 「추정」 배지가 말한다. */}
    </SectionCard>
  );
}
