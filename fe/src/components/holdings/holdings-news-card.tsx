"use client";

import { useEffect, useState } from "react";
import { ArrowRight } from "lucide-react";
import { HoldingsNewsModal } from "@/components/holdings/holdings-news-modal";
import { useNewsArticleModal } from "@/components/news/news-article-modal-host";
import { SectionCard, SectionEmpty, SectionSkeleton } from "@/components/ui/section-card";
import { formatRelativeTime } from "@/lib/format-news";
import { fetchStockNews } from "@/lib/news-api";
import type { NewsArticle } from "@/lib/news-types";

type Row = NewsArticle & { holdingName: string; holdingSymbol: string };

/**
 * 보유 종목 뉴스 — 비중 큰 종목 몇 개만.
 *
 * 뉴스 화면은 시장 전체를 보여준다. 여기서는 **내 돈이 놓인 종목**에서 무슨 일이
 * 있었는지만 본다. 종목별 요청이라 상위 6종목까지만 조회한다 — 스무 종목이면 스무
 * 요청이 나가는데, 비중 1%짜리 종목의 어제 기사는 이 카드가 있어야 할 이유가 아니다.
 *
 * 종목별 최신 2건씩 모아 시각순으로 섞는다. 한 종목이 목록을 다 채우면 나머지 보유
 * 종목에 아무 일도 없었던 것처럼 보인다.
 */
const MAX_SYMBOLS = 6;
const PER_SYMBOL = 2;

export function HoldingsNewsCard({
  symbols,
}: {
  /** 비중 높은 순 [symbol, name, market] */
  symbols: Array<{ symbol: string; name: string; market: "domestic" | "us" }>;
}) {
  const [modalOpen, setModalOpen] = useState(false);
  // 어느 심볼 집합에 대한 결과인지 함께 들고 있는다 — 별도 loading 상태를 이펙트에서
  // 동기로 세팅하지 않기 위해서다(캐스케이드 렌더).
  const [loaded, setLoaded] = useState<{ key: string; rows: Row[] } | null>(null);

  const key = symbols
    .slice(0, MAX_SYMBOLS)
    .map((s) => s.symbol)
    .join(",");
  const rows = !key ? [] : loaded?.key === key ? loaded.rows : null;

  /*
   * 기사 줄은 **모달로 연다**. 예전에는 `/news/{기사ID}`로 링크를 걸었는데 그 라우트가
   * 없다 — 뉴스 화면은 `/news/domestic`과 `/news/us` 둘뿐이라 누르면 404였다.
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
              holdingName: t.name,
              holdingSymbol: t.symbol,
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
      title="보유 종목 뉴스"
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
            <li key={`${a.holdingSymbol}-${a.id}`} className="min-w-0 py-1">
              <button
                type="button"
                onClick={() => newsModal.open(a)}
                className="block w-full min-w-0 rounded-md px-1 py-0.5 text-left hover:bg-accent"
              >
                <p className="line-clamp-2 text-[11px] leading-snug">
                  {a.headlineKo || a.headline}
                </p>
                <p className="mt-0.5 truncate text-[10px] text-muted-foreground">
                  {a.holdingName} · {formatRelativeTime(a.publishedAt)}
                </p>
              </button>
            </li>
          ))}
        </ul>
      )}
    </SectionCard>

    <HoldingsNewsModal open={modalOpen} onOpenChange={setModalOpen} symbols={symbols} />

    {newsModal.modal}
    </>
  );
}
