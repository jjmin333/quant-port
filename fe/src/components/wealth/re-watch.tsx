"use client";

import Link from "next/link";
import { useCallback, useEffect, useState } from "react";
import { Star } from "lucide-react";
import { toast } from "sonner";
import { useAuth } from "@/components/providers/auth-provider";
import { useQuickLook } from "@/components/providers/quick-look-provider";
import { SectionCard } from "@/components/ui/section-card";
import { Pct, ScoreBadge } from "@/components/wealth/asset-intel-ui";
import { fetchReWatch, money, setReWatch, type ReWatchItem } from "@/lib/wealth-api";
import { cn } from "@/lib/utils";

/**
 * 관심 부동산 (2026-10-03 사용자 결정 R3) — 단지·지역 ★. 알림 푸시는 없고 화면에 표시만:
 * 부동산 시세의 단지·지역 머리 별 · 내 포트폴리오 「관심 부동산」 · 홈 「관심 단지 새 거래」.
 */

export function reHref(it: Pick<ReWatchItem, "kind" | "key" | "lawdCd">): string {
  return it.kind === "complex"
    ? `/real-estate?${new URLSearchParams({ lawd: it.lawdCd, complex: it.key })}`
    : `/real-estate?lawd=${it.lawdCd}`;
}

/** 로그인했을 때만 목록을 받는다. 비로그인이면 items = []. */
export function useReWatch() {
  const { session } = useAuth();
  const [items, setItems] = useState<ReWatchItem[] | null>(null);
  const reload = useCallback(() => {
    if (!session) return;
    fetchReWatch().then((d) => setItems(d.items)).catch(() => setItems([]));
  }, [session]);
  useEffect(() => {
    reload();
  }, [reload]);
  const on = (kind: ReWatchItem["kind"], key: string) => (items ?? []).some((i) => i.kind === kind && i.key === key);
  const toggle = async (kind: ReWatchItem["kind"], key: string) => {
    const next = !on(kind, key);
    try {
      await setReWatch(kind, key, next);
      toast.success(next ? "관심 부동산에 담았습니다" : "관심 부동산에서 뺐습니다");
      reload();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "처리하지 못했습니다");
    }
  };
  return { signedIn: Boolean(session), items: session ? items : [], on, toggle };
}

/** 머리의 ★ — 비로그인이면 그리지 않는다. */
export function WatchStar({ kind, itemKey, className }: { kind: ReWatchItem["kind"]; itemKey: string; className?: string }) {
  const w = useReWatch();
  if (!w.signedIn || w.items == null) return null;
  const active = w.on(kind, itemKey);
  return (
    <button
      type="button"
      onClick={() => void w.toggle(kind, itemKey)}
      aria-pressed={active}
      title={active ? "관심 부동산에서 빼기" : "관심 부동산에 담기"}
      className={cn("inline-flex items-center gap-1 rounded-md border px-2 py-0.5 text-micro hover:bg-muted", active && "border-amber-500/50 text-amber-600", className)}
    >
      <Star className={cn("size-3.5", active && "fill-current")} />
      {active ? "관심" : "관심 담기"}
    </button>
  );
}

function Row({ it }: { it: ReWatchItem }) {
  const last = it.trades?.last;
  const f = (it.score?.facts ?? {}) as Record<string, number>;
  const quickLook = useQuickLook();
  return (
    <li className="flex items-center border-b last:border-b-0 hover:bg-muted">
      {/* 누르면 미리보기 창(2026-10-09 요청 5). 주소는 남겨 새 탭 열기는 그대로 된다. */}
      <Link
        href={reHref(it)}
        onClick={(ev) => {
          if (!quickLook || ev.metaKey || ev.ctrlKey || ev.shiftKey) return;
          ev.preventDefault();
          quickLook.openAsset({ kind: it.kind === "complex" ? "apt" : "region", key: it.key, name: it.name });
        }}
        className="flex min-w-0 flex-1 items-baseline gap-2 py-1.5 pl-3 text-cell">
        <span className="min-w-0 flex-1">
          <span className="flex items-baseline gap-1.5">
            <b className="truncate">{it.name}</b>
            {it.trades?.newHigh && <span className="rounded bg-[var(--profit)]/10 px-1 text-micro text-[var(--profit)]">신고가</span>}
            {it.kind === "region" && <span className="text-micro text-[var(--text-muted)]">지역</span>}
          </span>
          {it.note?.body?.note && <span className="block truncate text-micro text-[var(--text-muted)]">{it.note.body.note}</span>}
        </span>
        {it.kind === "complex" ? (
          <span className="shrink-0 text-right text-micro text-[var(--text-muted)]">
            {last ? <><b className="font-num text-cell text-foreground tabular-nums">{money(last.price)}</b> <span>{last.dealDate.slice(5)} · {Math.floor(last.area)}㎡</span></> : "최근 거래 없음"}
            {it.trades && it.trades.recent30 > 0 && <span className="block">30일 {it.trades.recent30}건</span>}
          </span>
        ) : (
          <span className="shrink-0 text-micro text-[var(--text-muted)]">4주 <Pct v={f.w4Pct} digits={2} /></span>
        )}
      </Link>
      {/* 배지는 팝오버 버튼이라 링크 밖에 둔다(링크 안 버튼은 눌러도 이동한다). */}
      <span className="shrink-0 px-3"><ScoreBadge score={it.score} kind={it.kind} /></span>
    </li>
  );
}

/** 내 포트폴리오 「관심 부동산」 — 담은 것이 없으면 안내 한 줄. */
export function ReWatchCard() {
  const w = useReWatch();
  if (!w.signedIn) return null;
  return (
    <SectionCard title="관심 부동산" href="/real-estate" hrefLabel="부동산 시세" bodyClassName="p-0">
      {w.items == null ? (
        <p className="p-3 text-cell text-[var(--text-muted)]">불러오는 중…</p>
      ) : w.items.length === 0 ? (
        <p className="p-3 text-cell text-[var(--text-muted)]">부동산 시세의 단지·지역 화면에서 「관심 담기」를 누르면 여기서 새 거래와 신고가를 봅니다.</p>
      ) : (
        <ul>{w.items.map((it) => <Row key={`${it.kind}-${it.key}`} it={it} />)}</ul>
      )}
    </SectionCard>
  );
}

/** 홈 「관심 단지 새 거래」 — 최근 30일 거래가 있거나 신고가인 단지만. 없으면 그리지 않는다. */
export function ReWatchHomeCard() {
  const w = useReWatch();
  const fresh = (w.items ?? []).filter((it) => it.kind === "complex" && it.trades && (it.trades.recent30 > 0 || it.trades.newHigh)).slice(0, 5);
  if (fresh.length === 0) return null;
  return (
    <SectionCard title="관심 단지 새 거래" href="/holdings" hrefLabel="전체" bodyClassName="p-0">
      <ul>{fresh.map((it) => <Row key={it.key} it={it} />)}</ul>
    </SectionCard>
  );
}
