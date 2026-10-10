"use client";

import Link from "next/link";
import { ASSET_FEATURES_HIDDEN } from "@/lib/hidden-features";
import { NewsSymbolChips } from "@/components/news/news-symbol-chips";
import { useQuickLook } from "@/components/providers/quick-look-provider";
import {
  articleDashboardIndexIds,
  dashboardIndexLabel,
  type DashboardIndexId,
} from "@/lib/news-dashboard-indices";
import type { NewsArticle } from "@/lib/news-types";
import { formatPercent } from "@/lib/format";
import type { MarketIndex, QuoteSummary } from "@/lib/quotes-types";
import { trendTextClass } from "@/lib/trend-colors";
import { cn } from "@/lib/utils";

/**
 * 기사 하단 칩 — 종목 시세 + 지수 등락을 함께 표시한다.
 *
 * 예전에는 symbols만 보여서 금·비트코인·코스피 기사는 칩이 비어 있었다.
 * 지수는 article.tags / headline 규칙으로 이미 판정되므로 같은 규칙을 쓴다.
 */
function articleIndexRows(
  article: NewsArticle,
  indices: MarketIndex[],
  limit?: number,
) {
  const indexIds = articleDashboardIndexIds(article, limit ?? 32);
  const indexById = new Map(indices.map((i) => [i.id, i]));
  return indexIds
    .map((id) => ({ id, index: indexById.get(id) }))
    .filter(
      (row): row is { id: DashboardIndexId; index: MarketIndex } =>
        row.index != null && row.index.value > 0,
    );
}

function articleSymbolList(article: NewsArticle) {
  return (article.symbols ?? [])
    .map((s) => s.trim().toUpperCase())
    .filter(Boolean);
}

/** 기사에 붙일 수 있는 자산·지수·종목 칩 총 개수(시세 있는 지수만). */
export function articleChipTotalCount(article: NewsArticle, indices: MarketIndex[]) {
  return (
    (article.assetTags?.length ?? 0) +
    articleIndexRows(article, indices).length +
    articleSymbolList(article).length
  );
}

export function NewsArticleChips({
  article,
  quotes,
  quotesLoading = false,
  indices,
  max = 3,
  className,
  chipClassName,
  overflowHint = false,
}: {
  article: NewsArticle;
  quotes: Map<string, QuoteSummary>;
  quotesLoading?: boolean;
  indices: MarketIndex[];
  max?: number;
  className?: string;
  /** 좁은 칸(홈 목록)에서 긴 ETF 이름이 한 칩으로 줄을 넘기지 않게 한다. */
  chipClassName?: string;
  /** 숨긴 칩 개수를 +N으로 표시한다. */
  overflowHint?: boolean;
}) {
  /* 지수 칩도 종목 칩과 같은 규칙 — 평범한 클릭은 창을 연다(news-symbol-chips 참조). */
  const quickLook = useQuickLook();

  /*
    자산 칩(지역·금리, 2026-10-09 요청 4)이 맨 앞 — 부동산·금리 기사에는 종목이 없어 이것이 그 기사의
    「누를 거리」다. 누르면 글 태그와 같은 자산 미리보기 모달.
  */
  // 부동산·금리 자산 칩은 비노출 기능(lib/hidden-features) 동안 뺀다 — 미리보기가 닫힌 화면으로 이어진다.
  const allAssets = ASSET_FEATURES_HIDDEN ? [] : (article.assetTags ?? []);
  const assets = allAssets.slice(0, max);
  const allIndexRows = articleIndexRows(article, indices);
  const allSymbols = articleSymbolList(article);
  const indexRows = allIndexRows.slice(0, Math.max(0, max - assets.length));
  const symbolMax = Math.max(0, max - assets.length - indexRows.length);
  const symbols = allSymbols.slice(0, symbolMax);
  const hiddenCount = overflowHint
    ? allAssets.length +
      allIndexRows.length +
      allSymbols.length -
      assets.length -
      indexRows.length -
      symbols.length
    : 0;

  /*
    칩이 한 개도 안 들어가는 줄이 있다 — 폭이 모자라면 `measureMetaChipLayout`이 0을
    준다(2026-09-05). 그때도 **「+N」은 남긴다**: 「+11 · 11분 전」은 이 기사에 종목이
    열한 개 걸려 있다는 사실을 말하지만, 아무것도 안 그리면 그 기사만 유독 종목이
    없는 기사처럼 보인다.
  */
  if (assets.length === 0 && indexRows.length === 0 && symbols.length === 0 && hiddenCount === 0) {
    return null;
  }

  return (
    <div className={cn("flex min-w-0 flex-wrap items-center gap-1", className)}>
      {assets.map((a) => (
        <button
          key={`${a.kind}:${a.key}`}
          type="button"
          data-meta-chip=""
          onClick={(e) => {
            e.stopPropagation();
            e.preventDefault();
            quickLook?.openAsset({ kind: a.kind, key: a.key, name: a.name });
          }}
          disabled={!quickLook}
          className={cn(
            "inline-flex min-w-0 max-w-full items-center gap-1 rounded-full border border-[var(--brand)]/30 bg-[var(--brand)]/10 px-1.5 py-px text-micro font-semibold text-[var(--brand)] hover:border-[var(--brand)]",
            chipClassName,
          )}
          title={`${a.name} 자세히`}
        >
          <span aria-hidden>{a.kind === "rate" ? "％" : a.kind === "apt" ? "🏢" : a.kind === "sido" ? "🗺️" : "📍"}</span>
          <span className="truncate">{a.name}</span>
        </button>
      ))}
      {indexRows.map(({ id, index }) => (
        <Link
          key={id}
          href="/market"
          data-meta-chip=""
          onClick={(e) => {
            e.stopPropagation();
            if (!quickLook) return;
            if (e.metaKey || e.ctrlKey || e.shiftKey || e.altKey || e.button !== 0) return;
            e.preventDefault();
            quickLook.openIndex(id);
          }}
          className={cn(
            "inline-flex min-w-0 max-w-full items-center gap-1 rounded-full border border-[var(--border)] bg-[var(--surface)] px-1.5 py-px text-micro hover:border-[var(--brand)]",
            chipClassName,
          )}
        >
          <span className="truncate text-[var(--text-secondary)]">{dashboardIndexLabel(id)}</span>
          <span
            className={cn(
              "shrink-0 font-num font-bold tabular-nums",
              trendTextClass(index.changeRate),
            )}
          >
            {formatPercent(index.changeRate)}
          </span>
        </Link>
      ))}
      {symbols.length > 0 && (
        <NewsSymbolChips
          symbols={symbols}
          quotes={quotes}
          quotesLoading={quotesLoading}
          max={symbolMax}
          chipClassName={chipClassName}
          /*
            `flex-nowrap` 이라 칩이 줄바꿈하지 않는데, 넘치는 만큼을 잘라 주지 않으면
            **부모 밖으로 밀고 나간다** — 390px에서 종목 칩이 x=416까지 나갔고 그 줄에는
            가로 스크롤 컨테이너도 없어 닿을 수 없었다. 넘친 칩은 잘라 낸다. 몇 개가
            더 있는지는 옆의 `+N`이 이미 말해 준다.
          */
          className="min-w-0 flex-nowrap overflow-hidden"
        />
      )}
      {hiddenCount > 0 && (
        <span className="shrink-0 text-micro text-[var(--text-muted)]">+{hiddenCount}</span>
      )}
    </div>
  );
}