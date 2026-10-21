"use client";

import Link from "next/link";
import { Loader2 } from "lucide-react";
import { isPlainLeftClick } from "@/lib/plain-click";

import { PriceDisplay } from "@/components/ui/price-display";
import { StockAvatar } from "@/components/stocks/stock-avatar";
import type { QuoteAssetType, QuoteCurrency } from "@/lib/quotes-types";
import {
  formatChangeRate,
  formatCount,
  formatFieldValue,
  funnelSteps,
  likeDiffs,
  type ScreenCatalogField,
  type ScreenResult,
  type ScreenSymbolValues,
} from "@/lib/screener-api";
import { sectorLabel } from "@/lib/score-explain-api";
import { priceAsOfLabel, SCORE_TONE_TEXT, scoreTone } from "@/lib/stock-score-api";
import { cn } from "@/lib/utils";

/**
 * 결과 — 깔때기 + 표 (V2 요구 4).
 *
 * # 깔때기가 이 화면의 핵심입니다
 *
 * 결과 수만 보여 주면 0이 나왔을 때 사용자는 **시장을 탓합니다.** 어디서 몇이 빠졌는지
 * 말해야 「조건을 고칠지」와 「우리가 그 값을 아직 못 잰다」를 구분할 수 있습니다.
 *
 *   전체 → 시장 → 점수 자격 → 잴 수 있음 → 조건 통과
 *
 * 가운데 두 칸이 **재보지 못해서** 빠진 자리라 색이 다릅니다. 마지막 칸만이 진짜
 * 「조건을 통과한」 수입니다.
 *
 * 모바일에서는 앞 두 칸을 접습니다 — 다섯 칸은 375px에서 어느 것도 못 읽습니다.
 */

export function ScreenerResults({
  result,
  loading,
  error,
  fields,
  like,
  hitFields,
  cards = false,
  onSelect,
  className,
}: {
  result: ScreenResult | null;
  loading: boolean;
  error?: string | null;
  /** 결과표의 열 정의 — 카탈로그에서 받은 라벨·단위·자릿수를 씁니다. */
  fields: Map<string, ScreenCatalogField>;
  /** 「닮은꼴」 기준 종목 — 있으면 행마다 나은 점·모자란 점 칩(2026-10-01 결정 B). */
  like?: ScreenSymbolValues | null;
  /** 실제로 거른 조건의 필드 — 그 칸의 값은 파랗게(「왜 걸렸는지」, 2026-10-09). */
  hitFields?: Set<string>;
  /** 좁은 화면 — 표 대신 종목 카드. */
  cards?: boolean;
  onSelect?: (symbol: string) => void;
  className?: string;
}) {
  if (loading) {
    return (
      <div className={cn("flex items-center gap-1.5 p-3 text-cell text-[var(--text-muted)]", className)}>
        <Loader2 className="h-3 w-3 animate-spin" />
        조건에 맞는 종목을 찾는 중입니다…
      </div>
    );
  }
  if (error) {
    /* 「불러오지 못했습니다」와 「조건에 맞는 종목이 없습니다」는 다른 상태입니다. */
    return <div className={cn("p-3 text-cell text-[var(--text-muted)]", className)}>{error}</div>;
  }
  if (!result) {
    return (
      <div className={cn("p-3 text-cell text-[var(--text-muted)]", className)}>
        조건을 정하고 찾기를 누르시면 결과가 나옵니다.
      </div>
    );
  }

  /*
    종합 점수는 **전용 열이 이미 있습니다.**

    조건에 건 필드는 전부 결과표의 열이 되는데, 종합 점수를 걸면 고정 「점수」 열과
    똑같은 값이 나란히 두 번 섭니다. 실측에서 「88 88」이 그대로 보였습니다.
  */
  const cellFields = (result.rows[0]?.cells ?? [])
    .map((c) => c.field)
    .filter((id) => id !== "score");

  return (
    <div className={cn("flex min-h-0 flex-col", className)}>
      <Funnel result={result} />
      {like ? (
        <p className="border-b border-[var(--border)] bg-[var(--brand)]/5 px-2 py-1.5 text-micro text-[var(--text-secondary)]">
          <b className="text-[var(--brand)]">{like.name}</b>와 <b>닮은 순</b>
          {like.sector ? <> · {sectorLabel(like.sector)} 먼저</> : null} · 칩은 기준보다{" "}
          <span className="font-semibold text-[var(--brand)]">나은 점</span>·
          <span className="font-semibold text-[var(--text-muted)]">모자란 점</span>(5점 이상)
        </p>
      ) : null}

      {result.rows.length === 0 ? (
        <EmptyReason result={result} fields={fields} />
      ) : cards ? (
        <ul className="divide-y divide-[var(--border)]">
          {result.rows.map((row) => {
            const tone = scoreTone(row.score ?? 0);
            return (
              <li key={row.symbol}>
                {/* 종목 페이지로 가는 진짜 링크 — 일반 클릭은 옆 패널(2026-10-09 검색 색인 개선 B). */}
                <Link
                  href={`/stocks/${encodeURIComponent(row.symbol)}`}
                  prefetch={false}
                  onClick={(e) => {
                    if (!onSelect || !isPlainLeftClick(e)) return;
                    e.preventDefault();
                    onSelect(row.symbol);
                  }}
                  className="flex w-full flex-col gap-1 px-3 py-3 text-left hover:bg-[var(--surface-elevated)]"
                >
                  <span className="flex w-full items-center gap-2">
                    <StockAvatar symbol={row.symbol} name={row.name} currency={row.currency as QuoteCurrency | undefined} assetType={row.assetType as QuoteAssetType | undefined} size="xs" />
                    <span className="min-w-0 flex-1 truncate text-sm font-semibold text-[var(--text-primary)]">
                      {row.name || row.symbol}
                    </span>
                    <span className={cn("shrink-0 text-sm font-bold tabular-nums", SCORE_TONE_TEXT[tone])}>{row.score ?? "-"}</span>
                  </span>
                  <span className="flex w-full flex-wrap items-baseline gap-x-3 gap-y-0.5 text-cell tabular-nums">
                    <span className="text-[var(--text-primary)]">
                      <PriceDisplay value={row.close ?? 0} currency={(row.currency ?? "KRW") as "KRW" | "USD"} size="inherit" />
                      <span
                        className={cn(
                          "ml-1",
                          (row.changeRate ?? 0) > 0 && "text-[var(--profit)]",
                          (row.changeRate ?? 0) < 0 && "text-[var(--loss)]",
                        )}
                      >
                        {formatChangeRate(row.changeRate)}
                      </span>
                    </span>
                    {(row.cells ?? [])
                      .filter((c) => c.field !== "score")
                      .slice(0, 4)
                      .map((c) => (
                        <span key={c.field} className="text-[var(--text-muted)]">
                          {fields.get(c.field)?.label ?? c.field}{" "}
                          <b className={cn("font-semibold", hitFields?.has(c.field) ? "text-[var(--brand)]" : "text-[var(--text-primary)]")}>
                            {formatFieldValue(c.value, fields.get(c.field))}
                          </b>
                        </span>
                      ))}
                  </span>
                  {like ? <LikeChips row={row} like={like} fields={fields} /> : null}
                </Link>
              </li>
            );
          })}
        </ul>
      ) : (
        <div className="min-h-0 flex-1 overflow-auto">
          <table className="w-full border-collapse text-note">
            <thead className="sticky top-0 z-[1] bg-[var(--card)]">
              <tr>
                {/* 순서: 현재 주가 · 등락률 · AI 점수 · 보기 탭 열(개요는 시가총액 · 20일 · 60일) — 2026-10-09 요청 3.
                    가격이 없으면 등락률만으로는 같은 값인지 확인할 수 없다(09-23 QA). */}
                <Th className="text-left">종목</Th>
                <Th>현재 주가</Th>
                <Th>등락률</Th>
                <Th>AI 점수</Th>
                {cellFields.map((id) => (
                  <Th key={id}>{fields.get(id)?.label ?? id}</Th>
                ))}
              </tr>
            </thead>
            <tbody>
              {result.rows.map((row) => {
                const tone = scoreTone(row.score ?? 0);
                return (
                  <tr
                    key={row.symbol}
                    onClick={onSelect ? () => onSelect(row.symbol) : undefined}
                    className={cn(
                      "border-b border-[var(--border)] last:border-b-0",
                      onSelect && "cursor-pointer hover:bg-[var(--surface-elevated)]",
                    )}
                  >
                    <td className="px-3 py-2.5 font-semibold text-[var(--text-primary)]">
                      {/* 이름은 종목 페이지 링크 — 일반 클릭은 행의 onClick(옆 패널)에 맡긴다. */}
                      <StockAvatar
                        symbol={row.symbol}
                        name={row.name}
                        currency={row.currency as QuoteCurrency | undefined}
                        assetType={row.assetType as QuoteAssetType | undefined}
                        size="xs"
                        className="mr-2 inline-flex align-middle"
                      />
                      <Link
                        href={`/stocks/${encodeURIComponent(row.symbol)}`}
                        prefetch={false}
                        onClick={(e) => {
                          if (onSelect && isPlainLeftClick(e)) e.preventDefault();
                          else e.stopPropagation();
                        }}
                        className="truncate hover:underline"
                      >
                        {row.name || row.symbol}
                      </Link>
                      <span className="ml-1.5 font-normal text-[var(--text-muted)]">{row.symbol}</span>
                      {like ? <LikeChips row={row} like={like} fields={fields} /> : null}
                    </td>
                    <td
                      className="px-3 py-2.5 text-right tabular-nums text-[var(--text-primary)]"
                      title={priceAsOfLabel(row.priceAsOf)}
                    >
                      {/* 랭킹 행과 **같은 컴포넌트**입니다 — 통화·자릿수 규칙이
                          화면마다 갈리지 않게 합니다. */}
                      <PriceDisplay
                        value={row.close ?? 0}
                        currency={(row.currency ?? "KRW") as "KRW" | "USD"}
                        size="inherit"
                      />
                    </td>
                    <td
                      className={cn(
                        "px-3 py-2.5 text-right tabular-nums",
                        (row.changeRate ?? 0) > 0 && "text-[var(--profit)]",
                        (row.changeRate ?? 0) < 0 && "text-[var(--loss)]",
                      )}
                      title={priceAsOfLabel(row.priceAsOf)}
                    >
                      {formatChangeRate(row.changeRate)}
                    </td>
                    <td className={cn("px-3 py-2.5 text-right font-bold tabular-nums", SCORE_TONE_TEXT[tone])}>
                      {row.score ?? "-"}
                    </td>
                    {(row.cells ?? []).filter((cell) => cell.field !== "score").map((cell) => (
                      <td
                        key={cell.field}
                        className={cn(
                          "px-3 py-2.5 text-right tabular-nums",
                          hitFields?.has(cell.field) && "font-bold text-[var(--brand)]",
                        )}
                      >
                        {/* 값이 없으면 「-」입니다 — 0과 뜻이 정반대입니다. */}
                        {formatFieldValue(cell.value, fields.get(cell.field))}
                      </td>
                    ))}
                  </tr>
                );
              })}
            </tbody>
          </table>

          <PriceBasis result={result} />

          {result.truncated ? (
            <p className="border-t border-[var(--border)] px-2 py-1.5 text-micro text-[var(--text-muted)]">
              조건을 통과한 {formatCount(result.funnel.passed)}종목 중 {formatCount(result.rows.length)}종목만
              보여 드립니다. 조건을 좁히시면 전부 보실 수 있습니다.
            </p>
          ) : null}
        </div>
      )}
    </div>
  );
}

/** 기준 종목 대비 축 차이 — 기준 자신은 「기준」 표시. */
function LikeChips({
  row,
  like,
  fields,
}: {
  row: ScreenResult["rows"][number];
  like: ScreenSymbolValues;
  fields: Map<string, ScreenCatalogField>;
}) {
  if (row.symbol === like.symbol) {
    return (
      <span className="ml-1.5 rounded-full bg-[var(--brand)] px-1.5 text-micro font-semibold text-[var(--brand-foreground)]">기준</span>
    );
  }
  const diffs = likeDiffs(row.cells, like.values, fields);
  if (diffs.length === 0) return <span className="block text-micro font-normal text-[var(--text-muted)]">거의 같음</span>;
  return (
    <span className="mt-0.5 flex flex-wrap gap-1">
      {diffs.map((d) => (
        <span
          key={d.field}
          className={cn(
            "rounded-full px-1.5 text-micro font-semibold tabular-nums",
            d.diff > 0 ? "bg-[var(--brand)]/10 text-[var(--brand)]" : "bg-[var(--surface-elevated)] text-[var(--text-muted)]",
          )}
        >
          {d.label} {d.diff > 0 ? "+" : "−"}
          {Math.abs(d.diff)}
        </span>
      ))}
    </span>
  );
}

/**
 * 이 표의 숫자가 **언제 것인가** (2026-09-23 QA P0-02).
 *
 * 화면에 날짜가 하나도 없었습니다. 점수는 하루 한 번 도는 배치의 것이고 가격은
 * 장중이면 지금 값이라 **기준이 둘**인데, 둘 다 안 적혀 있으면 사용자는 표 전체를
 * 한 시점의 것으로 읽습니다.
 *
 * 문구는 새로 만들지 않고 랭킹이 쓰던 것을 그대로 씁니다(`priceAsOfLabel`) —
 * 같은 뜻을 두 화면이 다른 말로 하면 그 자체가 불일치입니다.
 */
function PriceBasis({ result }: { result: ScreenResult }) {
  // 행마다 다를 수 있습니다(어떤 종목만 실시간). 하나라도 실시간이면 그렇게 적습니다.
  const anyLive = result.rows.some((r) => r.priceAsOf === "live");
  const priceBasis = anyLive
    ? "실시간 시세"
    : priceAsOfLabel(result.rows.find((r) => r.priceAsOf)?.priceAsOf)?.replace(/ 기준$/, "") ?? null;

  if (!result.tradeDate && !priceBasis) return null;

  return (
    <p className="border-t border-[var(--border)] px-2 py-1.5 text-micro text-[var(--text-muted)]">
      {priceBasis ? (
        <>
          가격 <b className="font-semibold text-[var(--text-primary)]">{priceBasis}</b>
        </>
      ) : null}
      {priceBasis && result.tradeDate ? <span aria-hidden> · </span> : null}
      {result.tradeDate ? (
        <>
          AI 점수 <b className="font-semibold text-[var(--text-primary)]">{result.tradeDate}</b> 기준
        </>
      ) : null}
    </p>
  );
}

function Th({ children, className }: { children: React.ReactNode; className?: string }) {
  return (
    <th
      className={cn(
        "border-b border-[var(--border)] px-3 py-2 text-right text-micro font-semibold text-[var(--text-muted)]",
        className,
      )}
    >
      {children}
    </th>
  );
}

function Funnel({ result }: { result: ScreenResult }) {
  const steps = funnelSteps(result.funnel, result.market);
  const compact = funnelSteps(result.funnel, result.market, true);
  const unmeasured = result.funnel.eligible - result.funnel.measurable;

  return (
    <div className="border-b border-[var(--border)] px-2 py-2">
      {/* 넓은 화면 — 다섯 칸 전부 */}
      <div className="hidden sm:flex">
        {steps.map((step, i) => (
          <FunnelCell key={step.key} step={step} first={i === 0} last={i === steps.length - 1} />
        ))}
      </div>
      {/* 좁은 화면 — 앞 두 칸을 접습니다 */}
      <div className="flex sm:hidden">
        {compact.map((step, i) => (
          <FunnelCell key={step.key} step={step} first={i === 0} last={i === compact.length - 1} />
        ))}
      </div>

      {unmeasured > 0 ? (
        <p className="mt-1.5 text-micro text-[var(--score-mid)]">
          {formatCount(unmeasured)}종목은 조건을 틀린 것이 아니라 그 값을 잴 수 없어 빠졌습니다.
        </p>
      ) : null}
    </div>
  );
}

function FunnelCell({
  step,
  first,
  last,
}: {
  step: ReturnType<typeof funnelSteps>[number];
  first: boolean;
  last: boolean;
}) {
  return (
    <div
      className={cn(
        "min-w-0 flex-1 border border-r-0 border-[var(--border)] px-1.5 py-1",
        first && "rounded-l-md",
        last && "rounded-r-md border-r",
        step.drop && "bg-[var(--score-mid)]/8",
        step.final && "bg-[var(--brand)]/10",
      )}
    >
      <span className="block truncate text-micro text-[var(--text-muted)]">{step.label}</span>
      <span
        className={cn(
          "block text-note font-bold tabular-nums",
          step.drop && "text-[var(--score-mid)]",
          step.final && "text-[var(--brand)]",
        )}
      >
        {formatCount(step.value)}
      </span>
    </div>
  );
}

/**
 * 0종목일 때 **왜** 0인지.
 *
 * 이 화면이 경쟁 서비스와 갈라지는 자리입니다. 커버리지가 가장 낮은 조건을 지목해
 * 주면 사용자가 무엇을 고쳐야 할지 압니다 — 실측에서 「심리·수급 60점 이상」을 더한
 * 순간 5종목이 0이 됐는데, 원인은 그 값이 국내 자격 종목의 12%에만 있다는 것이었습니다.
 */
function EmptyReason({
  result,
  fields,
}: {
  result: ScreenResult;
  fields: Map<string, ScreenCatalogField>;
}) {
  const worst = [...(result.coverage ?? [])].sort((a, b) => a.count - b.count)[0];
  const worstField = worst ? fields.get(worst.field) : undefined;
  const denom = result.funnel.eligible;

  return (
    <div className="p-3">
      <p className="text-cell font-bold text-[var(--text-primary)]">조건을 통과한 종목이 없습니다.</p>
      {worst && worstField && denom > 0 && worst.count / denom < 0.9 ? (
        <p className="mt-1.5 text-cell text-[var(--text-muted)]">
          가장 좁은 조건은 <b className="text-[var(--text-primary)]">{worstField.label}</b>입니다 — 이
          시장에서 {formatCount(denom)}종목 중 <b className="text-[var(--score-mid)]">{formatCount(worst.count)}종목</b>
          에만 값이 있습니다. 시장에 그런 종목이 없다기보다 <b>재본 적이 없는</b> 쪽에 가깝습니다.
        </p>
      ) : (
        <p className="mt-1.5 text-cell text-[var(--text-muted)]">
          값은 전부 잴 수 있었지만 조건을 만족하는 종목이 없었습니다. 범위를 넓혀 보시면 됩니다.
        </p>
      )}
    </div>
  );
}
