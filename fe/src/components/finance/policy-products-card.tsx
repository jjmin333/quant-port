import { ChevronDown } from "lucide-react";
import { Card } from "@/components/ui/card";
import { YOUTH_FUTURE_SAVINGS as P } from "@/lib/policy-products";

const maxRate = Math.max(...P.banks.map((b) => b.max));

/**
 * 정부 정책상품 — 금감원 공시 API에 없는 청년미래적금을 은행연합회 공시 기준으로 보여 준다.
 * 세후 이자 순위에는 섞지 않는다(정부 저축장려금·비과세가 붙는 다른 계산이라 같은 줄에 세우면 틀린다).
 */
export function PolicyProductsCard() {
  return (
    <Card className="border-l-4 border-l-[var(--brand)] bg-[var(--brand-muted)] p-3 sm:p-4">
      <details className="group">
        <summary className="flex cursor-pointer list-none flex-col gap-2 [&::-webkit-details-marker]:hidden sm:flex-row sm:items-center sm:gap-4">
          <div className="min-w-0 flex-1 space-y-1">
            <div className="flex flex-wrap items-center gap-2">
              <span className="rounded-full bg-[var(--brand)] px-2 py-0.5 text-micro font-semibold text-white">정부 정책상품</span>
              <span className="text-micro text-[var(--text-muted)]">금융감독원 공시에 없는 상품 · 은행연합회 공시 기준</span>
            </div>
            <div className="text-base font-bold sm:text-lg">
              {P.name} <span className="text-[var(--brand)]">최고 연 {maxRate}%</span>
            </div>
            <div className="text-cell text-[var(--text-secondary)]">
              만 19~34세 청년 · 월 최대 50만원 · 3년 · 정부 저축장려금 + 이자 비과세
            </div>
          </div>
          <span className="inline-flex shrink-0 items-center justify-center gap-1 rounded-md bg-[var(--brand)] px-3 py-2 text-cell font-semibold text-white">
            <span className="group-open:hidden">은행 {P.banks.length}곳 금리 비교 보기</span>
            <span className="hidden group-open:inline">접기</span>
            <ChevronDown className="size-4 transition-transform group-open:rotate-180" aria-hidden />
          </span>
        </summary>

        <dl className="mt-3 grid gap-x-4 gap-y-1 text-micro sm:grid-cols-[auto_1fr]">
          {P.overview.map(([k, v]) => (
            <div key={k} className="contents">
              <dt className="text-[var(--text-muted)]">{k}</dt>
              <dd>{v}</dd>
            </div>
          ))}
        </dl>

        <ul className="mt-3 divide-y rounded-lg border">
          {P.banks.map((b) => (
            <li key={b.name}>
              <details className="group">
                <summary className="flex cursor-pointer items-center gap-3 px-3 py-2 text-cell">
                  <span className="min-w-0 flex-1 truncate font-medium">{b.name}</span>
                  <span className="text-[var(--text-muted)]">기본 {b.base.toFixed(2)}%</span>
                  <span className="w-24 text-right font-semibold">최고 {b.max.toFixed(2)}%</span>
                </summary>
                <div className="space-y-2 px-3 pb-3 text-micro text-[var(--text-secondary)]">
                  <ul className="list-disc space-y-0.5 pl-4">
                    {b.perks.map((x) => <li key={x}>{x}</li>)}
                  </ul>
                  <p><span className="text-[var(--text-muted)]">만기 후 금리 </span>{b.after}</p>
                  <p className="text-[var(--text-muted)]">은행 최종 제공일 {b.provided}</p>
                </div>
              </details>
            </li>
          ))}
        </ul>

        <p className="mt-2 text-micro text-[var(--text-muted)]">
          {P.pending} 우대조건 전문·상세는{" "}
          <a href={P.sourceUrl} target="_blank" rel="noopener noreferrer" className="underline">{P.sourceLabel}</a>
          의 청년미래적금 금리비교공시에서 확인하세요. 기준일 {P.asOf} · 금융회사·정부 어디에서도 대가를 받지 않으며 가입 연결 링크는 없습니다.
        </p>
      </details>
    </Card>
  );
}
