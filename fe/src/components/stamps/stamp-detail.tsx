"use client";

import Link from "next/link";
import { useState } from "react";
import { ExternalLink, Trash2 } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { analysisSymbolHref } from "@/lib/analysis-tabs";
import { formatDateKST, formatDateTimeKST, formatPercent, formatPrice, formatVolume } from "@/lib/format";
import { SCORE_AXIS_LABELS, SCORE_AXIS_ORDER, type ScoreAxisId } from "@/lib/stock-score-api";
import {
  BENCHMARK_LABELS,
  HORIZON_LABELS,
  STANCE_LABELS,
  dataGapLabel,
  type Stamp,
  type StampStance,
  type StampUpdateInput,
} from "@/lib/stamp-types";
import { cn } from "@/lib/utils";
import { StampSparkline } from "@/components/stamps/stamp-sparkline";
import { StampTargetGauge } from "@/components/stamps/stamp-target-gauge";

function currencyOf(stamp: Stamp): "KRW" | "USD" {
  return stamp.currency === "USD" ? "USD" : "KRW";
}

function toneClass(value: number | undefined): string {
  if (value === undefined) return "text-[var(--text-muted)]";
  if (value > 0) return "text-[var(--profit)]";
  if (value < 0) return "text-[var(--loss)]";
  return "text-[var(--text-secondary)]";
}

/** "—" is deliberate for a horizon that has not elapsed — never 0%. */
function pct(value: number | undefined): string {
  return value === undefined ? "—" : formatPercent(value);
}

export function StampDetail({
  stamp,
  readOnly = false,
  onUpdate,
  onDelete,
  onOpen,
}: {
  stamp: Stamp;
  /** 예시 데이터 — 서버에 없는 행이라 저장할 곳이 없다. 동작하지 않을 버튼을 그리지 않는다. */
  readOnly?: boolean;
  onUpdate: (input: StampUpdateInput) => Promise<void>;
  onDelete: () => void;
  onOpen: () => void;
}) {
  const [note, setNote] = useState(stamp.note);
  const [saving, setSaving] = useState(false);
  const currency = currencyOf(stamp);

  const patch = async (next: Partial<StampUpdateInput>) => {
    setSaving(true);
    try {
      await onUpdate({
        conviction: next.conviction ?? stamp.conviction,
        note: next.note ?? note,
        tags: stamp.tags,
      });
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="space-y-4">
      {stamp.due && !stamp.opened && !readOnly && (
        <div className="flex flex-wrap items-center gap-2 rounded-lg border border-[var(--brand)] bg-[var(--brand-muted)] px-3 py-2">
          <span className="text-xs text-[var(--text-primary)]">
            {stamp.targetHit
              ? `${formatDateKST(stamp.targetHit.date)} 장중 목표가에 도달해 적중으로 종료됐습니다.`
              : "검증 기간이 끝났습니다 — 결과를 확인하면 개봉 대기에서 사라집니다."}
          </span>
          <button
            type="button"
            onClick={onOpen}
            className="ml-auto rounded-md bg-[var(--brand)] px-2.5 py-1 text-[11px] font-medium text-white"
          >
            개봉 확인
          </button>
        </div>
      )}

      {/* 예측: 목표가와 만기는 스탬프의 채점 기준이라 수정할 수 없다. */}
      <SidePanel title="내 예측">
        <div className="grid gap-3 sm:grid-cols-[1fr_150px] sm:items-center">
          <div>
            <MetricRow
              label="예상 주가"
              value={`${formatPrice(stamp.targetPrice, currency)}${
                stamp.expectedReturn !== undefined
                  ? ` (${stamp.expectedReturn > 0 ? "+" : ""}${stamp.expectedReturn.toFixed(1)}%)`
                  : ""
              }`}
            />
            <MetricRow
              label="관점"
              value={`${STANCE_LABELS[stamp.stance]} · 확신 ${stamp.conviction}/5`}
            />
            <MetricRow
              label="검증 기간"
              value={`${HORIZON_LABELS[stamp.horizonKind]} · 개봉 ${formatDateTimeKST(stamp.dueAt)}`}
            />
            {stamp.targetAccuracy !== undefined && (
              <MetricRow
                label="목표가 오차"
                value={`${stamp.targetAccuracy.toFixed(1)}%`}
                valueClass={
                  stamp.targetAccuracy <= 3
                    ? "text-[var(--profit)]"
                    : stamp.targetAccuracy > 20
                      ? "text-amber-600 dark:text-amber-400"
                      : undefined
                }
              />
            )}
            {(stamp.peakReturn !== undefined || stamp.troughReturn !== undefined) && (
              <MetricRow
                label="이후 최고 / 최저"
                value={`${pct(stamp.peakReturn)} / ${pct(stamp.troughReturn)}`}
              />
            )}
          </div>
          <StampTargetGauge stamp={stamp} showTargetPrice />
        </div>
        <p className="mt-2 border-t border-[var(--border)] pt-1.5 text-[10px] text-[var(--text-muted)]">
          예상 주가와 검증 기간은 스탬프 직후에만 고칠 수 있습니다 — 결과를 보고 고칠 수 있으면
          적중률이 아무 뜻도 갖지 못합니다.
        </p>
      </SidePanel>

      {/* 당시 vs 현재 */}
      <div className="grid gap-3 md:grid-cols-2">
        <SidePanel title={`당시 · ${formatDateTimeKST(stamp.stampedAt)}`}>
          <MetricRow label="주가" value={formatPrice(stamp.price, currency)} />
          <MetricRow
            label="등락률"
            value={stamp.changeRate === undefined ? "—" : formatPercent(stamp.changeRate)}
            valueClass={toneClass(stamp.changeRate)}
          />
          <MetricRow label="거래량" value={formatVolume(stamp.volume)} />
          <MetricRow
            label="종합 점수"
            value={stamp.score == null ? "—" : `${stamp.score}${stamp.grade ? ` (${stamp.grade})` : ""}`}
          />
          {(stamp.snapshot.week52High ?? 0) > 0 && (
            <MetricRow
              label="52주 범위"
              value={`${formatPrice(stamp.snapshot.week52Low ?? 0, currency)} ~ ${formatPrice(stamp.snapshot.week52High ?? 0, currency)}`}
            />
          )}
          {stamp.snapshot.held && (
            <MetricRow
              label="보유"
              value={
                stamp.snapshot.holdingAvgPrice
                  ? `${formatVolume(stamp.snapshot.holdingQuantity ?? 0)}주 · 평단 ${formatPrice(stamp.snapshot.holdingAvgPrice, currency)}`
                  : "보유 중"
              }
            />
          )}
        </SidePanel>

        <SidePanel title="현재">
          <MetricRow
            label="주가"
            value={stamp.current ? formatPrice(stamp.current.price, currency) : "—"}
          />
          <MetricRow
            label="스탬프 대비"
            value={pct(stamp.current?.changeRate)}
            valueClass={toneClass(stamp.current?.changeRate)}
          />
          <MetricRow
            label="종합 점수"
            value={
              stamp.current?.score == null
                ? "—"
                : `${stamp.current.score}${stamp.current.grade ? ` (${stamp.current.grade})` : ""}`
            }
          />
          <MetricRow
            label="기준"
            value={stamp.current?.source === "quote" ? "실시간 시세" : `종가 ${stamp.current?.tradeDate ?? ""}`}
          />
        </SidePanel>
      </div>

      {/* 점수 5축 — 당시 대비 현재 */}
      {(stamp.score != null || stamp.current?.score != null) && (
        <SidePanel title="점수 5축 · 당시 → 현재">
          <div className="space-y-1.5">
            {SCORE_AXIS_ORDER.map((axis: ScoreAxisId) => {
              const then = stamp.snapshot.breakdown?.[axis];
              const now = stamp.current?.breakdown?.[axis];
              const delta = then !== undefined && now !== undefined ? now - then : undefined;
              return (
                <div key={axis} className="flex items-center gap-2">
                  <span className="w-14 shrink-0 text-[11px] text-[var(--text-muted)]">
                    {SCORE_AXIS_LABELS[axis]}
                  </span>
                  <div className="relative h-1.5 min-w-0 flex-1 rounded-full bg-[var(--surface-elevated)]">
                    {then !== undefined && (
                      <div
                        className="absolute inset-y-0 left-0 rounded-full bg-[var(--text-muted)]/50"
                        style={{ width: `${Math.max(2, Math.min(100, then))}%` }}
                      />
                    )}
                    {now !== undefined && (
                      <div
                        className="absolute inset-y-0 left-0 rounded-full bg-[var(--brand)]"
                        style={{ width: `${Math.max(2, Math.min(100, now))}%`, opacity: 0.75 }}
                      />
                    )}
                  </div>
                  <span className="w-20 shrink-0 text-right text-[11px] tabular-nums text-[var(--text-secondary)]">
                    {then === undefined ? "-" : Math.round(then)}
                    {" → "}
                    {now === undefined ? "-" : Math.round(now)}
                    {delta !== undefined && delta !== 0 && (
                      <span className={cn("ml-1", toneClass(delta))}>
                        {delta > 0 ? "▲" : "▼"}
                        {Math.abs(Math.round(delta))}
                      </span>
                    )}
                  </span>
                </div>
              );
            })}
          </div>
        </SidePanel>
      )}

      {/* 차트: 박제된 봉 + 이후 경로 */}
      {stamp.chart.bars.length > 0 && (
        <SidePanel title={`차트 · 스탬프 시점 (${stamp.chart.interval})`}>
          <StampSparkline stamp={stamp} />
        </SidePanel>
      )}

      {/* 기간별 수익률 */}
      <SidePanel title="기간별 수익률">
        <div className="overflow-x-auto">
          <table className="w-full min-w-[420px] text-[11px]">
            <thead>
              <tr className="text-left text-[var(--text-muted)]">
                <th className="py-1 font-medium">구간</th>
                <th className="py-1 font-medium">기준일</th>
                <th className="py-1 text-right font-medium">수익률</th>
                <th className="py-1 text-right font-medium">
                  {BENCHMARK_LABELS[stamp.benchmarkId ?? ""] ?? "벤치마크"}
                </th>
                <th className="py-1 text-right font-medium">알파</th>
              </tr>
            </thead>
            <tbody>
              {stamp.returns.map((r) => (
                <tr key={r.horizon} className="border-t border-[var(--border)]">
                  <td className="py-1 text-[var(--text-secondary)]">{r.horizon}</td>
                  <td className="py-1 text-[var(--text-muted)]">{r.asOfDate ?? "—"}</td>
                  <td className={cn("py-1 text-right tabular-nums", toneClass(r.return))}>
                    {pct(r.return)}
                  </td>
                  <td className={cn("py-1 text-right tabular-nums", toneClass(r.benchmarkReturn))}>
                    {pct(r.benchmarkReturn)}
                  </td>
                  <td className={cn("py-1 text-right tabular-nums", toneClass(r.alpha))}>
                    {pct(r.alpha)}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </SidePanel>

      {/* 당시 컨텍스트 */}
      <div className="grid gap-3 md:grid-cols-2">
        <SidePanel title="당시 뉴스">
          {stamp.context.news.length === 0 ? (
            <p className="text-[11px] text-[var(--text-muted)]">기록된 뉴스가 없습니다.</p>
          ) : (
            <ul className="space-y-1.5">
              {stamp.context.news.map((n, i) => (
                <li key={n.id ?? i} className="text-[11px]">
                  {n.url ? (
                    <a
                      href={n.url}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="inline-flex items-start gap-1 text-[var(--text-secondary)] hover:text-[var(--brand)]"
                    >
                      <span className="line-clamp-2">{n.headline}</span>
                      <ExternalLink className="mt-0.5 h-3 w-3 shrink-0" />
                    </a>
                  ) : (
                    <span className="text-[var(--text-secondary)]">{n.headline}</span>
                  )}
                  <span className="ml-1 text-[var(--text-muted)]">
                    {n.publisherName ? `${n.publisherName} · ` : ""}
                    {formatDateTimeKST(n.publishedAt)}
                  </span>
                </li>
              ))}
            </ul>
          )}
        </SidePanel>

        <SidePanel title="당시 시장">
          {stamp.context.indices.length === 0 ? (
            <p className="text-[11px] text-[var(--text-muted)]">기록된 지수가 없습니다.</p>
          ) : (
            <ul className="space-y-1">
              {stamp.context.indices.map((idx) => (
                <li key={idx.id} className="flex items-center justify-between text-[11px]">
                  <span className="text-[var(--text-muted)]">{idx.name || idx.id}</span>
                  <span className="tabular-nums text-[var(--text-secondary)]">
                    {idx.value.toLocaleString("ko-KR")}
                    <span className={cn("ml-1", toneClass(idx.changeRate))}>
                      {formatPercent(idx.changeRate)}
                    </span>
                  </span>
                </li>
              ))}
            </ul>
          )}
          {stamp.context.disclosures.length > 0 && (
            <ul className="mt-2 space-y-1 border-t border-[var(--border)] pt-2">
              {stamp.context.disclosures.map((d, i) => (
                <li key={`${d.date}-${i}`} className="text-[11px] text-[var(--text-secondary)]">
                  <span className="text-[var(--text-muted)]">{d.date} 공시 · </span>
                  {d.event}
                </li>
              ))}
            </ul>
          )}
        </SidePanel>
      </div>

      {stamp.context.aiSummary && (
        <SidePanel title="당시 AI 요약">
          <p className="whitespace-pre-wrap text-[11px] leading-relaxed text-[var(--text-secondary)]">
            {stamp.context.aiSummary}
          </p>
        </SidePanel>
      )}

      {stamp.context.dataGaps && stamp.context.dataGaps.length > 0 && (
        <p className="text-[10px] text-[var(--text-muted)]">
          이 스탬프에 기록되지 않은 항목: {stamp.context.dataGaps.map(dataGapLabel).join(", ")} — 당시
          데이터가 없어 비워 두었습니다.
        </p>
      )}

      {/* 메모·확신도는 미래에 대한 주장이 아니라서 언제든 고칠 수 있다. */}
      {readOnly ? (
        stamp.note ? (
          <SidePanel title="메모">
            <p className="text-[11px] text-[var(--text-secondary)]">{stamp.note}</p>
          </SidePanel>
        ) : null
      ) : (
      <SidePanel title="메모와 확신도">
        <div className="flex flex-wrap items-center gap-2">
          <span className="text-[10px] text-[var(--text-muted)]">확신도</span>
          {[1, 2, 3, 4, 5].map((v) => (
            <button
              key={v}
              type="button"
              disabled={saving}
              onClick={() => patch({ conviction: v })}
              className={cn(
                "size-6 rounded-md border text-[11px]",
                stamp.conviction === v
                  ? "border-[var(--brand)] bg-[var(--brand-muted)] text-[var(--text-primary)]"
                  : "border-[var(--border)] text-[var(--text-muted)] hover:text-[var(--text-secondary)]",
              )}
            >
              {v}
            </button>
          ))}
        </div>

        <div className="mt-2 flex gap-1">
          <input
            type="text"
            value={note}
            onChange={(e) => setNote(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter") patch({ note });
            }}
            placeholder="투자 이유 / 메모"
            className="h-8 min-w-0 flex-1 rounded-md border border-[var(--border)] bg-[var(--surface)] px-2 text-[11px] text-[var(--text-primary)] placeholder:text-[var(--text-muted)] focus:border-[var(--brand)] focus:outline-none"
          />
          <button
            type="button"
            disabled={saving || note === stamp.note}
            onClick={() => patch({ note })}
            className="rounded-md bg-[var(--brand)] px-3 text-[11px] font-medium text-white disabled:opacity-40"
          >
            저장
          </button>
        </div>
      </SidePanel>
      )}

      <div className="flex items-center gap-2">
        <Link
          href={analysisSymbolHref(stamp.symbol, "ai")}
          className="rounded-lg border border-[var(--border)] px-2.5 py-1 text-[11px] text-[var(--text-secondary)] hover:bg-[var(--surface-elevated)]"
        >
          종목 분석 보기
        </Link>
        {!readOnly && (
          <button
            type="button"
            onClick={onDelete}
            className="ml-auto inline-flex items-center gap-1 rounded-lg border border-[var(--border)] px-2.5 py-1 text-[11px] text-[var(--text-muted)] hover:text-rose-400"
          >
            <Trash2 className="h-3.5 w-3.5" />
            삭제
          </button>
        )}
      </div>
    </div>
  );
}

function SidePanel({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <section className="rounded-lg border border-[var(--border)] bg-[var(--surface)] p-3">
      <h4 className="mb-2 text-[11px] font-semibold text-[var(--text-primary)]">{title}</h4>
      {children}
    </section>
  );
}

function MetricRow({
  label,
  value,
  valueClass,
}: {
  label: string;
  value: string;
  valueClass?: string;
}) {
  return (
    <div className="flex items-center justify-between py-0.5 text-[11px]">
      <span className="text-[var(--text-muted)]">{label}</span>
      <span className={cn("tabular-nums text-[var(--text-secondary)]", valueClass)}>{value}</span>
    </div>
  );
}

export function StanceBadge({ stance }: { stance: StampStance }) {
  const variant = stance === "bullish" ? "profit" : stance === "bearish" ? "loss" : "default";
  return (
    <Badge variant={variant} className="px-1.5 py-0 text-[10px]">
      {STANCE_LABELS[stance]}
    </Badge>
  );
}
