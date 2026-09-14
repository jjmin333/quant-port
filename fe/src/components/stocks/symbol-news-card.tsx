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

/*
「내 종목 뉴스」 — **한 벌** (2026-09-05 사용자 요청 7).

# 무엇이 두 벌이었나

관심 화면(`watchlist-news-card`)과 보유 화면(`holdings-news-card`)에 같은 카드가 있었다.
두 파일의 diff는 **제목 두 줄과 필드 이름 두 개**(`subjectName` ↔ `holdingName`)뿐이었고,
가져오는 방식·정렬·빈 문구·모달 여는 법이 전부 같았다. 모달은 이미 한 벌이었다
(`SymbolNewsModal`, 08-14에 합쳤다) — 카드만 남아 있었다.

# 무엇을 하나

뉴스 화면은 시장 전체를 보여준다. 이 카드는 **부르는 쪽이 골라 준 종목**에 무슨 일이
있었는지만 본다. 그 종목이 「내가 지켜보기로 한 것」인지 「내 돈이 놓인 것」인지는
화면이 알고, 이 카드는 `title`·`subjectLabel`로만 안다.

종목별 요청이라 앞 6종목까지만 본다. 목록 순서를 그대로 받으므로 관심 화면에서는
사용자가 위에 둔 종목이, 보유 화면에서는 비중이 큰 종목이 먼저 조회된다 — 비중 1%
종목의 어제 기사는 이 카드가 있어야 할 이유가 아니다.

종목당 최신 2건씩 모아 시각순으로 섞는다. 한 종목이 목록을 다 채우면 나머지 종목에
아무 일도 없었던 것처럼 보인다.

**종목당 1요청은 남는다.** 뉴스 배치 엔드포인트는 이 세션의 범위 밖이다(사용자 지시:
BE 변경은 2차).
*/
const MAX_SYMBOLS = 6;
const PER_SYMBOL = 2;

export function SymbolNewsCard({
  symbols,
  title,
  subjectLabel,
}: {
  /** 이 카드가 훑을 종목 — **부르는 쪽이 정한 순서 그대로** 앞에서부터 여섯을 본다. */
  symbols: Array<{ symbol: string; name: string; market: "domestic" | "us" }>;
  /** 카드와 모달의 제목. 화면마다 이 종목들이 무엇인지가 다르다. */
  title: string;
  /** 모달 안에서 목록의 출처를 적는 말 — 「관심종목 상위」·「보유 비중 상위」. */
  subjectLabel: string;
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
        title={title}
        action={
          <button
            type="button"
            onClick={() => setModalOpen(true)}
            className="flex shrink-0 items-center gap-0.5 text-cell text-muted-foreground hover:text-foreground"
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
                  <p className="line-clamp-2 text-cell leading-snug">
                    {a.headlineKo || a.headline}
                  </p>
                  <p
                    className="mt-0.5 truncate text-micro text-muted-foreground"
                    suppressHydrationWarning
                  >
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
        title={title}
        subjectLabel={subjectLabel}
      />

      {newsModal.modal}
    </>
  );
}
