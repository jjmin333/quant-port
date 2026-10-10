"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { Loader2 } from "lucide-react";
import { toast } from "sonner";
import { PageHeader } from "@/components/layout/header";
import { PageScrollShell } from "@/components/layout/page-content-shell";
import { useAuth } from "@/components/providers/auth-provider";
import { Card } from "@/components/ui/card";
import { isAdminSession } from "@/lib/admin-features";
import {
  fetchCommunityModeration,
  setCommunityStatus,
  type CommunityModeration,
} from "@/lib/community-api";
import { toUserMessage } from "@/lib/error-message";
import { formatDateTimeKST } from "@/lib/format";
import { cn } from "@/lib/utils";

/**
 * 커뮤니티 신고 관리 (2026-09-27 사용자 결정: 신고 3회 자동 숨김 + 관리자 화면).
 *
 * 신고가 한 번이라도 들어왔거나 숨겨진 글·댓글만 모은다. 숨겨진 것이 맨 위 — 자동 숨김은
 * **관리자가 확인하기 전까지의 임시 조치**라, 확인해서 되살리거나 그대로 두는 것이 이 화면의 일이다.
 */
export function AdminCommunityPanel() {
  const router = useRouter();
  const { session, loading } = useAuth();
  const [data, setData] = useState<CommunityModeration | null>(null);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    try {
      setData(await fetchCommunityModeration());
      setError(null);
    } catch (e) {
      setError(toUserMessage(e, "목록을 불러오지 못했습니다"));
    }
  }, []);

  useEffect(() => {
    if (loading) return;
    if (!isAdminSession(session)) {
      router.replace("/");
      return;
    }
    let alive = true;
    Promise.resolve().then(() => {
      if (alive) void load();
    });
    return () => {
      alive = false;
    };
  }, [loading, session, router, load]);

  async function setStatus(target: "post" | "comment", id: number, status: "visible" | "hidden") {
    try {
      await setCommunityStatus(target, id, status);
      toast.success(status === "hidden" ? "숨겼습니다" : "다시 보이게 했습니다");
      await load();
    } catch (e) {
      toast.error(toUserMessage(e, "처리하지 못했습니다"));
    }
  }

  if (loading || !isAdminSession(session)) {
    return (
      <div className="flex h-48 items-center justify-center text-sm text-[var(--text-muted)]">
        <Loader2 className="mr-2 h-4 w-4 animate-spin" />
        확인 중…
      </div>
    );
  }

  return (
    <>
      <PageHeader title="커뮤니티 신고 관리" subtitle="신고가 들어왔거나 숨겨진 글·댓글 — 서로 다른 3명이 신고하면 자동으로 숨겨집니다" />
      <PageScrollShell bare>
        <div className="space-y-4">
          {error && <Card className="p-4 text-sm text-[var(--loss)]">{error}</Card>}
          {data === null ? (
            <Card className="p-6 text-center text-sm text-[var(--text-muted)]">불러오는 중…</Card>
          ) : (
            <>
              <Card className="p-0">
                <h2 className="border-b border-[var(--border)] px-4 py-2.5 text-note font-semibold">글 {data.posts.length}</h2>
                {data.posts.length === 0 ? (
                  <p className="px-4 py-4 text-cell text-[var(--text-muted)]">신고된 글이 없습니다.</p>
                ) : (
                  <ul className="divide-y divide-[var(--border)]">
                    {data.posts.map((p) => (
                      <Row
                        key={p.id}
                        status={p.status}
                        reports={p.reportCount ?? 0}
                        reason={p.hiddenReason}
                        who={p.author.displayName}
                        at={p.createdAt}
                        onToggle={() => void setStatus("post", p.id, p.status === "hidden" ? "visible" : "hidden")}
                      >
                        <Link href={`/community/${p.id}`} className="font-semibold hover:text-[var(--brand)]">
                          {p.title}
                        </Link>
                      </Row>
                    ))}
                  </ul>
                )}
              </Card>
              <Card className="p-0">
                <h2 className="border-b border-[var(--border)] px-4 py-2.5 text-note font-semibold">댓글 {data.comments.length}</h2>
                {data.comments.length === 0 ? (
                  <p className="px-4 py-4 text-cell text-[var(--text-muted)]">신고된 댓글이 없습니다.</p>
                ) : (
                  <ul className="divide-y divide-[var(--border)]">
                    {data.comments.map((c) => (
                      <Row
                        key={c.id}
                        status={c.status}
                        reports={c.reportCount ?? 0}
                        who={c.author.displayName}
                        at={c.createdAt}
                        onToggle={() => void setStatus("comment", c.id, c.status === "hidden" ? "visible" : "hidden")}
                      >
                        <span className="line-clamp-2">{c.body}</span>
                        <Link href={`/community/${c.postId}`} className="ml-1 text-micro text-[var(--brand)] hover:underline">
                          글 보기
                        </Link>
                      </Row>
                    ))}
                  </ul>
                )}
              </Card>
            </>
          )}
        </div>
      </PageScrollShell>
    </>
  );
}

function Row({
  status,
  reports,
  reason,
  who,
  at,
  onToggle,
  children,
}: {
  status: string;
  reports: number;
  reason?: string;
  who: string;
  at: string;
  onToggle: () => void;
  children: React.ReactNode;
}) {
  const hidden = status === "hidden";
  return (
    <li className="flex min-w-0 items-start gap-3 px-4 py-3 text-cell">
      <div className="min-w-0 flex-1 space-y-1">
        <div className="min-w-0">{children}</div>
        <div className="flex flex-wrap items-center gap-2 text-micro text-[var(--text-muted)]">
          <span>{who}</span>
          <span>{formatDateTimeKST(at)}</span>
          <span className="rounded bg-rose-500/10 px-1 font-semibold text-rose-600">신고 {reports}</span>
          {hidden && (
            <span className="rounded bg-amber-500/15 px-1 font-semibold text-amber-700 dark:text-amber-400">
              숨김{reason === "reports" ? " (자동)" : reason === "admin" ? " (관리자)" : ""}
            </span>
          )}
        </div>
      </div>
      <button
        type="button"
        onClick={onToggle}
        className={cn(
          "shrink-0 rounded-lg border px-2.5 py-1 text-micro font-semibold",
          hidden ? "border-[var(--brand)] text-[var(--brand)]" : "border-rose-400 text-rose-500",
        )}
      >
        {hidden ? "다시 보이기" : "숨기기"}
      </button>
    </li>
  );
}
