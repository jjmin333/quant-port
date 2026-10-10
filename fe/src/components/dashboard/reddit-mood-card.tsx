"use client";

import { useEffect, useState } from "react";
import { Sparkles } from "lucide-react";
import { PostCard, RedditMark, ScoreGauge, scoreColor } from "@/components/community/stock-reddit-panel";
import { SectionCard } from "@/components/ui/section-card";
import { fetchMarketMood, type MarketMood } from "@/lib/stock-signals-api";
import { cn } from "@/lib/utils";

/**
 * 홈 「해외 Reddit 커뮤니티」 (2026-10-01 요청 3 · 10-09 요청 8 · 10-09 2차 요청 6 개편).
 *
 *   왼쪽(넓게)  AI 요약 · 낙관 근거 | 주의할 점 · 태그
 *               분위기 점수·지수 ETF 감성 | 언급·급증·서브레딧·추이
 *   오른쪽      오늘의 인기 글 — 왼쪽 높이를 그대로 채운다(넘치면 칸 안에서 스크롤)
 *
 * 예전(요약 전폭 + 아래 3열)은 셋째 칸 인기 글이 2개뿐이라 아래가 비었다. 오른쪽 칸을 통째로 인기 글에 주고
 * 높이는 왼쪽이 정하게 해(lg:absolute) 빈자리가 생기지 않는다. 좁은 화면은 위아래로 쌓고 글은 4개.
 *
 * 전부 배치가 채운 저장분이다(ApeWisdom 00:05 · 시장 인기 글 00:30 · AI 요약 04:20) — 하루 한 번 바뀌므로
 * 폴링하지 않는다. 자료가 없으면 카드를 그리지 않는다. 출처·면책 꼬리말은 걷었다(요청 8·9) —
 * 기준일은 요약 머리에 남긴다.
 */
export function RedditMoodCard({ onPick }: { onPick?: (symbol: string) => void }) {
  const [mood, setMood] = useState<MarketMood | null>(null);
  useEffect(() => {
    let alive = true;
    fetchMarketMood()
      .then((m) => alive && setMood(m))
      .catch(() => {});
    return () => {
      alive = false;
    };
  }, []);
  if (!mood) return null;

  const delta = mood.mentionsPrev > 0 ? Math.round(((mood.mentions - mood.mentionsPrev) / mood.mentionsPrev) * 100) : null;
  const maxMentions = Math.max(1, ...mood.history.map((d) => d.mentions));
  const etfs = mood.etfs.filter((e) => e.sentimentPct != null);
  const summary = mood.summary;
  const stamp = (mood.summaryDate ?? mood.date).slice(5).replace("-", "/");

  return (
    <SectionCard
      title={
        <span className="inline-flex items-center gap-1.5">
          <RedditMark />
          해외 Reddit 커뮤니티
        </span>
      }
    >
      <div className="grid min-w-0 gap-4 lg:grid-cols-[minmax(0,1.7fr)_minmax(0,1fr)]">
        <div className="min-w-0 space-y-4">
          {/* ── 왼쪽 위: AI 요약 ── */}
          {summary?.summary ? (
            <div className="rounded-xl border border-[var(--border)] border-l-4 border-l-[#8B5CF6]/60 bg-[var(--surface)] p-3">
              <div className="flex flex-wrap items-center gap-1.5">
                <span className="inline-flex items-center gap-0.5 rounded-full bg-[#8B5CF6] px-2 py-0.5 text-micro font-bold text-white">
                  <Sparkles className="h-3 w-3" aria-hidden />
                  {mood.summaryAi ? "AI 요약" : "요약"}
                </span>
                {summary.stance && (
                  <span className="text-cell font-semibold" style={mood.score != null ? { color: scoreColor(mood.score) } : undefined}>
                    {summary.stance}
                  </span>
                )}
                <span className="ml-auto text-micro text-[var(--text-muted)]">{stamp} 기준</span>
              </div>
              <p className="mt-2 text-read leading-relaxed text-[var(--text-primary)]">{summary.summary}</p>
              {((summary.bull?.length ?? 0) > 0 || (summary.bear?.length ?? 0) > 0) && (
                <div className="mt-2.5 grid gap-2 sm:grid-cols-2">
                  <Reasons title="낙관 근거" items={summary.bull} color="var(--profit)" />
                  <Reasons title="주의할 점" items={summary.bear} color="var(--loss)" />
                </div>
              )}
              {summary.tags.length > 0 && (
                <div className="mt-2.5 flex flex-wrap gap-1.5">
                  {summary.tags.map((t) => (
                    <span key={t} className="rounded-full border border-[var(--brand)]/20 bg-[var(--brand)]/5 px-2 py-0.5 text-micro font-semibold text-[var(--brand)]">
                      #{t}
                    </span>
                  ))}
                </div>
              )}
            </div>
          ) : (
            <p className="text-cell text-[var(--text-muted)]">오늘 요약은 아직 없습니다.</p>
          )}

          {/* ── 왼쪽 아래: 점수 | 버즈 ── */}
          <div className="grid min-w-0 gap-4 sm:grid-cols-2">
            <div className="min-w-0 space-y-2">
              <ColumnTitle>분위기 점수</ColumnTitle>
              {mood.score != null ? (
                <div
                  className="flex items-center gap-3"
                  title="지수 ETF(SPY·QQQ·IWM·DIA) 감성 60% + 언급 상위 종목 전체 40%, 보통 55% 기준(-100~100)"
                >
                  <span className="text-2xl font-extrabold tabular-nums" style={{ color: scoreColor(mood.score) }}>
                    {mood.score > 0 ? `+${mood.score}` : mood.score}
                  </span>
                  <span className="text-note font-semibold" style={{ color: scoreColor(mood.score) }}>
                    {mood.stance}
                  </span>
                  <ScoreGauge score={mood.score} />
                </div>
              ) : (
                <p className="text-cell text-[var(--text-muted)]">표본이 적어 오늘 점수는 쉽니다.</p>
              )}
              {etfs.length > 0 && (
                <ul className="space-y-1">
                  {etfs.map((e) => (
                    <li key={e.symbol} className="flex items-center gap-2 text-cell" title={`${e.symbol} — 언급 ${e.mentions}회`}>
                      <b className="w-9 shrink-0">{e.symbol}</b>
                      <span className="h-1.5 flex-1 overflow-hidden rounded-full bg-[var(--surface-elevated)]">
                        <span
                          className="block h-full rounded-full"
                          style={{ width: `${e.sentimentPct}%`, background: scoreColor(((e.sentimentPct ?? 55) - 55) * 2.2) }}
                        />
                      </span>
                      <span className="shrink-0 whitespace-nowrap text-right font-num text-micro tabular-nums text-[var(--text-secondary)]">
                        긍정 {e.sentimentPct}%{e.users != null && <span className="text-[var(--text-muted)]"> · {e.users}명</span>}
                      </span>
                    </li>
                  ))}
                </ul>
              )}
            </div>

            <div className="min-w-0 space-y-2 text-cell">
              <ColumnTitle>버즈</ColumnTitle>
              <div className="grid grid-cols-2 gap-2">
                <Stat label="24시간 언급">
                  <b className="tabular-nums">{mood.mentions.toLocaleString()}</b>
                  {delta != null && (
                    <span className={cn("ml-1 text-micro tabular-nums", delta >= 0 ? "text-[var(--profit)]" : "text-[var(--loss)]")}>
                      {delta >= 0 ? "+" : ""}
                      {delta}%
                    </span>
                  )}
                </Stat>
                <Stat label="낙관 : 비관 종목" hint="언급 사용자 5명 이상 종목 중 커뮤니티 점수 +25 이상 / -25 이하">
                  <b className="tabular-nums text-[var(--profit)]">{mood.bulls}</b> :{" "}
                  <b className="tabular-nums text-[var(--loss)]">{mood.bears}</b>
                </Stat>
              </div>
              {mood.surges.length > 0 && (
                <div className="flex flex-wrap items-center gap-1">
                  <span className="text-micro text-[var(--text-muted)]">언급 급증</span>
                  {mood.surges.map((s) => (
                    <button
                      key={s.symbol}
                      type="button"
                      onClick={onPick ? () => onPick(s.symbol) : undefined}
                      title={`${s.name} — ${s.mentionsPrev}회 → ${s.mentions}회`}
                      className="rounded-full bg-[var(--brand)]/10 px-2 py-0.5 text-micro font-semibold text-[var(--brand)] hover:bg-[var(--brand)]/20"
                    >
                      {s.symbol} ×{(s.mentions / Math.max(1, s.mentionsPrev)).toFixed(1)}
                    </button>
                  ))}
                </div>
              )}
              {mood.subreddits.length > 0 && (
                <div className="flex flex-wrap items-center gap-1">
                  <span className="text-micro text-[var(--text-muted)]">서브레딧</span>
                  {mood.subreddits.slice(0, 5).map((s) => (
                    <span key={s.source} className="rounded-full border border-[var(--border)] px-2 py-0.5 text-micro tabular-nums text-[var(--text-secondary)]">
                      {s.source.replace(/^r\//, "")} {s.mentions.toLocaleString()}
                    </span>
                  ))}
                </div>
              )}
              {mood.history.length > 1 && (
                <div
                  className="flex h-14 items-end gap-[3px]"
                  role="img"
                  aria-label={`최근 ${mood.history.length}일 언급 추이`}
                  title={`최근 ${mood.history.length}일 언급 — 막대 높이는 언급 수, 색은 그날 분위기`}
                >
                  {mood.history.map((d) => (
                    <span
                      key={d.date}
                      title={`${d.date} 언급 ${d.mentions.toLocaleString()}${d.score != null ? ` · 분위기 ${d.score}` : ""}`}
                      className="flex-1 rounded-t-sm"
                      style={{
                        height: `${Math.max(8, (d.mentions / maxMentions) * 100)}%`,
                        background: d.score != null ? scoreColor(d.score) : "var(--border)",
                        opacity: 0.6,
                      }}
                    />
                  ))}
                </div>
              )}
            </div>

          </div>
        </div>

        {/* ── 오른쪽: 오늘의 인기 글 ── */}
        <div className="relative min-w-0">
          <ColumnTitle>오늘의 인기 글</ColumnTitle>
          <div className="mt-2 space-y-2 lg:absolute lg:inset-x-0 lg:bottom-0 lg:top-6 lg:mt-0 lg:overflow-y-auto lg:pr-1">
            {mood.posts.length > 0 ? (
              mood.posts.map((p, i) => (
                <div key={p.id} className={cn(i >= 4 && "max-lg:hidden")}>
                  <PostCard post={p} rank={i + 1} />
                </div>
              ))
            ) : (
              <p className="text-cell text-[var(--text-muted)]">아직 모은 글이 없습니다.</p>
            )}
          </div>
        </div>
      </div>
    </SectionCard>
  );
}

function ColumnTitle({ children }: { children: React.ReactNode }) {
  return <p className="text-cell font-semibold text-[var(--text-muted)]">{children}</p>;
}

function Stat({ label, hint, children }: { label: string; hint?: string; children: React.ReactNode }) {
  return (
    <div className="rounded-lg bg-[var(--surface-elevated)] px-2.5 py-1.5" title={hint}>
      <p className="text-micro text-[var(--text-muted)]">{label}</p>
      <p className="mt-0.5 text-note">{children}</p>
    </div>
  );
}

function Reasons({ title, items, color }: { title: string; items?: string[]; color: string }) {
  if (!items || items.length === 0) return null;
  return (
    <div className="rounded-lg bg-[var(--surface-elevated)]/60 px-2.5 py-2">
      <p className="mb-1 text-cell font-semibold" style={{ color }}>
        {title}
      </p>
      <ul className="list-disc space-y-0.5 pl-4 text-cell leading-relaxed text-[var(--text-secondary)]">
        {items.slice(0, 4).map((t) => (
          <li key={t}>{t}</li>
        ))}
      </ul>
    </div>
  );
}
