import type { CSSProperties } from "react";
import Link from "next/link";
import { CalendarClock, GitCompareArrows, Layers, LineChart } from "lucide-react";
import { ScoreRadar } from "@/components/analysis/score-radar";
import { GuestHeroSearch, GuestMovers, GuestOnly } from "@/components/dashboard/home-guest-client";
import { Card } from "@/components/ui/card";
import { earningsDaysFromToday } from "@/lib/earnings-date";
import type { GuestShowcase } from "@/lib/home-guest.server";
import { peerGroupLabel } from "@/lib/score-explain-api";
import {
  SCORE_AXIS_LABELS,
  SCORE_AXIS_ORDER,
  SCORE_TONE_TEXT,
  scoreTone,
  type StockScoreData,
} from "@/lib/stock-score-api";
import { cn } from "@/lib/utils";

/*
홈 비로그인 블록 (2026-10-10 사용자 결정 — 시안 `docs/mockups/home-guest-v2.html` A안, 대표 종목 고정, 성적 숫자 공개).

처음 온 사람이 3초 안에 「무엇을 하는 서비스인지」 알게 한다: 질문 하나(같은 업종 안에서 몇 등?) →
검색 → 이름을 아는 종목의 점수 모양 → 오늘 바뀐 것 → 다가오는 실적 → 다른 서비스와 다른 셋. 로그인한
사람에게는 그리지 않는다(지금 홈 그대로). 서버가 그리므로 크롤러도 같은 본문을 본다.
*/

/** 어두운 히어로 위에서 레이더가 쓸 색 — 컴포넌트는 CSS 변수로 칠한다. */
const HERO_RADAR_VARS = {
  "--border": "rgba(255,255,255,0.22)",
  "--text-muted": "#CBD5E1",
  "--text-primary": "#FFFFFF",
  "--brand": "#60A5FA",
} as CSSProperties;

function extremes(s: StockScoreData): { best?: string; worst?: string } {
  const ranked = SCORE_AXIS_ORDER.filter((a) => typeof s.breakdown?.[a] === "number").sort(
    (a, b) => (s.breakdown[b] ?? 0) - (s.breakdown[a] ?? 0),
  );
  return { best: ranked[0], worst: ranked[ranked.length - 1] };
}

function stockHref(symbol: string): string {
  return `/stocks/${encodeURIComponent(symbol)}`;
}

function FeaturedCard({ s }: { s: StockScoreData }) {
  const tone = scoreTone(s.score ?? 0);
  const { best, worst } = extremes(s);
  return (
    <Link
      href={stockHref(s.symbol)}
      className="flex w-40 shrink-0 flex-col rounded-xl border border-[var(--border)] bg-[var(--card)] p-2.5 transition-colors hover:border-[var(--brand)]/60 sm:w-auto"
    >
      <div className="flex items-baseline justify-between gap-1">
        <span className="truncate text-cell font-semibold text-[var(--text-primary)]">{s.name ?? s.symbol}</span>
        <span className="shrink-0 text-micro text-[var(--text-muted)]">{s.symbol}</span>
      </div>
      <div className="flex items-baseline gap-1">
        <span className={cn("text-xl font-bold tabular-nums", SCORE_TONE_TEXT[tone])}>{s.score}</span>
        {s.grade && <span className={cn("text-micro font-semibold", SCORE_TONE_TEXT[tone])}>{s.grade}</span>}
      </div>
      <ScoreRadar breakdown={s.breakdown} labels={false} className="mx-auto my-1 h-20 w-20" />
      <p className="truncate text-micro text-[var(--text-muted)]">
        강 {best ? SCORE_AXIS_LABELS[best as keyof typeof SCORE_AXIS_LABELS] : "-"} · 약{" "}
        {worst ? SCORE_AXIS_LABELS[worst as keyof typeof SCORE_AXIS_LABELS] : "-"}
      </p>
    </Link>
  );
}

function pct(v: number, unit = "%"): string {
  return `${v > 0 ? "+" : v < 0 ? "−" : ""}${Math.abs(v).toFixed(2)}${unit}`;
}

export function HomeGuestShowcase({ data }: { data: GuestShowcase }) {
  if (data.featured.length === 0) return null;
  // 히어로 카드 = 대표 종목 가운데 점수가 가장 높은 종목(오늘의 얼굴).
  const hero = [...data.featured].sort((a, b) => (b.score ?? 0) - (a.score ?? 0))[0]!;
  const heroEx = extremes(hero);
  const heroGroup = peerGroupLabel(hero.peerGroup);
  const bt = data.backtest;

  return (
    <GuestOnly>
      <section aria-label="Quant Port 소개" className="space-y-3">
        <div className="grid items-center gap-5 rounded-2xl bg-gradient-to-br from-slate-900 to-blue-900 px-5 py-6 text-white sm:px-7 lg:grid-cols-[1.15fr_0.85fr]">
          <div className="min-w-0">
            <p className="text-micro font-semibold text-blue-300">국내·미국 전 종목 · 매일 00:25 갱신 · 투자 권유 아님</p>
            <h2 className="mt-1 text-2xl font-bold leading-snug tracking-tight sm:text-[28px]">
              이 종목, 같은 업종 안에서
              <br />몇 등일까요?
            </h2>
            <p className="mt-2 max-w-xl text-cell leading-relaxed text-slate-300">
              수익성·성장·가치·추세·수급 다섯 축으로 매긴 AI 점수와 그 근거를 보여 드립니다. 점수가 바뀌면 왜 바뀌었는지도.
            </p>
            <div className="mt-4 max-w-xl">
              <GuestHeroSearch />
            </div>
            <ul className="mt-3 flex flex-wrap gap-1.5">
              {data.featured.map((s) => (
                <li key={s.symbol}>
                  <Link
                    href={stockHref(s.symbol)}
                    className="rounded-full border border-white/30 px-2.5 py-0.5 text-cell text-slate-200 hover:bg-white/10"
                  >
                    {s.name ?? s.symbol}
                  </Link>
                </li>
              ))}
            </ul>
          </div>
          <Link
            href={stockHref(hero.symbol)}
            // 모바일에서는 뺀다 — 첫 화면을 혼자 다 차지했고, 아래 대표 종목 카드가 같은 일을 한다.
            className="hidden items-center gap-3 rounded-xl border border-white/15 bg-white/5 p-3 hover:bg-white/10 sm:flex"
          >
            <div style={HERO_RADAR_VARS} className="shrink-0">
              <ScoreRadar breakdown={hero.breakdown} className="h-40 w-48" />
            </div>
            <div className="min-w-0">
              <p className="text-micro text-blue-300">
                {hero.name} · {hero.symbol}
              </p>
              <p className="flex items-baseline gap-1.5">
                <span className="text-5xl font-extrabold tabular-nums leading-none">{hero.score}</span>
                <span className="font-bold text-blue-300">{hero.grade}</span>
              </p>
              {typeof hero.peerPct === "number" && (
                <p className="mt-1 text-micro text-slate-300">
                  {heroGroup ? `${heroGroup} 중 ` : "동종 "}상위 {hero.peerPct}%
                </p>
              )}
              <p className="mt-1 text-micro leading-relaxed text-slate-300">
                가장 강한 축{" "}
                <b className="text-white">
                  {heroEx.best ? `${SCORE_AXIS_LABELS[heroEx.best as keyof typeof SCORE_AXIS_LABELS]} ${Math.round(hero.breakdown[heroEx.best as keyof typeof hero.breakdown] ?? 0)}` : "-"}
                </b>
                <br />
                가장 약한 축{" "}
                <b className="text-white">
                  {heroEx.worst ? `${SCORE_AXIS_LABELS[heroEx.worst as keyof typeof SCORE_AXIS_LABELS]} ${Math.round(hero.breakdown[heroEx.worst as keyof typeof hero.breakdown] ?? 0)}` : "-"}
                </b>
              </p>
            </div>
          </Link>
        </div>

        {/* 대표 종목 — 넓으면 6칸, 좁으면 가로로 밀어 본다. */}
        <div className="-mx-1 flex gap-2.5 overflow-x-auto px-1 pb-1 sm:mx-0 sm:grid sm:grid-cols-3 sm:overflow-visible sm:px-0 lg:grid-cols-6">
          {data.featured.map((s) => (
            <FeaturedCard key={s.symbol} s={s} />
          ))}
        </div>

        <div className="grid gap-3 lg:grid-cols-[1.25fr_0.75fr]">
          <Card className="p-3.5">
            <GuestMovers domestic={data.movers.domestic} us={data.movers.us} date={data.movers.date} />
          </Card>
          {data.earnings.length > 0 && (
            <Card className="p-3.5">
              <div className="flex items-center gap-1.5">
                <CalendarClock className="h-3.5 w-3.5 text-[var(--brand)]" aria-hidden />
                <h2 className="text-cell font-semibold text-[var(--text-primary)]">다가오는 주요 실적</h2>
              </div>
              <ul className="mt-2 divide-y divide-[var(--border)]">
                {data.earnings.map((e) => {
                  const days = earningsDaysFromToday(e.date);
                  return (
                    <li key={e.symbol}>
                      <Link href={stockHref(e.symbol)} className="flex items-center gap-2 py-2 text-cell hover:bg-[var(--surface-elevated)]/60">
                        <span className="w-11 shrink-0 rounded-md bg-[var(--brand)]/10 text-center text-micro font-semibold text-[var(--brand)]">
                          {days == null ? e.date.slice(5) : days === 0 ? "오늘" : `D-${days}`}
                        </span>
                        <span className="min-w-0 flex-1 truncate font-medium text-[var(--text-primary)]">{e.name}</span>
                        <span className="shrink-0 text-micro text-[var(--text-muted)]">
                          {e.period}
                          {e.score != null ? ` · 점수 ${e.score}` : ""}
                        </span>
                      </Link>
                    </li>
                  );
                })}
              </ul>
              <Link href="/earnings/us" className="mt-2 inline-block text-micro text-[var(--brand)] hover:underline">
                실적 일정 전체 →
              </Link>
            </Card>
          )}
        </div>

        <div className="grid gap-3 md:grid-cols-3">
          <Card className="p-3.5">
            <Layers className="h-4 w-4 text-[var(--brand)]" aria-hidden />
            <h3 className="mt-1.5 text-cell font-semibold text-[var(--text-primary)]">같은 업종 안에서 비교</h3>
            <p className="mt-1 text-micro leading-relaxed text-[var(--text-muted)]">
              수익성·성장·가치는 같은 업종끼리, 종합 점수는 같은 시장·비슷한 규모끼리 비교합니다. 「PER이 낮다」가 업종 탓인지 회사 탓인지 가립니다.
            </p>
          </Card>
          <Card className="p-3.5">
            <GitCompareArrows className="h-4 w-4 text-[var(--brand)]" aria-hidden />
            <h3 className="mt-1.5 text-cell font-semibold text-[var(--text-primary)]">점수가 바뀐 이유를 분해</h3>
            <p className="mt-1 text-micro leading-relaxed text-[var(--text-muted)]">
              어제 대비 변화를 축별로 남김없이 나눕니다. 데이터가 비어서 생긴 변동은 사건으로 올리지 않습니다.
            </p>
          </Card>
          <Card className="p-3.5">
            <LineChart className="h-4 w-4 text-[var(--brand)]" aria-hidden />
            <h3 className="mt-1.5 text-cell font-semibold text-[var(--text-primary)]">점수의 과거 성적 공개</h3>
            {bt ? (
              <p className="mt-1 text-micro leading-relaxed text-[var(--text-muted)]">
                국내 종합 점수 상위 20종목을 매주 바꿔 담았다면 {bt.fromDate.slice(5)}~{bt.toDate.slice(5)}{" "}
                <b className={bt.totalReturn >= 0 ? "text-[var(--score-high)]" : "text-[var(--score-low)]"}>{pct(bt.totalReturn)}</b>
                {bt.benchmarks.map((b) => (
                  <span key={b.label}>
                    {" · "}
                    {b.label} 대비{" "}
                    <b className={b.alpha >= 0 ? "text-[var(--score-high)]" : "text-[var(--score-low)]"}>{pct(b.alpha, "%p")}</b>
                  </span>
                ))}
                . 좋을 때도 나쁠 때도 그대로 보여 줍니다.{" "}
                <Link href="/analysis?backtest=1&market=domestic" className="text-[var(--brand)] hover:underline">
                  성적 보기 →
                </Link>
              </p>
            ) : (
              <p className="mt-1 text-micro leading-relaxed text-[var(--text-muted)]">
                상위 종목을 샀다면 어땠는지 지수와 비교해 그대로 보여 줍니다.{" "}
                <Link href="/analysis?backtest=1&market=domestic" className="text-[var(--brand)] hover:underline">
                  성적 보기 →
                </Link>
              </p>
            )}
          </Card>
        </div>
      </section>
    </GuestOnly>
  );
}
