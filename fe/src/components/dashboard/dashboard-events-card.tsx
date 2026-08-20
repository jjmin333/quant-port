"use client";

import { useEffect, useMemo, useState } from "react";
import { SectionCard, SectionEmpty, SectionSkeleton } from "@/components/ui/section-card";
import { StockAvatar } from "@/components/stocks/stock-avatar";
import { fetchEarningsBatchClient } from "@/lib/earnings-api";
import type { EarningsCalendarEvent } from "@/lib/earnings-types";
import { earningsDaysFromToday, formatEarningsDateLabel } from "@/lib/earnings-date";
import type { QuoteSummary } from "@/lib/quotes-types";
import { cn } from "@/lib/utils";

/**
 * 실적·공시 — 홈에서 **시간축을 가진 유일한 칸**.
 *
 * 나머지 세 칸(점수 TOP·거래대금·오늘의 키워드)은 전부 "지금"이다. 그래서 실적 발표를
 * 발표 당일 뉴스로 알게 되는 일이 생긴다.
 *
 * ⚠️ **국내에는 예정 일정이 구조적으로 없다.** BE 주석이 그대로 말한다 — "DART has no
 * forward schedule API, so domestic symbols only ever produce `filed` events". 실측에서도
 * 거래대금 상위 국내 8종목의 `upcoming`이 0건이었다. 그래서 「다가오는 일정」이라는
 * 이름으로 국내 종목만 담으면 이 카드는 **영원히 빈 카드**가 된다.
 *
 * 대신 가진 것을 정확히 말한다 — **예정(미국 Finnhub)과 제출됨(국내 DART)을 한 목록에
 * 두되 행마다 어느 쪽인지 표시**한다. 「D-9 예정」과 「2일 전 제출」은 둘 다 알 값어치가
 * 있고, 섞어 놓고 구분하지 않는 것만이 거짓말이다. 예정이 먼저, 그다음이 최근 제출순이다.
 *
 * 대상 종목은 옆 칸의 **거래대금 상위**를 그대로 쓴다. 종목을 따로 고르면 "왜 이 종목의
 * 일정인지"를 설명할 길이 없는데, 바로 옆이 방금 그 목록을 보여줬으므로 이유가 화면에 있다.
 * 일정은 배치 엔드포인트로 **한 번에** 받는다(`/stocks/earnings/batch`). 여덟 종으로
 * 자르는 건 이제 요청 수 때문이 아니라 카드에 일곱 줄만 들어가기 때문이다.
 */

// 이름은 행에 담지 않는다 — 일정이 시세보다 먼저 올 수 있어(둘이 나란히 나간다)
// 담아 두면 그때의 빈 이름이 그대로 굳는다. 그릴 때 `nameBySymbol`에서 찾는다.
type Row = EarningsCalendarEvent & { symbol: string; days: number | null };

const MAX_SYMBOLS = 8;
/** 이보다 오래된 제출 공시는 "최근"이 아니다. */
const FILED_WINDOW_DAYS = 30;

/**
 * `symbols`는 랭킹이 오는 즉시, `quotes`는 그 뒤 배치 시세가 올 때 채워진다.
 *
 * 둘을 나눈 이유는 **기다림을 줄이기 위해서**다. 예전에는 이 카드가 `quotes`만 받아
 * 「랭킹 → 배치 시세 → 실적」 3단으로 줄을 섰다. 일정을 물어보는 데 필요한 건 종목
 * 코드뿐이라, 시세를 기다릴 이유가 없다 — 이제 시세와 일정이 나란히 나간다.
 * 이름은 늦게 와도 되고, 그동안은 종목 코드가 그 자리를 지킨다.
 */
export function DashboardEventsCard({
  symbols,
  quotes,
}: {
  symbols: string[] | null;
  quotes: QuoteSummary[] | null;
}) {
  const nameBySymbol = useMemo(
    () => new Map((quotes ?? []).map((q) => [q.symbol, q.name || q.symbol])),
    [quotes],
  );
  const targets = useMemo(
    () =>
      (symbols ?? []).slice(0, MAX_SYMBOLS).map((symbol) => ({
        symbol,
        name: nameBySymbol.get(symbol) || symbol,
      })),
    [symbols, nameBySymbol],
  );

  const key = targets.map((t) => t.symbol).join(",");
  const [loaded, setLoaded] = useState<{ key: string; rows: Row[] } | null>(null);

  useEffect(() => {
    if (!key) return;
    let alive = true;
    const wanted = key.split(",");
    // 예전에는 종목당 한 요청씩 여덟 개를 동시에 쐈다. 서버가 같은 팬아웃을 대신
    // 하되(handler.GetMany), 브라우저는 왕복 한 번만 쓴다 — 지연이 붙는 회선에서
    // 그 여덟이 연결 슬롯을 먹고 같은 화면의 다른 카드를 뒤로 밀었다.
    fetchEarningsBatchClient(wanted)
      .then((bySymbol) => {
        if (!alive) return;
        const rows = wanted.flatMap((symbol) =>
          (bySymbol[symbol]?.calendar ?? []).map((ev) => ({
            ...ev,
            symbol,
            days: earningsDaysFromToday(ev.date),
          })),
        );
        setLoaded({ key, rows });
      })
      // 요청 하나가 통째로 실패하면 카드는 빈 채로 둔다 — "일정 없음"이라고 적으면
      // 물어보지도 못한 것을 없다고 말하는 셈이다.
      .catch(() => {
        if (alive) setLoaded({ key, rows: [] });
      });
    return () => {
      alive = false;
    };
    // 심볼 배열은 렌더마다 새 참조라 문자열 키로 의존성을 잡는다.
  }, [key]);

  const rows = loaded?.key === key ? loaded.rows : null;

  const ordered = useMemo(() => {
    if (!rows) return null;
    const upcoming = rows
      .filter((r) => r.kind === "upcoming" && (r.days == null || r.days >= 0))
      .sort((a, b) => (a.days ?? 9999) - (b.days ?? 9999));
    const filed = rows
      .filter(
        (r) => r.kind === "filed" && r.days != null && r.days <= 0 && r.days >= -FILED_WINDOW_DAYS,
      )
      .sort((a, b) => (b.days ?? -9999) - (a.days ?? -9999));

    /*
     * **종목당 한 줄**. 실측에서 SK스퀘어 하나가 「반기보고서」와 그 기재정정 둘까지
     * 세 줄을 먹어, 일곱 줄이 전부 같은 날 같은 공시를 말했다. 한 종목이 목록을 다
     * 채우면 나머지 일곱 종목의 소식은 화면에서 사라진다.
     */
    const seen = new Set<string>();
    return [...upcoming, ...filed]
      .filter((r) => (seen.has(r.symbol) ? false : (seen.add(r.symbol), true)))
      .slice(0, 7);
  }, [rows]);

  // 옆 칸이 아직 종목을 못 받았으면 물어볼 대상조차 없다. 그때 "일정 없음"이라고
  // 적으면 사실이 아니다 — 아직 물어보지도 않았다.
  const pending = symbols === null || (targets.length > 0 && ordered === null);

  return (
    <SectionCard title="실적 · 공시" href="/earnings" hrefLabel="실적 캘린더">
      {pending ? (
        <SectionSkeleton rows={6} />
      ) : !ordered || ordered.length === 0 ? (
        <SectionEmpty text="예정·최근 공시가 없습니다" />
      ) : (
        <ul className="min-w-0">
          {ordered.map((event, i) => {
            const soon = event.kind === "upcoming" && event.days != null && event.days <= 3;
            // 시세가 아직이면 종목 코드로 버틴다 — 자리를 비워 두면 줄이 흔들린다.
            const name = nameBySymbol.get(event.symbol) || event.symbol;
            return (
              <li
                key={`${event.symbol}-${event.date}-${i}`}
                className="flex h-[32px] min-w-0 items-center gap-1.5 px-1 text-xs"
              >
                <span
                  className={cn(
                    "w-10 shrink-0 text-right font-mono text-[11px] tabular-nums",
                    soon ? "font-bold text-[var(--brand)]" : "text-muted-foreground",
                  )}
                >
                  {formatEarningsDateLabel(event.date)}
                </span>
                <StockAvatar
                  symbol={event.symbol}
                  name={name}
                  size="xs"
                  className="shrink-0"
                />
                <span className="min-w-0 max-w-[45%] truncate">{name}</span>
                {/*
                  이 칸의 값은 "무슨 일이 있느냐"다. 예정/제출 배지만 달고 라벨을 툴팁에
                  숨겼더니 일곱 줄이 「제출」 일곱 개가 됐다 — 앞일인지 지난 일인지는 왼쪽
                  날짜 표기(`D-9` ↔ `2일 전`)가 이미 말하고 있다.
                */}
                <span
                  className="min-w-0 flex-1 truncate text-right text-[10px] text-muted-foreground"
                  title={`${event.label.trim()} · ${event.kind === "upcoming" ? "예정" : "제출됨"}`}
                >
                  {event.label.trim()}
                </span>
              </li>
            );
          })}
        </ul>
      )}
      <p className="mt-1.5 border-t pt-1.5 text-[10px] leading-relaxed text-muted-foreground">
        거래대금 상위 종목 기준. 국내는 예정 일정을 제공하는 소스가 없어, 앞으로의 일정은{" "}
        <b>작년 발표일로 추정</b>한 값입니다.
      </p>
    </SectionCard>
  );
}
