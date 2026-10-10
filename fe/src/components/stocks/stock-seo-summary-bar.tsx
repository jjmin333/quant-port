import Link from "next/link";
import type { FundamentalsData } from "@/lib/fundamentals-types";
import { answerEarnings, type StockQA } from "@/lib/stock-qa";
import type { EarningsTabData } from "@/lib/earnings-types";
import { comparePath, sectorPath, stockSummarySentence, stylePath, type StockSeoSummary } from "@/lib/stock-seo";
import { stockStyle } from "@/lib/stock-style";
import type { IndexableStock } from "@/lib/stock-seo.server";

/*
 * 종목 페이지 맨 위의 **접힌 요약 바** (2026-10-09 사용자 결정 — 검색 색인 개선 C1).
 *
 * 예전에는 요약 한 줄(평균 209자)이 서버 HTML의 본문 전부였다 — 1,000개 페이지가 거의 같은 틀로
 * 보여 구글이 「크롤링됨 – 색인 안 됨」으로 미뤘다. 이제 지표·최근 변화·같은 섹터 종목을 서버가
 * 그린다. 기본은 접힘이라 화면 높이는 그대로고, 접힌 <details> 안의 글도 검색엔진은 읽는다.
 * 숨긴 글자가 아니라 사용자가 펼쳐 볼 수 있는 글이어야 한다(숨김은 속임수로 본다).
 *
 * 같은 섹터 링크는 **색인 대상 안에서만** 건다 — noindex 페이지로 링크를 흘리지 않는다.
 */

type Metric = { label: string; value: string };

const AXES = [
  ["technical", "추세"],
  ["quality", "수익성"],
  ["growth", "성장"],
  ["valuation", "가치"],
  ["sentiment", "심리·수급"],
] as const;

function fixed(v: number | undefined, digits: number, suffix: string, signed = false): string | null {
  if (v == null || !Number.isFinite(v)) return null;
  return `${signed && v > 0 ? "+" : ""}${v.toFixed(digits)}${suffix}`;
}

function metrics(summary: StockSeoSummary, f: FundamentalsData | null): Metric[] {
  const rows: [string, string | null][] = [
    ["AI 점수", summary.score == null ? null : `${Math.round(summary.score)}${summary.grade ? ` · ${summary.grade}` : ""}`],
    // 적자 기업의 음수 PER은 「싸다」로 읽히므로 싣지 않는다.
    ["PER", f?.per != null && f.per > 0 ? fixed(f.per, 1, "배") : null],
    ["PBR", f?.pbr != null && f.pbr > 0 ? fixed(f.pbr, 2, "배") : null],
    ["ROE", fixed(f?.roe, 1, "%")],
    ["영업이익률", fixed(f?.operatingMargin, 1, "%")],
    ["매출 성장(전년)", fixed(f?.revenueGrowthYoy, 1, "%", true)],
  ];
  return rows.filter((r): r is [string, string] => r[1] != null).map(([label, value]) => ({ label, value }));
}

function shortDate(d: string): string {
  const [, m, day] = d.split("-");
  return m && day ? `${Number(m)}/${Number(day)}` : d;
}

export function StockSeoSummaryBar({
  summary,
  qa,
  fundamentals,
  earnings,
  overview,
  peers,
}: {
  summary: StockSeoSummary;
  qa: StockQA | null;
  fundamentals: FundamentalsData | null;
  earnings?: EarningsTabData | null;
  overview?: string | null;
  peers: IndexableStock[];
}) {
  const sentence = stockSummarySentence(summary);
  const items = metrics(summary, fundamentals);
  const changes = (qa?.changes ?? []).slice(0, 3);
  const earningsLines = earnings ? answerEarnings(earnings).lines : [];
  const style = stockStyle(summary.axes);
  const axes = AXES.filter(([k]) => typeof summary.axes?.[k] === "number");

  return (
    <details className="group/seo shrink-0 border-b border-[var(--border)]">
      <summary
        className="flex cursor-pointer list-none items-center gap-2 px-3 py-1 text-micro text-[var(--text-muted)] [&::-webkit-details-marker]:hidden"
        title={sentence}
      >
        {/* 펼치면 같은 줄이 전체 문장으로 풀린다 — 본문에 다시 적지 않는다. */}
        <span className="min-w-0 flex-1 truncate group-open/seo:whitespace-normal group-open/seo:text-[var(--text-secondary)]">
          {sentence}
        </span>
        <span className="shrink-0 text-[var(--brand)]">
          <span className="group-open/seo:hidden">요약 펼치기 ▾</span>
          <span className="hidden group-open/seo:inline">접기 ▴</span>
        </span>
      </summary>
      <div className="space-y-3 px-3 pb-3 pt-1 text-cell">
        {items.length > 0 && (
          <dl className="grid grid-cols-3 gap-x-3 gap-y-2 sm:grid-cols-6">
            {items.map((m) => (
              <div key={m.label} className="flex flex-col-reverse">
                <dt className="text-micro text-[var(--text-muted)]">{m.label}</dt>
                <dd className="font-semibold tabular-nums text-[var(--text-primary)]">{m.value}</dd>
              </div>
            ))}
          </dl>
        )}
        {axes.length > 0 && (
          <section>
            <h2 className="mb-1 text-micro font-semibold text-[var(--text-muted)]">
              {summary.name} AI 점수 다섯 축
              {style && (
                <>
                  {" · "}
                  <Link href={stylePath(style.slug)} prefetch={false} className="text-[var(--brand)] hover:underline">
                    {style.label}
                  </Link>
                </>
              )}
            </h2>
            <p className="text-[var(--text-secondary)]">
              {axes.map(([k, label]) => `${label} ${Math.round(summary.axes![k] as number)}점`).join(" · ")}
              <span className="text-[var(--text-muted)]"> — 같은 시장·업종 안 백분위(100이 가장 좋음, 가치는 쌀수록 높음)</span>
            </p>
          </section>
        )}
        {overview && (
          <section>
            <h2 className="mb-1 text-micro font-semibold text-[var(--text-muted)]">{summary.name} AI 요약</h2>
            <p className="leading-relaxed text-[var(--text-secondary)]">{overview}</p>
          </section>
        )}
        {earningsLines.length > 0 && (
          <section>
            <h2 className="mb-1 text-micro font-semibold text-[var(--text-muted)]">{summary.name} 실적 발표</h2>
            <ul className="space-y-0.5 text-[var(--text-secondary)]">
              {earningsLines.map((line) => (
                <li key={line}>{line}</li>
              ))}
            </ul>
          </section>
        )}
        {changes.length > 0 && (
          <section>
            <h2 className="mb-1 text-micro font-semibold text-[var(--text-muted)]">최근 변화</h2>
            <ul className="space-y-0.5 text-[var(--text-secondary)]">
              {changes.map((c, i) => (
                <li key={`${c.date}-${i}`}>
                  <span className="tabular-nums text-[var(--text-muted)]">{shortDate(c.date)}</span> {c.title}
                </li>
              ))}
            </ul>
          </section>
        )}
        {peers.length > 0 && (
          <section>
            <h2 className="mb-1 text-micro font-semibold text-[var(--text-muted)]">
              같은 섹터{summary.sectorLabel ? `(${summary.sectorLabel})` : ""} 종목
              {summary.sectorLabel && (
                <>
                  {" · "}
                  <Link
                    href={sectorPath(summary.assetType, summary.sectorLabel)}
                    prefetch={false}
                    className="font-normal text-[var(--brand)] hover:underline"
                  >
                    {summary.sectorLabel} 업종 점수 순위
                  </Link>
                </>
              )}
            </h2>
            <ul className="flex flex-wrap gap-1.5">
              {peers.map((p) => (
                <li key={p.symbol}>
                  <Link
                    href={`/stocks/${encodeURIComponent(p.symbol)}`}
                    prefetch={false}
                    className="inline-flex items-center gap-1 rounded-full border border-[var(--border)] px-2 py-0.5 text-[var(--text-primary)] hover:bg-[var(--surface-elevated)]"
                  >
                    {p.name}
                    {p.grade && <span className="text-micro text-[var(--text-muted)]">{p.grade}</span>}
                  </Link>
                </li>
              ))}
            </ul>
            <p className="mt-1.5 flex flex-wrap gap-x-3 gap-y-1">
              {peers.slice(0, 3).map((p) => (
                <Link
                  key={p.symbol}
                  href={comparePath(summary.symbol, p.symbol)}
                  prefetch={false}
                  className="text-[var(--brand)] hover:underline"
                >
                  {summary.name} vs {p.name}
                </Link>
              ))}
            </p>
          </section>
        )}
      </div>
    </details>
  );
}
