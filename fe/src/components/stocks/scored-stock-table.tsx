import Link from "next/link";
import type { IndexableScored } from "@/lib/stock-seo.server";
import { stockStyle, type StockAxes } from "@/lib/stock-style";
import { stylePath } from "@/lib/stock-seo";

/*
 * 업종 순위·스타일 목록이 함께 쓰는 **서버 렌더 표** (2026-10-10 사업화 검토 — 롱테일 페이지).
 * 크롤러가 읽는 본문이 이 표다. 종목 이름은 전부 `<a href=/stocks/…>` — 색인 대상끼리만 잇는다.
 */

const AXIS_LABEL: Record<string, string> = {
  technical: "추세",
  quality: "수익성",
  growth: "성장",
  valuation: "가치",
  sentiment: "심리·수급",
};

function extremes(axes: StockAxes): string {
  const list = Object.entries(axes)
    .filter((e): e is [string, number] => typeof e[1] === "number" && AXIS_LABEL[e[0]] != null)
    .sort((a, b) => b[1] - a[1]);
  if (list.length < 2) return "";
  const [hi, lo] = [list[0], list[list.length - 1]];
  return `강 ${AXIS_LABEL[hi[0]]} ${Math.round(hi[1])} · 약 ${AXIS_LABEL[lo[0]]} ${Math.round(lo[1])}`;
}

export function ScoredStockTable({ rows, showStyle = true }: { rows: IndexableScored[]; showStyle?: boolean }) {
  return (
    <div className="overflow-x-auto">
      <table className="w-full min-w-[480px] text-cell">
        <thead className="text-micro text-[var(--text-muted)]">
          <tr className="border-b border-[var(--border)] text-left">
            <th className="py-1.5 pr-2 font-normal">#</th>
            <th className="py-1.5 pr-2 font-normal">종목</th>
            <th className="py-1.5 pr-2 text-right font-normal">AI 점수</th>
            <th className="py-1.5 pr-2 font-normal">가장 강한 · 약한 축</th>
            {showStyle && <th className="py-1.5 font-normal">스타일</th>}
          </tr>
        </thead>
        <tbody>
          {rows.map((r, i) => {
            const style = showStyle ? stockStyle(r.axes) : null;
            return (
              <tr key={r.symbol} className="border-b border-[var(--border)] last:border-0">
                <td className="py-1.5 pr-2 tabular-nums text-[var(--text-muted)]">{i + 1}</td>
                <td className="py-1.5 pr-2">
                  <Link
                    href={`/stocks/${encodeURIComponent(r.symbol)}`}
                    prefetch={false}
                    className="font-medium text-[var(--text-primary)] hover:underline"
                  >
                    {r.name}
                  </Link>{" "}
                  <span className="text-micro text-[var(--text-muted)]">{r.symbol}</span>
                </td>
                <td className="py-1.5 pr-2 text-right tabular-nums">
                  <b className="text-[var(--text-primary)]">{Math.round(r.score)}</b>
                  {r.grade && <span className="ml-1 text-micro text-[var(--text-muted)]">{r.grade}</span>}
                </td>
                <td className="py-1.5 pr-2 text-[var(--text-secondary)]">{extremes(r.axes)}</td>
                {showStyle && (
                  <td className="py-1.5">
                    {style && (
                      <Link href={stylePath(style.slug)} prefetch={false} className="text-[var(--brand)] hover:underline">
                        {style.label}
                      </Link>
                    )}
                  </td>
                )}
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}

/** 목록 페이지 아래 공통 고지 — 점수가 무엇이고 무엇이 아닌지. */
export function ScoreListFootnote({ asOf }: { asOf?: string }) {
  return (
    <p className="text-micro leading-relaxed text-[var(--text-muted)]">
      대상: 최근 20거래일 거래대금 상위 국내·미국 각 500종목. AI 점수는 수익성·성장·가치·추세·심리·수급 다섯 축을 같은
      시장·업종 안 백분위로 매긴 값이며 매일 00:25에 갱신됩니다{asOf ? `(${asOf} 기준)` : ""}. 과거 성적은{" "}
      <Link href="/analysis?backtest=1" prefetch={false} className="text-[var(--brand)] hover:underline">
        점수 성적표
      </Link>
      에 그대로 공개합니다. 투자 권유가 아닙니다.
    </p>
  );
}
