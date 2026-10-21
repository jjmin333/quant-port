"use client";

import { useState } from "react";
import { ChevronDown, ExternalLink, Languages, MessageSquare, Sparkles } from "lucide-react";
import { PageNav } from "@/components/ui/page-nav";
import {
  type CommunityBody,
  type RedditPost,
  type SocialDay,
  type SocialSubreddit,
  type StockSignals,
} from "@/lib/stock-signals-api";
import { cn } from "@/lib/utils";

/**
 * 해외 종목의 「레딧」 탭 (2026-09-29 사용자 요청 — 첨부 화면 구성, 10-09부터 커뮤니티 탭에서 떼어 낸 제 탭).
 *
 *   머리글      커뮤니티 점수(-100..100, 규칙 — ApeWisdom 감성을 표본 크기로 줄인 값) + 게이지
 *   AI 핵심 요약 배치가 쓴 글(언급 상위 40종목) · 태그 · 「전체 분석 보기」(낙관·주의 근거, 키워드, 서브레딧)
 *   Reddit 버즈  24시간 순위·언급·추천 수·긍정 비율 + 14일 막대
 *   인기 글      이번 주 인기 Reddit 글(검색 RSS) — 번역이 있으면 한국어가 기본, 아이콘으로 원문
 *
 * 전부 배치가 채운 표를 읽는다. 자료는 워크스페이스가 받아 넘긴다 — 같은 응답으로 탭을 세울지도 정한다.
 */

const REDDIT = "#FF4500";
const POSTS_PER_PAGE = 5;

export function StockRedditPanel({ symbol, signals }: { symbol: string; signals: StockSignals }) {
  if (signals.social.length === 0) return null;
  const reddit = signals.reddit;

  return (
    // 종목을 옮기면 접힘·쪽 상태를 처음으로 — 다른 종목의 3쪽을 보고 있지 않게.
    <div key={symbol} className="space-y-3">
      <CommunityHeader score={reddit?.community?.score ?? null} stance={reddit?.community?.stance ?? null} />
      {reddit?.community?.body && (
        <AISummaryCard
          body={reddit.community.body}
          source={reddit.community.source}
          date={reddit.community.bodyDate ?? reddit.community.date}
          keywords={reddit.insight?.keywords ?? []}
          subreddits={signals.subreddits}
        />
      )}
      <BuzzCard days={signals.social} users={reddit?.insight?.users} sentimentPct={reddit?.insight?.sentimentPct} />
      {reddit && reddit.posts.length > 0 && <PostList posts={reddit.posts} />}
    </div>
  );
}

/**
 * 개요 맨 아래의 레딧 카드 (2026-10-13 요청 6, 시안 `docs/mockups/stock-overview-v2.html` ⑥) —
 * 점수 · 24시간 숫자 셋 · 요약 문단 · 인기 글 2건만, 나머지는 「레딧」 탭으로. 자료는 탭과 같은 응답.
 */
export function StockRedditOverviewCard({ signals, onOpen }: { signals: StockSignals; onOpen?: () => void }) {
  const last = signals.social[0];
  if (!last) return null;
  const reddit = signals.reddit;
  const community = reddit?.community;
  const insight = reddit?.insight;
  return (
    <div className="overflow-hidden rounded-xl border border-[var(--border)] bg-[var(--surface)]">
      <div className="flex flex-wrap items-center gap-x-2 gap-y-0.5 border-b border-[var(--border)] px-3 py-2">
        <RedditMark />
        <h3 className="text-cell font-semibold text-[var(--text-primary)]">레딧 분위기</h3>
        {community && (
          <span
            className="text-note font-bold tabular-nums"
            style={{ color: scoreColor(community.score) }}
            title="커뮤니티 점수 — 언급한 사용자들의 긍정 비율을 표본 크기로 보정한 값(-100~100)"
          >
            {community.score > 0 ? `+${community.score}` : community.score}{" "}
            <span className="text-micro font-semibold">{community.stance}</span>
          </span>
        )}
        <span className="ml-auto text-micro text-[var(--text-muted)]">
          {last.date.slice(5).replace("-", "/")} · ApeWisdom 24시간
        </span>
      </div>
      <div className="space-y-2 px-3 py-2.5">
        <dl className="grid grid-cols-3 gap-1.5">
          <OverviewStat label="언급" value={`${last.mentions.toLocaleString()}회`} extra={<Delta now={last.mentions} prev={last.mentionsPrev} pct />} />
          <OverviewStat label="언급 순위" value={`#${last.rank}`} extra={<Delta now={last.rank} prev={last.rankPrev} />} />
          <OverviewStat
            label="긍정 비율"
            value={insight?.sentimentPct != null ? `${insight.sentimentPct}%` : "—"}
            extra={insight?.users != null ? <span className="text-micro text-[var(--text-muted)]">{insight.users}명</span> : null}
          />
        </dl>
        {community?.body?.summary && (
          <p className="text-cell leading-relaxed text-[var(--text-secondary)]">{community.body.summary}</p>
        )}
        {reddit?.posts.slice(0, 2).map((p, i) => <PostCard key={p.id} post={p} rank={i + 1} />)}
      </div>
      {onOpen && (
        <button
          type="button"
          onClick={onOpen}
          className="flex w-full items-center justify-center gap-0.5 border-t border-[var(--border)] px-3 py-1.5 text-cell font-medium text-[var(--brand)] hover:bg-[var(--brand)]/5"
        >
          레딧 탭에서 전체 보기 ›
        </button>
      )}
    </div>
  );
}

function OverviewStat({ label, value, extra }: { label: string; value: string; extra?: React.ReactNode }) {
  return (
    <div className="min-w-0 rounded-md border border-[var(--border)] px-2 py-1">
      <dt className="truncate text-micro text-[var(--text-muted)]">{label}</dt>
      <dd className="flex items-baseline gap-1">
        <span className="text-note font-bold tabular-nums text-[var(--text-primary)]">{value}</span>
        {extra}
      </dd>
    </div>
  );
}

export function scoreColor(score: number): string {
  if (score >= 25) return "var(--profit)";
  if (score <= -25) return "var(--loss)";
  return "var(--score-mid)";
}

function CommunityHeader({ score, stance }: { score: number | null; stance: string | null }) {
  return (
    <div className="flex flex-wrap items-start justify-between gap-3 px-1">
      <div className="flex min-w-0 flex-1 items-start gap-3">
        <span className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl border border-[var(--brand)]/20 bg-[var(--brand)]/10 text-[var(--brand)]">
          <MessageSquare className="h-4 w-4" aria-hidden />
        </span>
        <h3 className="min-w-0 self-center text-base font-bold text-[var(--text-primary)]">Reddit 커뮤니티 분위기</h3>
      </div>
      {score != null && (
        <div
          className="flex items-center gap-3"
          title="커뮤니티 점수 — 언급한 사용자들의 긍정 비율을 표본 크기로 보정한 값(-100~100)"
        >
          <span className="text-2xl font-extrabold tabular-nums" style={{ color: scoreColor(score) }}>
            {score > 0 ? `+${score}` : score}
          </span>
          <span className="text-note font-semibold" style={{ color: scoreColor(score) }}>
            {stance}
          </span>
          <ScoreGauge score={score} />
        </div>
      )}
    </div>
  );
}

/** -100..100을 막대 10칸에. 가운데(0)에서 점수 쪽으로 칠한다. */
export function ScoreGauge({ score }: { score: number }) {
  const bars = 10;
  const pos = Math.min(bars - 1, Math.floor(((score + 100) / 200) * bars));
  const mid = bars / 2;
  return (
    <span
      className="relative flex h-10 items-end gap-[3px] rounded-lg bg-[var(--surface-elevated)] px-1.5 py-1"
      aria-hidden
    >
      {Array.from({ length: bars }, (_, i) => {
        const on = score >= 0 ? i >= mid && i <= pos : i <= mid - 1 && i >= pos;
        return (
          <span
            key={i}
            className="h-full w-2 rounded-sm bg-[var(--border)]"
            style={
              on
                ? {
                    background: scoreColor(score),
                    opacity: i === pos ? 1 : 0.45,
                  }
                : undefined
            }
          />
        );
      })}
      <span className="absolute left-1/2 top-0.5 bottom-0.5 w-px -translate-x-1/2 bg-[var(--text-muted)]" />
    </span>
  );
}

function AISummaryCard({
  body,
  source,
  date,
  keywords,
  subreddits,
}: {
  body: CommunityBody;
  source?: string;
  date: string;
  keywords: { word: string; n: number }[];
  subreddits: SocialSubreddit[];
}) {
  const [open, setOpen] = useState(false);
  const subTotal = subreddits.reduce((a, s) => a + s.mentions, 0);
  return (
    <div className="overflow-hidden rounded-xl border border-[var(--border)] border-l-4 border-l-[var(--brand)]/60 bg-[var(--surface)] shadow-sm">
      <div className="flex items-center gap-2 px-4 pt-3">
        <span className="inline-flex items-center gap-0.5 rounded-full bg-[var(--brand)] px-2 py-0.5 text-micro font-bold text-[var(--brand-foreground)]">
          <Sparkles className="h-3 w-3" aria-hidden />
          AI
        </span>
        <span className="text-note font-semibold text-[var(--text-primary)]">핵심 요약</span>
        {source === "rule" && (
          <span className="rounded-full bg-[var(--surface-elevated)] px-2 py-0.5 text-micro text-[var(--text-muted)]">
            규칙 문장
          </span>
        )}
        <span className="ml-auto text-micro text-[var(--text-muted)]">{date.slice(5).replace("-", "/")} 기준</span>
      </div>
      <p className="px-4 pt-2 text-note leading-relaxed text-[var(--text-secondary)]">{body.summary}</p>
      {body.tags.length > 0 && (
        <div className="flex flex-wrap gap-1.5 px-4 pt-2.5">
          {body.tags.map((t) => (
            <span
              key={t}
              className="rounded-full border border-[var(--brand)]/20 bg-[var(--brand)]/5 px-2 py-0.5 text-micro font-semibold text-[var(--brand)]"
            >
              #{t}
            </span>
          ))}
        </div>
      )}
      <div className="p-3">
        <button
          type="button"
          onClick={() => setOpen((v) => !v)}
          aria-expanded={open}
          className="flex w-full items-center justify-between rounded-lg border border-[var(--border)] bg-[var(--surface-elevated)]/60 px-3 py-2 text-cell font-semibold text-[var(--brand)] hover:bg-[var(--surface-elevated)]"
        >
          {open ? "접기" : "전체 분석 보기"}
          <ChevronDown className={cn("h-4 w-4 transition-transform", open && "rotate-180")} aria-hidden />
        </button>
        {open && (
          <div className="grid gap-3 px-1 pt-3 text-cell sm:grid-cols-2">
            <ReasonList title="낙관 쪽 근거" items={body.bull} color="var(--profit)" />
            <ReasonList title="주의할 점" items={body.bear} color="var(--loss)" />
            {keywords.length > 0 && (
              <div>
                <p className="mb-1 font-semibold text-[var(--text-primary)]">함께 나온 낱말</p>
                <p className="flex flex-wrap gap-1">
                  {keywords.map((k) => (
                    <span
                      key={k.word}
                      className="rounded bg-[var(--surface-elevated)] px-1.5 py-0.5 text-micro text-[var(--text-secondary)]"
                    >
                      {k.word} <span className="tabular-nums text-[var(--text-muted)]">×{k.n}</span>
                    </span>
                  ))}
                </p>
              </div>
            )}
            {subTotal > 0 && (
              <div>
                <p className="mb-1 font-semibold text-[var(--text-primary)]">어디서 이야기하나</p>
                <ul className="space-y-1">
                  {subreddits.slice(0, 5).map((s) => (
                    <li key={s.source} className="flex items-center gap-2">
                      <span className="w-28 shrink-0 truncate text-[var(--text-secondary)]">{s.source}</span>
                      <span className="h-1.5 flex-1 overflow-hidden rounded-full bg-[var(--surface-elevated)]">
                        <span
                          className="block h-full rounded-full"
                          style={{
                            width: `${(s.mentions / subTotal) * 100}%`,
                            background: REDDIT,
                          }}
                        />
                      </span>
                      <span className="w-9 text-right tabular-nums text-[var(--text-muted)]">
                        {Math.round((s.mentions / subTotal) * 100)}%
                      </span>
                    </li>
                  ))}
                </ul>
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  );
}

function ReasonList({ title, items, color }: { title: string; items?: string[]; color: string }) {
  if (!items || items.length === 0) return null;
  return (
    <div>
      <p className="mb-1 font-semibold" style={{ color }}>
        {title}
      </p>
      <ul className="list-disc space-y-0.5 pl-4 text-[var(--text-secondary)]">
        {items.map((t) => (
          <li key={t}>{t}</li>
        ))}
      </ul>
    </div>
  );
}

function Delta({ now, prev, pct }: { now: number; prev?: number; pct?: boolean }) {
  if (prev == null || prev === now) return null;
  if (pct) {
    if (prev <= 0) return <span className="text-micro font-semibold text-[var(--profit)]">신규</span>;
    const r = Math.round(((now - prev) / prev) * 100);
    return (
      <span
        className="text-micro font-semibold tabular-nums"
        style={{ color: r > 0 ? "var(--profit)" : "var(--loss)" }}
      >
        {r > 0 ? "▲" : "▼"}
        {Math.abs(r).toLocaleString()}%
      </span>
    );
  }
  // 순위는 작을수록 위 — 전일 15위 → 오늘 3위는 ▲12.
  const up = prev - now;
  return (
    <span className="text-micro font-semibold tabular-nums" style={{ color: up > 0 ? "var(--profit)" : "var(--loss)" }}>
      {up > 0 ? "▲" : "▼"}
      {Math.abs(up)}
    </span>
  );
}

function BuzzCard({ days, users, sentimentPct }: { days: SocialDay[]; users?: number; sentimentPct?: number }) {
  const last = days[0];
  const recent = days.slice(0, 14).reverse();
  const max = Math.max(1, ...recent.map((d) => d.mentions));
  const surge = last.mentionsPrev > 0 && last.mentions / last.mentionsPrev >= 3;
  return (
    <div className="rounded-xl border border-[var(--border)] bg-[var(--surface-elevated)]/50 px-4 py-3">
      <div className="flex flex-wrap items-center gap-2">
        <RedditMark />
        <span className="text-note font-bold text-[var(--text-primary)]">Reddit 버즈</span>
        {surge && (
          <span className="rounded-full bg-[#FF4500] px-2 py-0.5 text-micro font-bold text-white">언급 급증</span>
        )}
        <span className="ml-auto flex h-6 items-end gap-px" title="최근 14일 언급 수" aria-label="최근 14일 언급 수">
          {recent.map((d) => (
            <span
              key={d.date}
              className="w-1.5 rounded-sm"
              style={{
                height: `${Math.max(10, (d.mentions / max) * 100)}%`,
                background: `${REDDIT}99`,
              }}
              title={`${d.date} · ${d.mentions}회`}
            />
          ))}
        </span>
      </div>
      <dl className="mt-2 grid grid-cols-2 gap-y-2 text-center sm:grid-cols-4 sm:divide-x sm:divide-[var(--border)]">
        <Stat label="랭킹" value={`#${last.rank}`} extra={<Delta now={last.rank} prev={last.rankPrev} />} />
        <Stat
          label="언급 수"
          value={last.mentions.toLocaleString()}
          extra={<Delta now={last.mentions} prev={last.mentionsPrev} pct />}
        />
        <Stat label="추천 수" value={last.upvotes.toLocaleString()} hint="언급된 글·댓글이 받은 추천(업보트) 합" />
        <Stat
          label="긍정 비율"
          value={sentimentPct != null ? `${sentimentPct}%` : "—"}
          extra={users != null ? <span className="text-micro text-[var(--text-muted)]">{users}명</span> : null}
          hint="언급한 댓글 중 긍정으로 분류된 비율(ApeWisdom)"
        />
      </dl>
      <p className="mt-2 text-micro text-[var(--text-muted)]">{last.date} 기준 · ApeWisdom 24시간 집계</p>
    </div>
  );
}

function Stat({ label, value, extra, hint }: { label: string; value: string; extra?: React.ReactNode; hint?: string }) {
  return (
    <div className="px-2" title={hint}>
      <dt className="text-micro text-[var(--text-muted)]">{label}</dt>
      <dd className="mt-0.5 flex items-baseline justify-center gap-1">
        <span className="text-note font-bold tabular-nums text-[var(--text-primary)]">{value}</span>
        {extra}
      </dd>
    </div>
  );
}

export function RedditMark() {
  return (
    <span
      className="flex h-4 w-4 shrink-0 items-center justify-center rounded-full text-[9px] font-black leading-none text-white"
      style={{ background: REDDIT }}
      aria-hidden
    >
      r
    </span>
  );
}

const postDate = new Intl.DateTimeFormat("ko-KR", {
  timeZone: "Asia/Seoul",
  month: "long",
  day: "numeric",
  hour: "2-digit",
  minute: "2-digit",
});

function PostList({ posts }: { posts: RedditPost[] }) {
  const [page, setPage] = useState(1);
  const totalPages = Math.ceil(posts.length / POSTS_PER_PAGE);
  const shown = posts.slice((page - 1) * POSTS_PER_PAGE, page * POSTS_PER_PAGE);
  return (
    <div className="space-y-2">
      <p className="px-1 text-cell font-semibold text-[var(--text-secondary)]">이번 주 Reddit 인기 글</p>
      {shown.map((p, i) => (
        <PostCard key={p.id} post={p} rank={(page - 1) * POSTS_PER_PAGE + i + 1} />
      ))}
      <PageNav
        page={page}
        totalPages={totalPages}
        onChange={setPage}
        label="Reddit 글 페이지"
        className="border-t-0 pt-1"
      />
    </div>
  );
}

/**
 * Reddit 글 한 장. 한/영 전환·원문 열기는 **머리 줄 오른쪽의 작은 아이콘**이다(2026-10-09 요청 6) —
 * 아래 버튼 줄 하나가 사라져 같은 높이에 글이 더 들어간다(홈 카드는 오른쪽 칸 전체가 인기 글).
 */
export function PostCard({ post, rank }: { post: RedditPost; rank: number }) {
  const translated = Boolean(post.titleKo);
  const [original, setOriginal] = useState(false);
  const ko = translated && !original;
  const title = ko ? post.titleKo! : post.title;
  const snippet = ko && post.snippetKo ? post.snippetKo : post.snippet;
  return (
    <article className="rounded-xl border border-[var(--border)] bg-[var(--surface)] px-3 py-2.5 shadow-sm">
      <div className="flex items-center gap-1.5 text-micro text-[var(--text-muted)]">
        <RedditMark />
        <time dateTime={post.publishedAt}>{postDate.format(new Date(post.publishedAt))}</time>
        <span className="min-w-0 truncate rounded bg-[var(--surface-elevated)] px-1.5 py-0.5 font-medium text-[var(--text-secondary)]">
          r/{post.subreddit} · 인기 {rank}위
        </span>
        <span className="ml-auto flex shrink-0 items-center">
          {translated && (
            <button
              type="button"
              onClick={() => setOriginal((v) => !v)}
              aria-pressed={!original}
              title={ko ? "원문 보기" : "번역 보기"}
              className={cn(
                "rounded p-0.5 hover:bg-[var(--surface-elevated)]",
                ko ? "text-[#8B5CF6]" : "text-[var(--text-muted)]",
              )}
            >
              <Languages className="h-3.5 w-3.5" aria-hidden />
              <span className="sr-only">{ko ? "원문 보기" : "번역 보기"}</span>
            </button>
          )}
          <a
            href={post.url}
            target="_blank"
            rel="noopener noreferrer nofollow"
            title="Reddit에서 열기"
            className="rounded p-0.5 text-[var(--brand)] hover:bg-[var(--surface-elevated)]"
          >
            <ExternalLink className="h-3.5 w-3.5" aria-hidden />
            <span className="sr-only">Reddit에서 열기</span>
          </a>
        </span>
      </div>
      <a
        href={post.url}
        target="_blank"
        rel="noopener noreferrer nofollow"
        className="mt-1 block text-note font-semibold leading-snug text-[var(--text-primary)] hover:underline"
      >
        {title}
      </a>
      {snippet && <p className="mt-0.5 line-clamp-2 text-cell text-[var(--text-muted)]">{snippet}</p>}
    </article>
  );
}
