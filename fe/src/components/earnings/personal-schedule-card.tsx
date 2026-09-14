"use client";

import { CalendarDays } from "lucide-react";
import { QuoteCell } from "@/components/ui/quote-cell";
import { SectionCard, SectionEmpty, SectionSkeleton } from "@/components/ui/section-card";
import { daysUntil, type UpcomingEvent } from "@/hooks/use-upcoming-events";
import { useLiveQuotes } from "@/hooks/use-live-quotes";
import { useMemo } from "react";
import { cn } from "@/lib/utils";

/*
「다가오는 일정」 — **한 벌** (2026-09-05 사용자 요청 7).

# 무엇이 세 벌이었나

관심(`watchlist-schedule-card`)·보유(`holdings-schedule-card`)·뉴스 레일
(`news-rail-cards`의 `NewsUpcomingEventsCard`)에 같은 카드가 있었다. 셋 다 제목이
「다가오는 일정」이고, 같은 값(예정된 실적·공시)을 날짜순으로 여섯 줄 그리고, 빈
문구까지 같았다.

다른 것은 오른쪽 칸과 **가져오는 방식**이었다:

	관심   : D-데이 배지 + DART/실적 배지 · 화면이 `useUpcomingEvents`로 한 번 받아 넘김
	보유   : DART/실적 배지만          · **종목당 1요청**(`fetchEarningsClient` × 12)
	뉴스   : 지금 시세(QuoteCell)      · 화면이 `useUpcomingEvents`로 받아 넘김

# 보유 화면의 열두 요청

보유 카드만 배치를 안 쓰고 있었다. 상위 12종목을 도는 `Promise.all`이라 **보유 화면을
여는 것만으로 요청 12개**가 한꺼번에 나갔다. `useUpcomingEvents`가 이미 그 일을
배치 한 번(`/stocks/earnings/batch`)으로 하고 있었고 — 09-04에 나머지 두 화면은 그리로
옮겼는데 이 카드만 남아 있었다. 지금은 셋 다 같은 훅을 쓴다.

# 오른쪽 칸만 고른다

D-데이·출처 배지는 「언제·무엇」을 말하고, 시세는 「그 종목이 지금 어떤가」를 말한다.
뉴스 레일에서 시세를 붙인 것은 그 화면에 종목 시세가 없기 때문이고(2026-09-04 요청 8),
관심·보유 화면에는 목록 본문이 이미 시세를 적고 있어 레일에서 되풀이할 이유가 없다.
그 차이만 `showQuotes`로 가른다.
*/

type PersonalScheduleCardProps = {
  /** 아직 받는 중이면 `null`. 화면이 `useUpcomingEvents`로 한 번 받아 넘긴다. */
  events: UpcomingEvent[] | null;
  /** 행을 눌렀을 때 — 「무슨 일정인지」 다음 질문은 「그 종목은 지금 어떤가」다. */
  onPick?: (symbol: string) => void;
  /**
   * 오른쪽 끝에 **지금 시세**를 적는다(뉴스 레일). 그러면 배치 시세를 한 번 부른다.
   *
   * 관심·보유 화면에서는 끈다 — 본문 목록이 이미 같은 종목의 시세를 적고 있다.
   */
  showQuotes?: boolean;
  /** 시세를 적을 때 이름·통화의 기본값을 정하는 시장. */
  market?: "domestic" | "us";
  href?: string;
  hrefLabel?: string;
  /** 목록 아래 한 줄 — 뉴스 레일의 「예시입니다」 안내가 여기로 들어온다. */
  footer?: React.ReactNode;
  /** 그릴 줄 수. */
  max?: number;
};

export function PersonalScheduleCard({
  events,
  onPick,
  showQuotes = false,
  market = "domestic",
  href,
  hrefLabel,
  footer,
  max = 6,
}: PersonalScheduleCardProps) {
  const rows = events?.slice(0, max) ?? null;

  /* 빈 배열을 넘기면 `useLiveQuotes`가 아무것도 부르지 않는다 — 시세를 안 쓰는 화면의 값. */
  const quoteSymbols = useMemo(
    () => (showQuotes && rows ? rows.map((e) => e.symbol) : []),
    [showQuotes, rows],
  );
  const live = useLiveQuotes(quoteSymbols);

  return (
    <SectionCard
      title={
        showQuotes ? (
          "다가오는 일정"
        ) : (
          <span className="flex items-center gap-1.5">
            <CalendarDays className="size-3.5" />
            다가오는 일정
          </span>
        )
      }
      href={href}
      hrefLabel={hrefLabel}
    >
      {rows === null ? (
        <SectionSkeleton rows={showQuotes ? 4 : 3} />
      ) : rows.length === 0 ? (
        <SectionEmpty text="예정된 실적·공시가 없습니다" />
      ) : (
        <ul className="@container min-w-0">
          {rows.map((ev, i) => {
            const dday = daysUntil(ev.date);
            const q = live.get(ev.symbol.toUpperCase());
            /*
              이름은 **시세가 준 것을 먼저** 쓴다. `ev.name`은 화면이 들고 있던 시세
              맵에서 온 것인데, 그 맵에 없는 종목은 심볼로 되돌아간다 — 실측(2026-09-04
              비로그인 국내 뉴스)에서 세 줄이 전부 종목코드로 떴다. 시세를 이미 부르는
              화면에서는 요청을 더 늘리지 않고 고칠 수 있다.
            */
            const name = q?.name || ev.name;

            const row = (
              <>
                <span
                  className={cn(
                    "shrink-0 font-num tabular-nums text-muted-foreground",
                    showQuotes ? "w-12" : "w-[38px]",
                  )}
                >
                  {showQuotes ? relativeDateLabel(ev.date) : ev.date.slice(5)}
                </span>
                <span className="min-w-0 flex-1">
                  <span className={cn("block truncate", !showQuotes && "font-medium")}>{name}</span>
                  <span className="block truncate text-micro text-muted-foreground">
                    {ev.label.trim() || "실적 발표"}
                  </span>
                </span>

                {showQuotes ? (
                  <QuoteCell
                    price={q?.livePrice ?? 0}
                    changeRate={q?.liveChangeRate ?? 0}
                    currency={(q?.currency ?? (market === "domestic" ? "KRW" : "USD")) as "KRW" | "USD"}
                    symbol={ev.symbol}
                  />
                ) : (
                  <>
                    {/* 2주 안쪽만 D-데이를 단다 — 더 먼 것은 왼쪽 날짜로 충분하다. */}
                    {dday != null && dday <= 14 && (
                      <span className="shrink-0 rounded bg-[var(--brand-muted)] px-1 text-micro font-semibold text-[var(--brand)]">
                        {dday === 0 ? "오늘" : `D-${dday}`}
                      </span>
                    )}
                    <span
                      className={cn(
                        "shrink-0 rounded px-1 text-micro",
                        ev.source === "dart"
                          ? "bg-[var(--brand-muted)] text-[var(--brand)]"
                          : "bg-muted text-muted-foreground",
                      )}
                    >
                      {ev.source === "dart" ? "DART" : "실적"}
                    </span>
                  </>
                )}
              </>
            );

            return (
              <li key={`${ev.symbol}-${ev.date}-${i}`} className="min-w-0">
                {onPick ? (
                  <button
                    type="button"
                    onClick={() => onPick(ev.symbol)}
                    className="flex w-full min-w-0 items-start gap-2 rounded-md px-1 py-1 text-left text-cell hover:bg-accent"
                  >
                    {row}
                  </button>
                ) : (
                  <span className="flex items-center gap-2 border-b border-border/60 py-1.5 text-cell last:border-b-0">
                    {row}
                  </span>
                )}
              </li>
            );
          })}
        </ul>
      )}
      {footer}
    </SectionCard>
  );
}

/** 뉴스 레일의 날짜 칸 — 가까운 것은 D-데이로, 먼 것은 월/일로. */
function relativeDateLabel(iso: string): string {
  const at = new Date(iso);
  if (Number.isNaN(at.getTime())) return iso.slice(5);
  const days = Math.round((at.getTime() - Date.now()) / 86_400_000);
  if (days <= 0) return "오늘";
  if (days <= 30) return `D-${days}`;
  return `${at.getMonth() + 1}/${at.getDate()}`;
}
