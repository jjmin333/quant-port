"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { isPlainLeftClick } from "@/lib/plain-click";
import { useEffect, useMemo, useState } from "react";
import { Activity, CircleHelp } from "lucide-react";
import { useAuth } from "@/components/providers/auth-provider";
import { LoadErrorState } from "@/components/common/load-error-state";
import { StockAvatar } from "@/components/stocks/stock-avatar";
import { MiniToggle } from "@/components/ui/mini-toggle";
import { analysisSymbolHref } from "@/lib/analysis-tabs";
import { formatMarketCap } from "@/lib/format";
import { HelpHint } from "@/components/ui/help-hint";
import {
  fetchChanges,
  groupChanges,
  type ChangeEvent,
  type ChangeKind,
  type ChangesResponse,
} from "@/lib/changes-api";
import { cn } from "@/lib/utils";

/**
 * 「오늘의 변화」 — 그날 전 종목에서 일어난 일을 규칙으로 골라 보여 준다.
 *
 * 이 화면이 지키는 두 가지 (be/internal/service/change_detect.go 머리글):
 *  1. **데이터가 만든 변화는 없다.** AI 점수가 축이 생기거나 사라져서 움직인 경우는 서버가
 *     이미 뺐다 — 화면은 그 사실을 한 줄로 알린다.
 *  2. **설명할 수 없는 급등락을 숨기지 않는다.** 같은 날 사건도 기사도 없으면 「설명 불가」
 *     배지를 단다. 이유를 지어내지 않는다.
 *
 * 종목을 누르면 AI 분석 탭으로 간다 — 그 탭에 점수 근거와 타임라인이 있다.
 */

type Market = "domestic" | "us";

// 「전체」는 없다(2026-10-09 요청 3) — 순위가 시가총액이라 원·달러 종목을 한 줄에 세울 수 없다.
const MARKETS: { id: Market; label: string }[] = [
  { id: "domestic", label: "국내" },
  { id: "us", label: "해외" },
];

// 서버 응답이 오기 전에도 칩 순서가 흔들리지 않도록 같은 순서를 여기에도 둔다.
const KIND_ORDER: ChangeKind[] = [
  "price_move",
  "volume_surge",
  "score_jump",
  "news_event",
  "disclosure",
  "earnings",
  "insider",
  "social",
];

/** `initialDate` — 날짜별 보관 주소(`/changes/[date]`)가 그날로 연다(2026-10-10). */
export function ChangesClient({ initialDate = "" }: { initialDate?: string } = {}) {
  const { session } = useAuth();
  const router = useRouter();
  const [date, setDate] = useState<string>(initialDate);
  const [market, setMarket] = useState<Market>("domestic");
  const [kinds, setKinds] = useState<ChangeKind[]>([]);
  const [mine, setMine] = useState(false);
  const [data, setData] = useState<ChangesResponse | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [reloadKey, setReloadKey] = useState(0);

  const mineActive = mine && Boolean(session);

  useEffect(() => {
    let alive = true;
    // 요청 시작 시 로딩 표시는 이펙트 밖 핸들러에서 켜지 않고 여기서 켠다 — 첫 적재와
    // 필터 변경이 같은 길로 간다.
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setLoading(true);
    fetchChanges({ date: date || undefined, market, kinds, mine: mineActive })
      .then((res) => {
        if (!alive) return;
        setData(res);
        setError(null);
      })
      .catch((e: unknown) => {
        if (!alive) return;
        setError(e instanceof Error ? e.message : "변화를 불러오지 못했습니다");
      })
      .finally(() => {
        if (alive) setLoading(false);
      });
    return () => {
      alive = false;
    };
  }, [date, market, kinds, mineActive, reloadKey]);

  const groups = useMemo(() => groupChanges(data?.items ?? []), [data]);
  const kindLabels = useMemo(() => {
    const m = new Map<string, string>();
    for (const k of data?.kinds ?? []) m.set(k.id, k.label);
    return m;
  }, [data]);
  const kindCounts = useMemo(() => {
    const m = new Map<string, number>();
    for (const k of data?.kinds ?? []) if (typeof k.count === "number") m.set(k.id, k.count);
    return m;
  }, [data]);
  const unexplained = groups.filter((g) => g.hasUnexplained).length;

  function toggleKind(k: ChangeKind) {
    setKinds((cur) => (cur.includes(k) ? cur.filter((x) => x !== k) : [...cur, k]));
  }

  return (
    <main className="flex min-h-0 min-w-0 flex-1 flex-col overflow-y-auto">
      <div className="page-shell-wide flex min-w-0 flex-col gap-3 py-4">
        <header className="flex min-w-0 flex-wrap items-end justify-between gap-2">
          <div className="min-w-0">
            <h1 className="flex items-center gap-1.5 text-base font-semibold">
              <Activity className="h-4 w-4 text-[var(--brand)]" aria-hidden />
              오늘의 변화
              <HelpHint>
                시가총액 순위입니다. 급등락은 종목마다 평소 변동성을 기준으로 골랐습니다. AI 점수가
                평가 축이 생기거나 사라져서, 또는 비교 집단이 바뀌어서 달라진 경우와 종목의 15% 이상이
                한꺼번에 움직인 날의 점수 변화는 싣지 않습니다. 투자 권유가 아니라 그날 일어난 일의
                기록입니다.
              </HelpHint>
            </h1>
            <p className="mt-0.5 text-cell text-muted-foreground">
              {data?.date ? `기준 ${data.date} · 시가총액 순` : "시가총액 순"}
              {unexplained > 0 && ` · 설명할 수 없는 급등락 ${unexplained}종목`}
            </p>
          </div>
          {data && data.dates.length > 0 && (
            <label className="flex items-center gap-1.5 text-cell text-muted-foreground">
              날짜
              <select
                className="rounded-md border border-[var(--border)] bg-card px-2 py-1 text-cell text-foreground"
                value={date || data.date}
                onChange={(e) => setDate(e.target.value)}
              >
                {data.dates.map((d) => (
                  <option key={d} value={d}>
                    {d}
                  </option>
                ))}
              </select>
            </label>
          )}
        </header>

        <div className="flex min-w-0 flex-wrap items-center gap-2">
          <MiniToggle
            options={MARKETS}
            value={market}
            onChange={setMarket}
          />
          <button
            type="button"
            onClick={() => setMine((v) => !v)}
            disabled={!session}
            aria-pressed={mineActive}
            title={session ? undefined : "로그인하면 관심·보유 종목만 볼 수 있습니다"}
            className={cn(
              "rounded-full border px-2.5 py-0.5 text-cell transition-colors disabled:cursor-not-allowed disabled:opacity-50",
              mineActive
                ? "border-[var(--brand)] bg-[var(--brand)]/10 font-semibold text-[var(--brand)]"
                : "border-[var(--border)] text-muted-foreground hover:text-foreground",
            )}
          >
            내 종목만
          </button>
        </div>

        {kindLabels.size > 0 && (
          <div
            className="-mx-1 flex min-w-0 gap-1.5 overflow-x-auto px-1 pb-1"
            role="group"
            aria-label="변화 종류 (여러 개 고르면 모두 해당하는 종목만)"
          >
            {KIND_ORDER.filter((k) => kindLabels.has(k)).map((k) => {
              const on = kinds.includes(k);
              const count = kindCounts.get(k);
              // 그날 그 종류가 하나도 없으면 눌러 봐야 빈 목록이다 — 켜져 있을 때만 끌 수 있게 둔다.
              const empty = count === 0 && !on;
              return (
                <button
                  key={k}
                  type="button"
                  onClick={() => toggleKind(k)}
                  aria-pressed={on}
                  disabled={empty}
                  title={empty ? "이 날은 해당 변화가 없습니다" : undefined}
                  className={cn(
                    "shrink-0 rounded-full border px-2.5 py-0.5 text-cell transition-colors disabled:cursor-not-allowed disabled:opacity-40",
                    on
                      ? "border-foreground bg-foreground font-semibold text-background"
                      : "border-[var(--border)] text-muted-foreground hover:text-foreground",
                  )}
                >
                  {kindLabels.get(k)}
                  {count !== undefined && (
                    <span className="ml-1 tabular-nums opacity-70">{count}</span>
                  )}
                </button>
              );
            })}
          </div>
        )}

        {error ? (
          <LoadErrorState message={error} onRetry={() => setReloadKey((k) => k + 1)} />
        ) : loading && !data ? (
          <p className="py-12 text-center text-cell text-muted-foreground">변화를 불러오는 중…</p>
        ) : !data || data.dates.length === 0 ? (
          <p className="py-12 text-center text-cell text-muted-foreground">
            아직 변화 기록이 없습니다. 매일 00:25 점수 계산이 끝나면 채워집니다.
          </p>
        ) : groups.length === 0 ? (
          <p className="py-12 text-center text-cell text-muted-foreground">
            {mineActive
              ? "이 날 관심·보유 종목에는 조건에 맞는 변화가 없습니다."
              : kinds.length > 1
                ? "이 날은 고른 변화를 모두 가진 종목이 없습니다."
                : "이 날은 조건에 맞는 변화가 없습니다."}
          </p>
        ) : (
          <ul className={cn("flex min-w-0 flex-col gap-2", loading && "opacity-60")}>
            {groups.map((g) => (
              <li key={g.symbol}>
                {/*
                  href는 종목 페이지(크롤러·새 탭용, 2026-10-09 검색 색인 개선 B), 일반 클릭은
                  예전처럼 AI 랭킹 화면의 AI 탭으로 간다.
                */}
                <Link
                  href={`/stocks/${encodeURIComponent(g.symbol)}`}
                  prefetch={false}
                  onClick={(e) => {
                    if (!isPlainLeftClick(e)) return;
                    e.preventDefault();
                    router.push(analysisSymbolHref(g.symbol, "ai"));
                  }}
                  className="flex min-w-0 gap-3 rounded-lg border border-[var(--border)] bg-card p-3 transition-colors hover:bg-muted/40"
                >
                  <span
                    className={cn(
                      "mt-1 w-6 shrink-0 text-right text-sm font-bold tabular-nums",
                      g.rank <= 3 ? "text-[var(--brand)]" : "text-muted-foreground",
                    )}
                    aria-label={`시가총액 순위 ${g.rank}위`}
                  >
                    {g.rank}
                  </span>
                  <StockAvatar
                    symbol={g.symbol}
                    name={g.name}
                    assetType={g.assetType === "us" ? "us" : "domestic"}
                    size="sm"
                    className="mt-0.5 shrink-0"
                  />
                  <div className="min-w-0 flex-1">
                    <div className="flex min-w-0 items-baseline gap-1.5">
                      <span className="truncate text-sm font-semibold">{g.name}</span>
                      <span className="shrink-0 text-micro text-muted-foreground">{g.symbol}</span>
                      {g.marketCap > 0 && (
                        <span className="ml-auto shrink-0 text-micro tabular-nums text-muted-foreground">
                          시총 {formatMarketCap(g.marketCap, market === "us" ? "USD" : "KRW")}
                        </span>
                      )}
                    </div>
                    <ul className="mt-1 flex min-w-0 flex-col gap-0.5">
                      {g.events.map((e, i) => (
                        <EventLine
                          key={`${e.kind}-${e.subtype ?? ""}-${i}`}
                          e={e}
                          kindLabel={kindLabels.get(e.kind) ?? e.kind}
                        />
                      ))}
                    </ul>
                  </div>
                </Link>
              </li>
            ))}
          </ul>
        )}
      </div>
    </main>
  );
}

function EventLine({ e, kindLabel }: { e: ChangeEvent; kindLabel: string }) {
  const tone =
    e.direction > 0
      ? "text-[var(--profit)] border-[var(--profit)]/40"
      : e.direction < 0
        ? "text-[var(--loss)] border-[var(--loss)]/40"
        : "text-muted-foreground border-[var(--border)]";
  const threshold =
    e.kind === "price_move" && typeof e.detail?.thresholdPct === "number"
      ? `평소 기준 ±${e.detail.thresholdPct.toFixed(1)}%`
      : null;
  return (
    <li className="flex min-w-0 items-start gap-1.5 text-cell">
      <span className={cn("mt-px shrink-0 rounded border px-1 text-micro leading-4", tone)}>
        {kindLabel}
      </span>
      <span className="min-w-0 flex-1 break-words">
        {e.title}
        {threshold && <span className="ml-1 text-micro text-muted-foreground">{threshold}</span>}
      </span>
      {e.explained === false && e.kind === "price_move" && (
        <span
          className="flex shrink-0 items-center gap-0.5 rounded bg-amber-500/15 px-1 text-micro leading-4 text-amber-700 dark:text-amber-400"
          title="같은 날 이 종목에 대해 우리가 수집한 공시·실적·뉴스·내부자 기록이 없었습니다 — 원인이 없다는 뜻이 아니라, 우리 데이터로 설명되지 않는다는 뜻입니다"
        >
          <CircleHelp className="h-3 w-3" aria-hidden />
          설명 불가
        </span>
      )}
    </li>
  );
}
