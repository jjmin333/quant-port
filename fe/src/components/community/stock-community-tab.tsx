"use client";

import { useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { FileText, Heart, MessageSquare, PenLine } from "lucide-react";
import { fetchCommunityPosts } from "@/lib/community-api";
import type { CommunityPost } from "@/lib/community-types";
import { formatRelativeTime } from "@/lib/format";
import { plainMentions } from "@/lib/mention-text";
import { mentionTokenText } from "@/lib/mentions";
import { fetchResearchCallsClient } from "@/lib/research-api";
import { DIRECTION_LABELS, STATUS_LABELS, type ResearchCall } from "@/lib/research-types";
import { cn } from "@/lib/utils";

/**
 * 종목 화면 「커뮤니티」 탭 (2026-09-28 사용자 요청 3 · 결정: 한 목록 + 칩).
 *
 * 이 종목을 **붙이거나 본문에 태그한** 커뮤니티 글과, 이 종목 리포트 + 본문에 태그한 리포트를
 * 최신순 한 목록으로. 두 목록은 서버가 이미 종목으로 거른다(각 20건) — 합쳐 시간순으로 세운다.
 *
 * 해외 종목의 Reddit 자료는 2026-10-09부터 별도 「레딧」 탭이다(StockRedditPanel).
 */
type Row = { kind: "post"; at: string; post: CommunityPost } | { kind: "report"; at: string; call: ResearchCall };

type Filter = "all" | "post" | "report";

const LIMIT = 20;

export function StockCommunityTab({ symbol, name }: { symbol: string; name: string }) {
  const [posts, setPosts] = useState<CommunityPost[] | null>(null);
  const [calls, setCalls] = useState<ResearchCall[] | null>(null);
  const [filter, setFilter] = useState<Filter>("all");

  useEffect(() => {
    let alive = true;
    fetchCommunityPosts({ symbol, limit: LIMIT })
      .then((p) => alive && setPosts(p.items))
      .catch(() => alive && setPosts([]));
    fetchResearchCallsClient({ symbol, limit: LIMIT })
      .then((c) => alive && setCalls(c))
      .catch(() => alive && setCalls([]));
    return () => {
      alive = false;
    };
  }, [symbol]);

  const rows = useMemo<Row[]>(() => {
    const out: Row[] = [
      ...(posts ?? []).map((post) => ({
        kind: "post" as const,
        at: post.createdAt,
        post,
      })),
      ...(calls ?? []).map((call) => ({
        kind: "report" as const,
        at: call.publishedAt || call.createdAt,
        call,
      })),
    ];
    return out.sort((a, b) => b.at.localeCompare(a.at));
  }, [posts, calls]);

  const loading = posts === null || calls === null;
  const shown = rows.filter((r) => filter === "all" || r.kind === filter);
  const counts = {
    all: rows.length,
    post: posts?.length ?? 0,
    report: calls?.length ?? 0,
  };
  const writeHref = `/community/new?symbols=${encodeURIComponent(symbol)}`;

  return (
    <div className="space-y-4">
      <section className="overflow-hidden rounded-xl border border-[var(--border)] bg-[var(--surface)]">
        <div className="flex flex-wrap items-center gap-1.5 border-b border-[var(--border)] px-3 py-2">
          {(["all", "post", "report"] as const).map((f) => (
            <button
              key={f}
              type="button"
              onClick={() => setFilter(f)}
              aria-pressed={filter === f}
              className={cn(
                "rounded-full border px-2.5 py-0.5 text-cell",
                filter === f
                  ? "border-[var(--brand)] bg-[var(--brand)]/10 font-semibold text-[var(--brand)]"
                  : "border-[var(--border)] text-[var(--text-muted)] hover:text-[var(--text-primary)]",
              )}
            >
              {f === "all" ? "전체" : f === "post" ? "커뮤니티" : "리포트"}
              {!loading && <span className="ml-1 tabular-nums opacity-70">{counts[f]}</span>}
            </button>
          ))}
          <span className="ml-auto flex items-center gap-2">
            <Link href="/reports/new" className="text-cell text-[var(--text-muted)] hover:text-[var(--brand)]">
              리포트 쓰기
            </Link>
            <Link
              href={writeHref}
              className="inline-flex items-center gap-1 rounded-md border border-[var(--brand)] px-2 py-0.5 text-cell font-semibold text-[var(--brand)] hover:bg-[var(--brand)]/10"
            >
              <PenLine className="h-3 w-3" />이 종목으로 글쓰기
            </Link>
          </span>
        </div>

        {loading ? (
          <p className="px-4 py-8 text-center text-cell text-[var(--text-muted)]">불러오는 중…</p>
        ) : shown.length === 0 ? (
          <p className="px-4 py-8 text-center text-cell text-[var(--text-muted)]">
            아직 {name || symbol} 이야기가 없습니다. 글에 <code>{mentionTokenText("@", name || symbol)}</code>처럼
            태그하거나 종목을 붙이면 여기 모입니다.
          </p>
        ) : (
          <ul className="divide-y divide-[var(--border)]">
            {shown.map((r) =>
              r.kind === "post" ? (
                <li key={`p${r.post.id}`}>
                  <Link
                    href={`/community/${r.post.id}`}
                    className="flex min-w-0 gap-2.5 px-3 py-2.5 hover:bg-[var(--surface-elevated)]"
                  >
                    <span className="mt-0.5 h-fit shrink-0 rounded bg-[var(--surface-elevated)] px-1.5 text-micro text-[var(--text-secondary)]">
                      글
                    </span>
                    <span className="min-w-0 flex-1">
                      <span className="block truncate text-note font-semibold text-[var(--text-primary)]">
                        {r.post.title}
                      </span>
                      {r.post.excerpt && (
                        <span className="line-clamp-1 text-cell text-[var(--text-muted)]">
                          {plainMentions(r.post.excerpt)}
                        </span>
                      )}
                      <span className="mt-0.5 flex items-center gap-2 text-micro text-[var(--text-muted)]">
                        <span>{r.post.author.displayName}</span>
                        <span className="inline-flex items-center gap-0.5">
                          <Heart className="h-3 w-3" />
                          {r.post.likeCount}
                        </span>
                        <span className="inline-flex items-center gap-0.5">
                          <MessageSquare className="h-3 w-3" />
                          {r.post.commentCount}
                        </span>
                      </span>
                    </span>
                    <span className="shrink-0 text-micro text-[var(--text-muted)]">
                      {formatRelativeTime(r.post.createdAt)}
                    </span>
                  </Link>
                </li>
              ) : (
                <li key={`r${r.call.id}`}>
                  <Link
                    href={`/reports/${r.call.id}`}
                    className="flex min-w-0 gap-2.5 px-3 py-2.5 hover:bg-[var(--surface-elevated)]"
                  >
                    <span className="mt-0.5 inline-flex h-fit shrink-0 items-center gap-0.5 rounded bg-[var(--brand)]/10 px-1.5 text-micro font-semibold text-[var(--brand)]">
                      <FileText className="h-3 w-3" />
                      리포트
                    </span>
                    <span className="min-w-0 flex-1">
                      <span className="block truncate text-note font-semibold text-[var(--text-primary)]">
                        {r.call.title}
                      </span>
                      <span className="mt-0.5 flex flex-wrap items-center gap-x-2 text-micro text-[var(--text-muted)]">
                        {r.call.symbol.toUpperCase() !== symbol.toUpperCase() && (
                          <span>{r.call.symbolName || r.call.symbol} 리포트에서 언급</span>
                        )}
                        <span>{DIRECTION_LABELS[r.call.direction]}</span>
                        <span>{STATUS_LABELS[r.call.status]}</span>
                        {r.call.authorName && <span>{r.call.authorName}</span>}
                      </span>
                    </span>
                    <span className="shrink-0 text-micro text-[var(--text-muted)]">
                      {formatRelativeTime(r.call.publishedAt || r.call.createdAt)}
                    </span>
                  </Link>
                </li>
              ),
            )}
          </ul>
        )}
      </section>
    </div>
  );
}
