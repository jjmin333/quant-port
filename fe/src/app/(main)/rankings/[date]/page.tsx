import Link from "next/link";
import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { PageScrollShell } from "@/components/layout/page-content-shell";
import { JsonLd } from "@/components/seo/json-ld";
import { breadcrumbJsonLd, NOINDEX, pageMetadata } from "@/lib/seo";
import {
  DAILY_TOP_DATE_RE,
  fetchDailyTopServer,
  koreanMonthDay,
  type DailyTopRow,
} from "@/lib/daily-top.server";

/*
날짜별 「AI 점수 상위」 보관 주소 (2026-10-21 티켓 QP-12, 참고: Danelfin 「오늘의 상위 점수」).
서버가 다 그린다 — 크롤러가 읽을 본문과 종목 링크가 HTML에 있다. 보관 목록에 없는 날짜는 404.
*/
type Props = { params: Promise<{ date: string }> };

async function load(date: string) {
  if (!DAILY_TOP_DATE_RE.test(date)) return null;
  const data = await fetchDailyTopServer(date);
  return data?.date === date ? data : null;
}

export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const { date } = await params;
  const data = await load(date);
  if (!data) return { title: "찾을 수 없는 날짜", robots: NOINDEX };
  const day = koreanMonthDay(date);
  const top = [...data.domestic.slice(0, 2), ...data.overseas.slice(0, 2)].map((r) => r.name).join(", ");
  return pageMetadata({
    title: `${date.slice(0, 4)}년 ${day} AI 점수 상위 종목`,
    description: `${date} 기준 국내·미국 AI 종합 점수 상위 20종목과 전 거래일 대비 순위 변화. 상위: ${top}. 투자 권유 아님.`,
    path: `/rankings/${date}`,
  });
}

function RankMove({ row }: { row: DailyTopRow }) {
  if (!row.prevRank) return <span className="text-micro text-[var(--brand)]">신규</span>;
  const d = row.prevRank - row.rank;
  if (d === 0) return <span className="text-micro text-[var(--text-muted)]">–</span>;
  return (
    <span className={d > 0 ? "text-micro text-[var(--profit)]" : "text-micro text-[var(--loss)]"}>
      {d > 0 ? `▲${d}` : `▼${-d}`}
    </span>
  );
}

function TopTable({ title, rows }: { title: string; rows: DailyTopRow[] }) {
  if (rows.length === 0) return null;
  return (
    <section className="min-w-0 space-y-1.5">
      <h2 className="text-cell font-semibold text-[var(--text-secondary)]">{title}</h2>
      <ol className="divide-y divide-[var(--border)] rounded-lg border border-[var(--border)]">
        {rows.map((r) => (
          <li key={r.symbol} className="flex items-center gap-2 px-3 py-1.5 text-cell">
            <span className="w-6 shrink-0 tabular-nums text-[var(--text-muted)]">{r.rank}</span>
            <Link
              href={`/stocks/${encodeURIComponent(r.symbol)}`}
              prefetch={false}
              className="min-w-0 flex-1 truncate font-medium text-[var(--text-primary)] hover:underline"
            >
              {r.name}
            </Link>
            <span className="shrink-0 tabular-nums font-semibold text-[var(--text-primary)]">{r.score}점</span>
            <span className="w-8 shrink-0 text-right">
              <RankMove row={r} />
            </span>
          </li>
        ))}
      </ol>
    </section>
  );
}

export default async function DailyTopPage({ params }: Props) {
  const { date } = await params;
  const data = await load(date);
  if (!data) notFound();
  const path = `/rankings/${date}`;
  const nearby = data.dates.filter((d) => d !== date).slice(0, 10);

  return (
    <>
      <JsonLd
        data={breadcrumbJsonLd([
          { name: "AI 점수 순위", path: "/analysis" },
          { name: `${date} 상위 종목`, path },
        ])}
      />
      <PageScrollShell contentClassName="space-y-4">
        <header className="space-y-1">
          <h1 className="text-page font-semibold text-[var(--text-primary)]">
            {date.slice(0, 4)}년 {koreanMonthDay(date)} AI 점수 상위 종목
          </h1>
          <p className="text-cell text-[var(--text-muted)]">
            그날 장 마감 뒤 매긴 종합 점수 기준 국내·미국 상위 20종목입니다. 화살표는 전 거래일
            {data.prevDate ? `(${data.prevDate})` : ""} 대비 순위 변화, 「신규」는 전날 20위 밖이었던 종목입니다.
          </p>
        </header>
        <div className="grid gap-4 md:grid-cols-2">
          <TopTable title="국내" rows={data.domestic} />
          <TopTable title="미국" rows={data.overseas} />
        </div>
        <p className="text-micro text-[var(--text-muted)]">
          점수는 같은 시장·업종 안에서의 상대 위치이며 매수·매도 권유가 아닙니다.{" "}
          <Link href="/analysis" prefetch={false} className="text-[var(--brand)] hover:underline">
            지금 순위 보기
          </Link>
        </p>
        <nav aria-label="다른 날짜" className="space-y-1.5">
          <div className="flex gap-3 text-cell">
            {data.prevDate && (
              <Link href={`/rankings/${data.prevDate}`} prefetch={false} className="text-[var(--brand)] hover:underline">
                ← {data.prevDate}
              </Link>
            )}
            {data.nextDate && (
              <Link href={`/rankings/${data.nextDate}`} prefetch={false} className="ml-auto text-[var(--brand)] hover:underline">
                {data.nextDate} →
              </Link>
            )}
          </div>
          {nearby.length > 0 && (
            <ul className="flex flex-wrap gap-1.5">
              {nearby.map((d) => (
                <li key={d}>
                  <Link
                    href={`/rankings/${d}`}
                    prefetch={false}
                    className="inline-flex rounded-full border border-[var(--border)] px-2 py-0.5 text-cell tabular-nums text-[var(--text-secondary)] hover:bg-[var(--surface-elevated)]"
                  >
                    {d}
                  </Link>
                </li>
              ))}
            </ul>
          )}
        </nav>
      </PageScrollShell>
    </>
  );
}
