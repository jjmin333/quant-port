import Link from "next/link";
import { PageScrollShell } from "@/components/layout/page-content-shell";
import { resolvePageMetadata } from "@/lib/page-meta.server";
import { breadcrumbJsonLd } from "@/lib/seo";
import { JsonLd } from "@/components/seo/json-ld";
import { groupBySector, sectorPath, stylePath } from "@/lib/stock-seo";
import { STOCK_STYLES } from "@/lib/stock-style";
import { fetchIndexableStocks } from "@/lib/stock-seo.server";

/*
 * 종목 모음 — 색인 대상 종목(시장별 거래대금 상위 500씩)을 섹터별 링크로 (2026-10-09 사용자 결정).
 *
 * 이 화면이 있기 전에는 종목 페이지 1,000개로 오는 서버 렌더 링크가 사이트 어디에도 없었다
 * (성장 감시 크롤: 1,000/1,000 고아). 사이트맵에만 있는 주소를 구글은 「발견됨 – 색인 안 됨」으로
 * 미룬다. 그래서 **전부 서버 HTML의 <a>로** 펼쳐 둔다 — 탭·접기 없이.
 *
 * `prefetch={false}`: 링크가 1,000개라 화면에 보이는 것만 미리 받아도 BE가 버티지 못한다.
 * 목록은 `fetchIndexableStocks`가 1시간 접는다(레이아웃이 요청마다 그려 페이지 revalidate는 무의미).
 */

export function generateMetadata() {
  return resolvePageMetadata("/stocks");
}

const MARKETS = [
  { assetType: "domestic", id: "kr", label: "국내" },
  { assetType: "us", id: "us", label: "미국" },
] as const;

export default async function StocksHubPage() {
  const stocks = await fetchIndexableStocks();

  return (
    <>
      <JsonLd data={breadcrumbJsonLd([{ name: "종목 모음", path: "/stocks" }])} />
      <PageScrollShell contentClassName="space-y-5">
        <header className="space-y-1">
          <h1 className="text-page font-semibold text-[var(--text-primary)]">종목 모음</h1>
          <p className="text-cell text-[var(--text-muted)]">
            최근 20거래일 거래대금 상위 종목을 시장·섹터별로 모았습니다. 매일 갱신됩니다.
          </p>
          <nav className="flex gap-1.5 pt-1" aria-label="시장">
            {MARKETS.map((m) => (
              <a
                key={m.id}
                href={`#${m.id}`}
                className="rounded-full border border-[var(--border)] px-2.5 py-0.5 text-cell text-[var(--text-secondary)] hover:bg-[var(--surface-elevated)]"
              >
                {m.label}
              </a>
            ))}
          </nav>
          <nav className="flex flex-wrap items-center gap-1.5 pt-1" aria-label="스타일">
            <span className="text-micro text-[var(--text-muted)]">스타일로 보기</span>
            {STOCK_STYLES.map((st) => (
              <Link
                key={st.slug}
                href={stylePath(st.slug)}
                prefetch={false}
                className="rounded-full border border-[var(--border)] px-2.5 py-0.5 text-cell text-[var(--text-secondary)] hover:bg-[var(--surface-elevated)]"
              >
                {st.label}
              </Link>
            ))}
          </nav>
        </header>

        {stocks.length === 0 && (
          <p className="text-sm text-[var(--text-muted)]">종목 목록을 불러오지 못했습니다. 잠시 후 다시 열어 주세요.</p>
        )}

        {MARKETS.map((m) => {
          const groups = groupBySector(stocks, m.assetType);
          if (groups.length === 0) return null;
          return (
            <section key={m.id} id={m.id} className="scroll-mt-4 space-y-4">
              <h2 className="text-base font-semibold text-[var(--text-primary)]">{m.label}</h2>
              {groups.map((g) => (
                <div key={g.sector} className="space-y-1.5">
                  <h3 className="text-cell font-semibold text-[var(--text-secondary)]">
                    {g.sector} <span className="font-normal text-[var(--text-muted)]">({g.stocks.length})</span>
                    {g.sector !== "기타" && (
                      <Link
                        href={sectorPath(m.assetType, g.sector)}
                        prefetch={false}
                        className="ml-2 text-micro font-normal text-[var(--brand)] hover:underline"
                      >
                        점수 순위 →
                      </Link>
                    )}
                  </h3>
                  <ul className="flex flex-wrap gap-1.5">
                    {g.stocks.map((s) => (
                      <li key={s.symbol}>
                        <Link
                          href={`/stocks/${encodeURIComponent(s.symbol)}`}
                          prefetch={false}
                          className="inline-flex items-center gap-1 rounded-full border border-[var(--border)] px-2 py-0.5 text-cell text-[var(--text-primary)] hover:bg-[var(--surface-elevated)]"
                        >
                          {s.name}
                          {s.grade && <span className="text-micro text-[var(--text-muted)]">{s.grade}</span>}
                        </Link>
                      </li>
                    ))}
                  </ul>
                </div>
              ))}
            </section>
          );
        })}
      </PageScrollShell>
    </>
  );
}
