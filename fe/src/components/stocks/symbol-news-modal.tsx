"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { useNewsArticleModal } from "@/components/news/news-article-modal-host";
import { formatRelativeTime } from "@/lib/format-news";
import { fetchStockNews } from "@/lib/news-api";
import type { NewsArticle } from "@/lib/news-types";
import { cn } from "@/lib/utils";

type Row = NewsArticle & { subjectName: string; subjectSymbol: string };

const MODAL_SYMBOLS = 10;
const FETCH_PER_SYMBOL = 15;
const PAGE_SIZE = 15;

/**
 * 여러 종목 관련 뉴스 모달.
 *
 * 카드(6건)와 달리 모달은 **미리 받아 둔 캐시**를 페이지 단위로 보여준다.
 * 종목당 한 번씩만 요청하고(최대 10종목 × 15건), 스크롤 시 추가 API 호출 없이
 * 캐시에서 더 내려준다 — 무한 스크롤 UX와 비용·속도 균형.
 *
 * 종목당 건수를 25에서 15로 줄이고 종목을 8에서 10으로 늘린 이유: 한 종목이 목록을
 * 뒤덮는 것보다 내가 든 종목을 고루 훑는 쪽이 이 화면의 쓸모에 가깝다.
 */
export function SymbolNewsModal({
  open,
  onOpenChange,
  symbols,
  title,
  subjectLabel,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  symbols: Array<{ symbol: string; name: string; market: "domestic" | "us" }>;
  title: string;
  /** 설명 줄의 수식어 — 예: "비중 상위", "관심종목 상위". */
  subjectLabel: string;
}) {
  const [loaded, setLoaded] = useState<{ key: string; rows: Row[] } | null>(null);
  const [filterSymbol, setFilterSymbol] = useState<string>("all");
  const [visibleCount, setVisibleCount] = useState(PAGE_SIZE);
  const sentinelRef = useRef<HTMLDivElement>(null);
  const scrollTopRef = useRef(0);

  const targets = useMemo(() => symbols.slice(0, MODAL_SYMBOLS), [symbols]);
  const key = targets.map((s) => s.symbol).join(",");

  /*
   * 기사는 앱 전체와 **같은 방식**으로 연다(`useNewsArticleModal`). 여기서 열린 기사만
   * 예전에는 관련 종목·지수 카드가 없었다 — 정작 이 목록은 「내 종목 기사」라 그 카드가
   * 제일 쓸모 있는 자리다.
   *
   * `pool`은 이미 받아 둔 전체 목록을 준다(보이는 페이지가 아니라) — 「같은 이슈」는
   * 스크롤을 얼마나 내렸느냐와 무관한 성질이다.
   */
  const newsModal = useNewsArticleModal({ pool: loaded?.key === key ? loaded.rows : undefined });
  const selectedArticle = newsModal.article;

  /**
   * 기사를 여는 동안 목록 Dialog는 **닫는다**.
   *
   * 예전에는 겹쳐 띄우고 `modal={false}`로 포커스 트랩을 풀었는데, 기사 백드롭을 누르면
   * 그 클릭이 아래 Dialog의 "바깥 클릭"으로도 세어져 목록까지 함께 닫혔다. 겹치지
   * 않으면 그런 다툼 자체가 없다. 목록 상태(필터·불러온 기사·페이지 수)는 이 컴포넌트가
   * 들고 있어 그대로 남고, 스크롤 위치만 따로 되살린다.
   */
  const listOpen = open && !selectedArticle;

  const attachScroll = useCallback((node: HTMLDivElement | null) => {
    if (node) node.scrollTop = scrollTopRef.current;
  }, []);

  useEffect(() => {
    if (!open || !key) return;
    let alive = true;
    Promise.all(
      targets.map((t) =>
        fetchStockNews({
          symbol: t.symbol,
          name: t.name,
          market: t.market,
          limit: FETCH_PER_SYMBOL,
        })
          .then((articles) =>
            articles.map((a) => ({
              ...a,
              subjectName: t.name,
              subjectSymbol: t.symbol,
            })),
          )
          .catch(() => [] as Row[]),
      ),
    )
      .then((lists) => {
        if (!alive) return;
        const seen = new Set<string>();
        const merged = lists
          .flat()
          .filter((a) => {
            const id = `${a.market}:${a.id}`;
            if (seen.has(id)) return false;
            seen.add(id);
            return true;
          })
          .sort((a, b) => b.publishedAt.localeCompare(a.publishedAt));
        setLoaded({ key, rows: merged });
        setVisibleCount(PAGE_SIZE);
        setFilterSymbol("all");
      })
      .catch(() => {
        if (alive) setLoaded({ key, rows: [] });
      });
    return () => {
      alive = false;
    };
    // targets는 key에 이미 반영되므로 의존성에 넣지 않는다 — 넣으면 필터가 "전체"로 리셋된다.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, key]);

  // 목록을 닫으면 읽던 기사도 접는다. 이펙트가 아니라 렌더 중 조정 —
  // 이펙트로 하면 기사 모달이 한 프레임 더 남는다.
  const [wasOpen, setWasOpen] = useState(open);
  if (wasOpen !== open) {
    setWasOpen(open);
    if (!open && selectedArticle) newsModal.close();
  }

  useEffect(() => {
    if (!open) scrollTopRef.current = 0;
  }, [open]);

  const filtered = useMemo(() => {
    const allRows = !key ? [] : loaded?.key === key ? loaded.rows : null;
    if (!allRows) return null;
    if (filterSymbol === "all") return allRows;
    return allRows.filter((r) => r.subjectSymbol === filterSymbol);
  }, [key, loaded, filterSymbol]);

  const visible = filtered?.slice(0, visibleCount) ?? null;
  const hasMore = filtered != null && visibleCount < filtered.length;

  useEffect(() => {
    if (!listOpen || !hasMore) return;
    const node = sentinelRef.current;
    if (!node) return;
    const observer = new IntersectionObserver(
      (entries) => {
        if (entries[0]?.isIntersecting) {
          setVisibleCount((n) => n + PAGE_SIZE);
        }
      },
      { rootMargin: "120px" },
    );
    observer.observe(node);
    return () => observer.disconnect();
  }, [listOpen, hasMore, visibleCount, filterSymbol]);

  const onFilterChange = useCallback((sym: string) => {
    setFilterSymbol(sym);
    setVisibleCount(PAGE_SIZE);
    scrollTopRef.current = 0;
  }, []);

  return (
    <>
      <Dialog open={listOpen} onOpenChange={(next) => !next && onOpenChange(false)}>
        <DialogContent className="flex max-h-[85dvh] flex-col gap-0 overflow-hidden p-0 sm:max-w-lg">
          <DialogHeader className="border-b px-4 py-3 pr-10 text-left">
            <DialogTitle className="text-sm">{title}</DialogTitle>
            <DialogDescription className="text-[11px]">
              {subjectLabel} {targets.length}종목 관련 기사 · 종목당 최대 {FETCH_PER_SYMBOL}건
            </DialogDescription>
          </DialogHeader>

          {targets.length > 0 && (
            <div className="flex flex-wrap gap-1 border-b px-3 py-2">
              <FilterChip
                active={filterSymbol === "all"}
                label="전체"
                onClick={() => onFilterChange("all")}
              />
              {targets.map((t) => (
                <FilterChip
                  key={t.symbol}
                  active={filterSymbol === t.symbol}
                  label={t.name || t.symbol}
                  onClick={() => onFilterChange(t.symbol)}
                />
              ))}
            </div>
          )}

          <div
            ref={attachScroll}
            onScroll={(e) => {
              scrollTopRef.current = e.currentTarget.scrollTop;
            }}
            className="min-h-0 flex-1 overflow-y-auto px-3 py-2"
          >
            {visible === null ? (
              <div className="space-y-2 py-4">
                {Array.from({ length: 6 }).map((_, i) => (
                  <div key={i} className="h-10 animate-pulse rounded bg-muted" />
                ))}
              </div>
            ) : visible.length === 0 ? (
              <p className="py-10 text-center text-[11px] text-muted-foreground">
                관련 뉴스가 없습니다
              </p>
            ) : (
              <ul className="min-w-0">
                {visible.map((a) => (
                  <li key={`${a.subjectSymbol}-${a.id}`} className="min-w-0 border-b py-2 last:border-0">
                    <button
                      type="button"
                      className="block w-full min-w-0 rounded-md px-1 py-0.5 text-left hover:bg-accent"
                      onClick={() => newsModal.open(a)}
                    >
                      <p className="text-[12px] leading-snug">{a.headlineKo || a.headline}</p>
                      <p className="mt-0.5 truncate text-[10px] text-muted-foreground">
                        {a.subjectName} · {formatRelativeTime(a.publishedAt)}
                      </p>
                    </button>
                  </li>
                ))}
              </ul>
            )}
            {hasMore && <div ref={sentinelRef} className="h-4" aria-hidden />}
            {filtered != null && filtered.length > 0 && (
              <p className="py-2 text-center text-[10px] text-muted-foreground">
                {Math.min(visibleCount, filtered.length)} / {filtered.length}건
              </p>
            )}
          </div>
        </DialogContent>
      </Dialog>

      {newsModal.modal}
    </>
  );
}

function FilterChip({
  active,
  label,
  onClick,
}: {
  active: boolean;
  label: string;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={cn(
        "max-w-[120px] truncate rounded-full px-2 py-0.5 text-[10px] transition-colors",
        active
          ? "bg-[var(--brand-muted)] font-semibold text-[var(--brand)]"
          : "bg-muted text-muted-foreground hover:text-foreground",
      )}
    >
      {label}
    </button>
  );
}
