import type { CSSProperties } from "react";
import Link from "next/link";
import { ScoreRadar } from "@/components/analysis/score-radar";
import { GuestHeroSearch, GuestOnly } from "@/components/dashboard/home-guest-client";
import type { GuestShowcase } from "@/lib/home-guest.server";
import { peerGroupLabel } from "@/lib/score-explain-api";
import {
  SCORE_AXIS_LABELS,
  SCORE_AXIS_ORDER,
  type StockScoreData,
} from "@/lib/stock-score-api";

/*
홈 비로그인 블록 (2026-10-10 사용자 결정 — 시안 `docs/mockups/home-guest-v2.html` A안, 대표 종목 고정, 성적 숫자 공개).

처음 온 사람이 3초 안에 「무엇을 하는 서비스인지」 알게 한다: 질문 하나(같은 업종 안에서 몇 등?) →
검색 → 이름을 아는 종목의 점수 모양. 대표 종목 카드·오늘 바뀐 종목·다가오는 실적·소개 카드 셋은
10-11 사용자 요청으로 뺐다. 로그인한 사람에게는 그리지 않는다(지금 홈 그대로). 서버가 그리므로 크롤러도 같은 본문을 본다.
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

export function HomeGuestShowcase({ data }: { data: GuestShowcase }) {
  if (data.featured.length === 0) return null;
  // 히어로 카드 = 대표 종목 가운데 점수가 가장 높은 종목(오늘의 얼굴).
  const hero = [...data.featured].sort((a, b) => (b.score ?? 0) - (a.score ?? 0))[0]!;
  const heroEx = extremes(hero);
  const heroGroup = peerGroupLabel(hero.peerGroup);

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
            // 모바일에서는 뺀다 — 첫 화면을 혼자 다 차지한다. 위 종목 칩이 같은 길을 낸다.
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
      </section>
    </GuestOnly>
  );
}
