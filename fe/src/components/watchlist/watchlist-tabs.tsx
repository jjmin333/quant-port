"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { PageScrollShell, pageShellClass } from "@/components/layout/page-content-shell";
import { SectionCard } from "@/components/ui/section-card";
import { WatchButton, useFinWatchlist } from "@/components/finance/fin-shared";
import { ReWatchCard } from "@/components/wealth/re-watch";
import { WatchlistClient } from "@/components/watchlist/watchlist-client";
import { fetchFinProduct, type FinProduct } from "@/lib/fin-products-api";
import { ASSET_FEATURES_HIDDEN } from "@/lib/hidden-features";
import { cn } from "@/lib/utils";

/**
 * 관심 목록 — 종목 · 아파트·지역 · 금융상품 (2026-10-09 요청 9).
 *
 * 관심 단지·상품은 이미 담을 수 있었지만(부동산·금융상품 화면의 「관심 담기」) 모아 볼 곳이 주식 메뉴뿐이었다.
 * 탭은 주소(`?tab=`)에 둔다 — 홈·사이드바에서 링크 하나로 그 탭을 연다.
 */
const TABS = [
  { id: "stock", label: "종목" },
  { id: "realestate", label: "아파트·지역" },
  { id: "finance", label: "금융상품" },
] as const;
type TabId = (typeof TABS)[number]["id"];

export function WatchlistTabs() {
  const params = useSearchParams();
  const router = useRouter();
  const pathname = usePathname();
  const raw = params.get("tab");
  const tab: TabId = TABS.some((t) => t.id === raw) ? (raw as TabId) : "stock";

  // 부동산·금융상품 비노출(lib/hidden-features)이면 탭 없이 종목만.
  if (ASSET_FEATURES_HIDDEN) {
    return (
      <div className="flex min-h-0 flex-1 flex-col">
        <WatchlistClient />
      </div>
    );
  }

  const bar = (
    <div className={cn(pageShellClass("wide"), "pt-3")}>
      <div className="inline-flex gap-0.5 rounded-lg bg-[var(--surface-elevated)] p-0.5" role="tablist" aria-label="관심 목록 종류">
        {TABS.map((t) => (
          <button
            key={t.id}
            type="button"
            role="tab"
            aria-selected={tab === t.id}
            onClick={() => router.replace(t.id === "stock" ? pathname : `${pathname}?tab=${t.id}`, { scroll: false })}
            className={cn(
              "rounded-md px-3 py-1 text-cell font-medium",
              tab === t.id ? "bg-[var(--surface)] text-[var(--text-primary)] shadow-sm" : "text-[var(--text-muted)] hover:text-[var(--text-secondary)]",
            )}
          >
            {t.label}
          </button>
        ))}
      </div>
    </div>
  );

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      {bar}
      {tab === "stock" ? (
        <WatchlistClient />
      ) : (
        <PageScrollShell contentClassName="max-w-3xl">{tab === "realestate" ? <ReWatchCard /> : <FinWatchCard />}</PageScrollShell>
      )}
    </div>
  );
}

/** 관심 금융상품 — 서버는 id만 준다. ponytail: 상품마다 한 번씩 읽는다(보통 10개 미만), 늘면 ids 일괄 API로. */
function FinWatchCard() {
  const { ids, toggle } = useFinWatchlist();
  const [items, setItems] = useState<FinProduct[] | null>(null);
  const key = [...ids].sort((a, b) => a - b).join(",");
  useEffect(() => {
    let alive = true;
    const list = key ? key.split(",").map(Number) : [];
    Promise.all(list.map((id) => fetchFinProduct(id).catch(() => null))).then(
      (rows) => alive && setItems(rows.filter((r): r is FinProduct => r !== null)),
    );
    return () => {
      alive = false;
    };
  }, [key]);

  return (
    <SectionCard title="관심 금융상품" href="/finance/savings" hrefLabel="예·적금 분석" bodyClassName="p-0">
      {items == null ? (
        <p className="p-3 text-cell text-[var(--text-muted)]">불러오는 중…</p>
      ) : items.length === 0 ? (
        <p className="p-3 text-cell text-[var(--text-muted)]">
          예·적금 분석·대출 분석에서 상품의 ♡를 누르면 여기에 모입니다.
        </p>
      ) : (
        <ul className="divide-y divide-[var(--border)]">
          {items.map((p) => (
            <li key={p.id} className="flex items-center gap-2 px-3 py-2.5">
              <Link href={`/finance/products/${p.id}`} className="min-w-0 flex-1 hover:underline">
                <span className="block truncate text-cell font-semibold text-[var(--text-primary)]">{p.productName}</span>
                <span className="block truncate text-micro text-[var(--text-muted)]">{p.companyName}</span>
              </Link>
              <WatchButton on={ids.has(p.id)} onClick={() => void toggle(p.id)} />
            </li>
          ))}
        </ul>
      )}
    </SectionCard>
  );
}
