"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { Loader2, PenLine } from "lucide-react";
import { CommunityPostRow } from "@/components/community/community-post-row";
import { FilterGroup } from "@/components/community/community-bits";
import { PageColumns } from "@/components/layout/page-columns";
import { PageScrollShell } from "@/components/layout/page-content-shell";
import { useAuth } from "@/components/providers/auth-provider";
import { Card } from "@/components/ui/card";
import { SectionCard } from "@/components/ui/section-card";
import { buttonVariants } from "@/components/ui/button";
import { useLiveQuotes } from "@/hooks/use-live-quotes";
import { fetchCommunityPosts } from "@/lib/community-api";
import type { CommunityPost, CommunityPostPage } from "@/lib/community-types";
import { toUserMessage } from "@/lib/error-message";
import { cn } from "@/lib/utils";
import { TOPIC_FILTERS, isContentTopic, type ContentTopic } from "@/lib/topics";

type Tab = "latest" | "popular" | "mine";

const TABS: { id: Tab; label: string }[] = [
  { id: "latest", label: "전체" },
  { id: "popular", label: "인기" },
  { id: "mine", label: "내 활동" },
];

/**
 * 커뮤니티 목록 (2026-09-27 사용자 요청 4 — 첫 번째 참고 사진의 흐름).
 *
 * 첫 페이지는 서버가 HTML에 실어 준다(검색 색인). 마운트 뒤에는 **내 세션으로 다시 받아**
 * 덮는다 — 서버 선반입은 익명이라 좋아요 여부·내 글 표시가 없다.
 *
 * 종목 필터는 주소(`?symbol=`)에 둔다: 종목 화면의 「이 종목 이야기」 입구가 링크 하나로
 * 들어오고, 뒤로가기로 필터가 풀린다.
 */
export function CommunityListClient({
  initialPage,
  initialSymbol,
}: {
  initialPage: CommunityPostPage | null;
  initialSymbol?: string;
}) {
  const { session, loading: authLoading } = useAuth();
  const params = useSearchParams();
  const router = useRouter();
  const pathname = usePathname();
  const symbol = (params.get("symbol") ?? initialSymbol ?? "").toUpperCase();
  // 주제 필터(2026-10-09 요청 9) — 종목 필터처럼 주소에 둔다.
  const rawTopic = params.get("topic");
  const topic: ContentTopic | "" = isContentTopic(rawTopic) ? rawTopic : "";

  const [tab, setTab] = useState<Tab>("latest");
  const [items, setItems] = useState<CommunityPost[]>(initialPage?.items ?? []);
  const [cursor, setCursor] = useState<string | undefined>(initialPage?.nextCursor);
  const [loading, setLoading] = useState(initialPage === null);
  const [loadingMore, setLoadingMore] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const needsLogin = tab === "mine" && !authLoading && !session;
  const queryKey = `${tab}|${symbol}|${topic}|${session?.user.id ?? 0}`;

  const load = useCallback(
    async (key: string, append: boolean, from?: string) => {
      const [t, sym, tp] = key.split("|");
      try {
        const page = await fetchCommunityPosts({
          sort: t === "popular" ? "popular" : "latest",
          mine: t === "mine",
          symbol: sym || undefined,
          topic: isContentTopic(tp) ? tp : undefined,
          cursor: from,
        });
        setItems((prev) => (append ? [...prev, ...page.items] : page.items));
        setCursor(page.nextCursor);
        setError(null);
      } catch (err) {
        setError(toUserMessage(err, "글을 불러오지 못했습니다."));
      } finally {
        setLoading(false);
        setLoadingMore(false);
      }
    },
    [],
  );

  useEffect(() => {
    if (authLoading || needsLogin) return;
    let alive = true;
    Promise.resolve().then(() => {
      if (alive) void load(queryKey, false);
    });
    return () => {
      alive = false;
    };
  }, [queryKey, authLoading, needsLogin, load]);

  const setParam = useCallback(
    (name: "symbol" | "topic", next: string | null) => {
      const q = new URLSearchParams(params.toString());
      if (next) q.set(name, next);
      else q.delete(name);
      const qs = q.toString();
      router.replace(qs ? `${pathname}?${qs}` : pathname, { scroll: false });
    },
    [params, pathname, router],
  );
  const setSymbol = useCallback((next: string | null) => setParam("symbol", next), [setParam]);

  const symbols = useMemo(() => {
    const set = new Set<string>();
    for (const p of items) for (const s of p.symbols) set.add(s.symbol.toUpperCase());
    return [...set];
  }, [items]);
  const quotes = useLiveQuotes(symbols);

  const symbolName = symbol
    ? items.flatMap((p) => p.symbols).find((s) => s.symbol.toUpperCase() === symbol)?.name || symbol
    : null;

  const newHref = topic ? `/community/new?topic=${topic}` : "/community/new";
  const writeHref = session ? newHref : `/login?next=${encodeURIComponent(newHref)}`;

  return (
    <PageScrollShell>
      <PageColumns rail={<CommunityRail items={items} onPick={setSymbol} activeSymbol={symbol} />}>
        <div className="space-y-2.5">
          <div className="flex flex-wrap items-center justify-between gap-2">
            <div>
              <h1 className="text-xl font-bold text-[var(--text-primary)]">커뮤니티</h1>
              <p className="text-cell text-[var(--text-muted)]">
                주식·부동산·금융상품 이야기를 나눠요. 종목·단지·상품을 태그하면 시세와 AI 점수가 함께 보입니다.
              </p>
            </div>
            <Link
              href={writeHref}
              className={cn(buttonVariants({ variant: "default" }), "bg-[var(--brand)] text-white hover:opacity-90")}
            >
              <PenLine className="h-3.5 w-3.5" />
              글쓰기
            </Link>
          </div>
          <div className="flex flex-wrap items-center gap-1.5">
            <FilterGroup
              options={TABS}
              value={tab}
              onChange={(id) => {
                setTab(id as Tab);
                setLoading(true);
              }}
            />
            <FilterGroup
              options={TOPIC_FILTERS}
              value={topic}
              onChange={(id) => {
                setParam("topic", id || null);
                setLoading(true);
              }}
            />
            {symbolName && (
              <button
                type="button"
                onClick={() => setSymbol(null)}
                className="shrink-0 rounded-full bg-[var(--brand-muted)] px-2.5 py-1 text-cell font-medium text-[var(--brand)]"
                title="종목 필터 풀기"
              >
                ＃{symbolName} ✕
              </button>
            )}
          </div>
        </div>

        {needsLogin ? (
          <Card className="mt-3 space-y-2 p-6 text-center text-sm text-[var(--text-muted)]">
            <p>내가 쓴 글은 로그인하면 볼 수 있습니다.</p>
            <Link href={`/login?next=${encodeURIComponent("/community")}`} className="text-[var(--brand)] hover:underline">
              로그인
            </Link>
          </Card>
        ) : error && items.length === 0 ? (
          <Card className="mt-3 p-4 text-sm text-[var(--loss)]">{error}</Card>
        ) : loading && items.length === 0 ? (
          <Card className="mt-3 flex items-center justify-center gap-2 p-8 text-sm text-[var(--text-muted)]">
            <Loader2 className="h-4 w-4 animate-spin" />
            불러오는 중…
          </Card>
        ) : items.length === 0 ? (
          <Card className="mt-3 space-y-2 p-8 text-center">
            <p className="text-sm text-[var(--text-secondary)]">
              {tab === "mine" ? "아직 쓴 글이 없습니다." : symbolName ? `${symbolName} 이야기가 아직 없습니다.` : "아직 글이 없습니다."}
            </p>
            <Link href={writeHref} className="text-cell text-[var(--brand)] hover:underline">
              첫 글 쓰기
            </Link>
          </Card>
        ) : (
          <Card className="mt-3 divide-y divide-[var(--border)] overflow-hidden p-0">
            {items.map((post) => (
              <CommunityPostRow key={post.id} post={post} quotes={quotes} onPickSymbol={(s) => setSymbol(s)} />
            ))}
          </Card>
        )}

        {cursor && items.length > 0 && (
          <div className="mt-3 flex justify-center">
            <button
              type="button"
              disabled={loadingMore}
              onClick={() => {
                setLoadingMore(true);
                void load(queryKey, true, cursor);
              }}
              className={cn(buttonVariants({ variant: "outline", size: "sm" }), "min-w-32")}
            >
              {loadingMore ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : "더 보기"}
            </button>
          </div>
        )}

        <p className="px-1 pb-2 pt-4 text-micro leading-relaxed text-[var(--text-muted)]">
          커뮤니티의 글은 회원 개인의 의견이며 투자 권유가 아닙니다. 리딩방·유료 추천 유도와 욕설은 올릴 수 없고,
          신고가 3건 모이면 운영자 확인 전까지 숨겨집니다.
        </p>
      </PageColumns>
    </PageScrollShell>
  );
}

/** 오른쪽 레일 — 지금 목록에서 많이 이야기되는 종목(받아 둔 글에서 센다, 요청 0). */
function CommunityRail({
  items,
  onPick,
  activeSymbol,
}: {
  items: CommunityPost[];
  onPick: (symbol: string | null) => void;
  activeSymbol: string;
}) {
  const top = useMemo(() => {
    const counts = new Map<string, { name: string; n: number }>();
    for (const p of items) {
      for (const s of p.symbols) {
        const key = s.symbol.toUpperCase();
        const cur = counts.get(key);
        counts.set(key, { name: s.name || s.symbol, n: (cur?.n ?? 0) + 1 });
      }
    }
    return [...counts.entries()].sort((a, b) => b[1].n - a[1].n).slice(0, 8);
  }, [items]);

  return (
    <div className="space-y-3">
      <SectionCard title="지금 이야기되는 종목">
        {top.length === 0 ? (
          <p className="px-1 py-2 text-cell text-[var(--text-muted)]">종목을 붙인 글이 아직 없습니다.</p>
        ) : (
          <ul className="space-y-0.5">
            {top.map(([sym, v]) => (
              <li key={sym}>
                <button
                  type="button"
                  onClick={() => onPick(activeSymbol === sym ? null : sym)}
                  className={cn(
                    "flex w-full items-center justify-between rounded-md px-2 py-1.5 text-cell hover:bg-[var(--surface-elevated)]",
                    activeSymbol === sym && "bg-[var(--brand-muted)] text-[var(--brand)]",
                  )}
                >
                  <span className="truncate font-medium">{v.name}</span>
                  <span className="text-micro text-[var(--text-muted)]">글 {v.n}</span>
                </button>
              </li>
            ))}
          </ul>
        )}
      </SectionCard>
      <SectionCard title="이렇게 쓰면 좋아요">
        <ul className="list-disc space-y-1 pl-4 text-cell leading-relaxed text-[var(--text-secondary)]">
          <li>종목을 붙이면 글 쓴 시점 가격이 기록돼 「그때 → 지금」이 함께 보입니다.</li>
          <li>기사를 붙이면 요약과 출처가 카드로 따라옵니다.</li>
          <li>포트폴리오에 담아 둔 종목 글에는 「주주」 뱃지가 붙습니다.</li>
        </ul>
      </SectionCard>
    </div>
  );
}
