"use client";

import type { NewsSort, NewsSource, NewsSourceId } from "@/lib/news-types";
import { cn } from "@/lib/utils";

/**
 * B2 — 필터 둘째 줄.
 *
 * 정렬과 출처는 지금까지 우상단 드롭다운에 있었다. 거기 있으면 있는 줄도 모르고, 첫
 * 줄(지수·종목)과 성격이 다른 조건이라 섞어 놓기도 어렵다. 그래서 별도 줄로 내리고,
 * 「사진 있는 기사」처럼 기사 성격으로 거르는 토글을 같이 둔다.
 *
 * 「번역 완료」는 해외 전용이다 — 국내 기사에는 `headlineKo`라는 개념이 없다.
 */

export type NewsSecondaryFilters = {
  withImage: boolean;
  translatedOnly: boolean;
};

export const EMPTY_SECONDARY_FILTERS: NewsSecondaryFilters = {
  withImage: false,
  translatedOnly: false,
};

export function NewsSecondaryFilterRow({
  market,
  sort,
  onSortChange,
  sourceId,
  sources,
  onSourceChange,
  filters,
  onFiltersChange,
}: {
  market: "domestic" | "us";
  sort: NewsSort;
  onSortChange: (sort: NewsSort) => void;
  sourceId: NewsSourceId | "all";
  sources: NewsSource[];
  onSourceChange: (id: NewsSourceId | "all") => void;
  filters: NewsSecondaryFilters;
  onFiltersChange: (next: NewsSecondaryFilters) => void;
}) {
  return (
    <div className="flex flex-wrap items-center gap-1.5 border-t border-dashed border-[var(--border)] pt-2">
      <Chip active={sort === "latest"} onClick={() => onSortChange("latest")}>
        🕐 최신순
      </Chip>
      <Chip active={sort === "popular"} onClick={() => onSortChange("popular")}>
        🔥 인기순
      </Chip>

      <span className="mx-0.5 h-4 w-px self-center bg-[var(--border)]" />

      <Chip
        active={filters.withImage}
        onClick={() => onFiltersChange({ ...filters, withImage: !filters.withImage })}
      >
        📷 사진 있는 기사
      </Chip>
      {market === "us" && (
        <Chip
          active={filters.translatedOnly}
          onClick={() =>
            onFiltersChange({ ...filters, translatedOnly: !filters.translatedOnly })
          }
        >
          🇰🇷 번역 완료
        </Chip>
      )}

      <span className="mx-0.5 h-4 w-px self-center bg-[var(--border)]" />

      {/*
       * 출처는 값이 많아 칩으로 늘어놓으면 줄이 넘친다. 셀렉트로 두되 라벨을 함께
       * 보여, 무엇으로 걸러져 있는지 줄에서 바로 읽히게 한다.
       */}
      <label className="inline-flex items-center gap-1 rounded-full border border-[var(--border)] bg-[var(--surface)] px-2.5 py-1 text-[11px] text-[var(--text-secondary)]">
        <span className="text-[var(--text-muted)]">📰</span>
        <select
          value={sourceId}
          onChange={(e) => onSourceChange(e.target.value as NewsSourceId | "all")}
          aria-label="출처 선택"
          className="max-w-[9rem] cursor-pointer truncate bg-transparent outline-none"
        >
          <option value="all">전체 출처</option>
          {sources.map((source) => (
            <option key={source.id} value={source.id}>
              {source.name}
            </option>
          ))}
        </select>
      </label>
    </div>
  );
}

function Chip({
  active,
  onClick,
  children,
}: {
  active: boolean;
  onClick: () => void;
  children: React.ReactNode;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-pressed={active}
      className={cn(
        "rounded-full border px-2.5 py-1 text-[11px] transition-colors",
        active
          ? "border-[var(--brand)]/40 bg-[var(--brand-muted)] text-[var(--brand)]"
          : "border-[var(--border)] bg-[var(--surface)] text-[var(--text-muted)] hover:text-[var(--text-primary)]",
      )}
    >
      {children}
    </button>
  );
}
