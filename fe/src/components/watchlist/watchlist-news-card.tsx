"use client";

import { useEffect, useState } from "react";
import { ArrowRight } from "lucide-react";
import { useNewsArticleModal } from "@/components/news/news-article-modal-host";
import { SymbolNewsModal } from "@/components/stocks/symbol-news-modal";
import { SectionCard, SectionEmpty, SectionSkeleton } from "@/components/ui/section-card";
import { formatRelativeTime } from "@/lib/format-news";
import { fetchStockNews } from "@/lib/news-api";
import type { NewsArticle } from "@/lib/news-types";

type Row = NewsArticle & { subjectName: string; subjectSymbol: string };

/**
 * 관심종목 뉴스.
 *
 * 뉴스 화면은 시장 전체를 보여준다. 여기서는 **내가 지켜보기로 한 종목**에 무슨 일이
 * 있었는지만 본다 — 관심종목을 담아두는 이유가 대개 그거다.
 *
 * 종목별 요청이라 앞 6종목까지만. 목록 순서(내 순서 또는 정렬된 순서)를 그대로 받으므로
 * 사용자가 위에 둔 종목이 먼저 조회된다.
 */
const MAX_SYMBOLS = 6;
const PER_SYMBOL = 2;

export function WatchlistNewsCard({
  symbols,
}: {
  symbols: Array<{ symbol: string; name: string; market: "domestic" | "us" }>;
}) {
  const [modalOpen, setModalOpen] = useState(false);
  const [loaded, setLoaded] = useState<{ key: string; rows: Row[] } | null>(null);

  const key = symbols
    .slice(0, MAX_SYMBOLS)
    .map((s) => s.symbol)
    .join(",");
  const rows = !key ? [] : loaded?.key === key ? loaded.rows : null;

  /*
   * 기사 줄은 **모달로 연다**. 예전에는 `/news/{기사ID}`로 링크를 걸었는데 그 라우트가
   * 없다 — 뉴스 화면은 `/news/domestic`과 `/news/us` 둘뿐이라 누르면 404였다. 바로 위
   * 「더보기」는 모달을 여는데 기사 줄만 없는 페이지로 나가고 있었던 셈이다.
   */
  const newsModal = useNewsArticleModal({ pool: rows ?? undefined });

  useEffect(() => {
    if (!key) return;
    let alive = true;
    const targets = symbols.slice(0, MAX_SYMBOLS);
    Promise.all(
      targets.map((t) =>
        fetchStockNews({ symbol: t.symbol, name: t.name, market: t.market, limit: PER_SYMBOL })
          .then((articles) =>
            articles.slice(0, PER_SYMBOL).map((a) => ({
              ...a,
              subjectName: t.name,
              subjectSymbol: t.symbol,
            })),
          )
          // 한 종목이 실패해도 나머지 뉴스는 유효하다.
          .catch(() => [] as Row[]),
      ),
    )
      .then((lists) => {
        if (!alive) return;
        const merged = lists
          .flat()
          .sort((a, b) => b.publishedAt.localeCompare(a.publishedAt))
          .slice(0, 6);
        setLoaded({ key, rows: merged });
      })
      .catch(() => {
        if (alive) setLoaded({ key, rows: [] });
      });
    return () => {
      alive = false;
    };
    // 배열 참조는 렌더마다 새로 만들어지므로 심볼 문자열로 의존성을 잡는다.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [key]);

  return (
    <>
      <SectionCard
        title="관심종목 뉴스"
        action={
          <button
            type="button"
            onClick={() => setModalOpen(true)}
            className="flex shrink-0 items-center gap-0.5 text-[11px] text-muted-foreground hover:text-foreground"
          >
            더보기
            <ArrowRight className="size-3" />
          </button>
        }
      >
        {rows === null ? (
          <SectionSkeleton rows={4} />
        ) : rows.length === 0 ? (
          <SectionEmpty text="최근 뉴스가 없습니다" />
        ) : (
          <ul className="min-w-0">
            {rows.map((a) => (
              <li key={`${a.subjectSymbol}-${a.id}`} className="min-w-0 py-1">
                <button
                  type="button"
                  onClick={() => newsModal.open(a)}
                  className="block w-full min-w-0 rounded-md px-1 py-0.5 text-left hover:bg-accent"
                >
                  <p className="line-clamp-2 text-[11px] leading-snug">
                    {a.headlineKo || a.headline}
                  </p>
                  <p className="mt-0.5 truncate text-[10px] text-muted-foreground">
                    {a.subjectName} · {formatRelativeTime(a.publishedAt)}
                  </p>
                </button>
              </li>
            ))}
          </ul>
        )}
      </SectionCard>

      <SymbolNewsModal
        open={modalOpen}
        onOpenChange={setModalOpen}
        symbols={symbols}
        title="관심종목 뉴스"
        subjectLabel="관심종목 상위"
      />

      {newsModal.modal}
    </>
  );
}
