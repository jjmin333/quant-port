"use client";

import { useState } from "react";
import { Loader2, ScrollText } from "lucide-react";

import { MiniToggle } from "@/components/ui/mini-toggle";
import {
  changeUnavailableLabel,
  formatDelta,
  formatMetricValue,
  missingReasonLabel,
  peerGroupLabel,
  topPercentLabel,
  type ScoreAxisExplain,
  type ScoreChange,
  type ScoreChangeDriver,
  type ScoreExplainData,
  type ScoreMetricExplain,
  explainSummarySentence,
} from "@/lib/score-explain-api";
import { SCORE_TONE_BG, SCORE_TONE_TEXT, scoreTone } from "@/lib/stock-score-api";
import { cn } from "@/lib/utils";

/**
 * 점수 근거 패널 — AI 분석 탭 (2026-09-17 사용자 결정 B).
 *
 * # 이 패널이 채우는 자리
 *
 * AI 분석 탭의 본문(LLM 글)은 17,916종목 중 744종목(4%)에만 있다. 점수는 거의 모든
 * 종목에 있으므로, 나머지 96%에게 이 탭은 사실상 빈 화면이었다.
 *
 * # 간단 / 상세 (사용자 결정)
 *
 * 「AI 핵심 요약」 카드와 **같은 손잡이**다(`MiniToggle`, 기본 「간단」, 상태 저장 안 함).
 * 이 앱에서 「한눈에 볼까, 다 볼까」는 언제나 이 모양이고, 데이터가 많아져 가독성이
 * 상하는 자리는 이 둘로 가른다.
 *
 *   · **간단** — 무엇이 점수를 만들었나(구성 막대) + 어제 무엇이 바꿨나(상위 3) + 기준일
 *   · **상세** — 위 전부 + 전주 창 + 축별 지표 표 + 제외된 값 + 출처
 *
 * # 모바일이 기준이다
 *
 * 표를 4열로 두지 않는다. 375px에서 「지표 · 값 · 순위 · 비교집단」은 어느 칸도 못
 * 읽는다. **비교 집단은 축 머리에 한 번** 적고, 그 축에서 혼자 다른 집단에서 잰
 * 지표만 자기 줄에 따로 밝힌다 — 실측상 한 축의 지표는 거의 같은 집단에서 매겨진다.
 *
 * # 산문을 싣지 않는다 (2026-09-17 사용자 결정)
 *
 * 「남은 네 축의 비중이 100%로 다시 나뉘었습니다…」 같은 설명문은 모든 종목·모든
 * 방문에 똑같이 붙으면서 높이만 먹는다. 같은 사실을 **숫자 두 개를 나란히** 적어
 * 말한다: `33.3% (설정 30)`.
 */

type DetailLevel = "brief" | "full";

const DETAIL_OPTIONS: { id: DetailLevel; label: string }[] = [
  { id: "brief", label: "간단" },
  { id: "full", label: "상세" },
];

/** 간단 보기에서 보여 줄 원인 수. 넷째부터는 대개 0.1점 미만이다. */
const BRIEF_DRIVER_LIMIT = 3;

export function ScoreExplainPanel({
  data,
  loading,
  error,
  compact = false,
  className,
}: {
  data: ScoreExplainData | null;
  loading: boolean;
  error?: string | null;
  compact?: boolean;
  className?: string;
}) {
  const [detail, setDetail] = useState<DetailLevel>("brief");
  const pad = compact ? "px-2.5" : "px-3";

  return (
    <div
      className={cn(
        "overflow-hidden rounded-lg border border-[var(--border)] bg-[var(--card)]",
        className,
      )}
    >
      <div className={cn("flex flex-wrap items-center gap-x-2 gap-y-1 pt-2.5 pb-1.5", pad)}>
        <ScrollText className="h-3.5 w-3.5 shrink-0 text-[var(--brand)]" aria-hidden />
        <span className="text-cell font-semibold text-[var(--text-primary)]">점수 근거</span>
        {/* 토글은 언제나 오른쪽 끝이다 — 종목을 옮길 때 누르는 자리가 움직이면 안 된다. */}
        <MiniToggle
          className="ml-auto"
          options={DETAIL_OPTIONS}
          value={detail}
          onChange={setDetail}
        />
      </div>

      {loading ? (
        <p
          className={cn("flex items-center gap-1.5 pb-2.5 text-cell text-[var(--text-muted)]", pad)}
        >
          <Loader2 className="h-3 w-3 animate-spin" />
          점수 근거를 불러오는 중입니다…
        </p>
      ) : error ? (
        /* 「불러오지 못했습니다」와 「아직 없습니다」는 다른 상태다 (요구 8). */
        <p className={cn("pb-2.5 text-cell text-[var(--text-muted)]", pad)}>{error}</p>
      ) : !data || data.score == null ? (
        <p className={cn("pb-2.5 text-cell text-[var(--text-muted)]", pad)}>
          {data?.composite.withheld
            ? "사용 가능한 데이터가 부족해 이번 회차의 점수는 보류되었습니다."
            : "이 종목은 아직 점수가 산출되지 않았습니다."}
        </p>
      ) : (
        <div className={cn("space-y-3 pb-3", pad)}>
          {/* 한 문장 요약 — 응답의 숫자를 고르고 이어 붙이기만 한다(LLM 0, 2026-09-24). */}
          {explainSummarySentence(data) && (
            <p className="text-cell leading-relaxed text-[var(--text-primary)]">
              {explainSummarySentence(data)}
            </p>
          )}
          <CompositionSection data={data} detail={detail} />
          <ChangeSections data={data} detail={detail} />
          {detail === "full" && <EvidenceSection data={data} />}
          {detail === "full" && <RejectedSection data={data} />}
          <ProvenanceSection data={data} detail={detail} />
        </div>
      )}
    </div>
  );
}

/* -------------------------------------------------------------------------- */
/* ① 구성                                                                      */
/* -------------------------------------------------------------------------- */

function CompositionSection({ data, detail }: { data: ScoreExplainData; detail: DetailLevel }) {
  const present = data.axes.filter((a) => a.available && typeof a.contribution === "number");
  const missing = data.axes.filter((a) => !a.available);
  const total = present.reduce((sum, a) => sum + (a.contribution ?? 0), 0);

  return (
    <section>
      <SectionTitle>구성</SectionTitle>

      <div className="flex h-5 overflow-hidden rounded border border-[var(--border)]">
        {present.map((axis) => {
          const share = total > 0 ? ((axis.contribution ?? 0) / total) * 100 : 0;
          return (
            <div
              key={axis.axis}
              className={cn(
                "flex items-center justify-center",
                SCORE_TONE_BG[scoreTone(axis.score ?? 0)],
              )}
              style={{ width: `${share}%` }}
              title={`${axis.label} ${axis.contribution?.toFixed(2)}점`}
            >
              {/* 좁은 조각에 글자를 넣으면 잘린 음절만 남는다 — 범례가 대신 말한다. */}
              {share >= 16 && (
                <span className="truncate px-1 text-micro font-semibold text-white">
                  {axis.label}
                </span>
              )}
            </div>
          );
        })}
      </div>

      <ul className="mt-1.5 flex flex-wrap gap-x-3 gap-y-1">
        {present.map((axis) => (
          <li key={axis.axis} className="flex items-center gap-1 text-micro">
            <span
              className={cn(
                "h-2 w-2 shrink-0 rounded-sm",
                SCORE_TONE_BG[scoreTone(axis.score ?? 0)],
              )}
              aria-hidden
            />
            <span className="text-[var(--text-secondary)]">{axis.label}</span>
            <span className="font-semibold tabular-nums text-[var(--text-primary)]">
              {axis.contribution?.toFixed(2)}
            </span>
            {/*
              실제로 작동한 비중과 설정값을 **나란히** 적는다. 둘이 다른 이유(빈 축이
              있어 분모가 줄었다)를 문장으로 쓰는 대신 숫자 둘로 말한다.
            */}
            <span className="tabular-nums text-[var(--text-muted)]">
              {axis.weightShare.toFixed(1)}%
              {Math.round(axis.weightShare) !== axis.weight && ` (설정 ${axis.weight})`}
            </span>
          </li>
        ))}
      </ul>

      {missing.length > 0 && (
        <ul className="mt-1.5 flex flex-wrap gap-1">
          {missing.map((axis) => (
            <li
              key={axis.axis}
              className="rounded border border-dashed border-[var(--border)] px-1.5 py-0.5 text-micro text-[var(--text-muted)]"
            >
              {axis.label} · {missingReasonLabel(axis.missingReason)}
            </li>
          ))}
        </ul>
      )}

      {detail === "full" && (
        <p className="mt-1.5 text-micro tabular-nums text-[var(--text-muted)]">
          가중 평균 {data.composite.raw.toFixed(2)} · 적용 가중치 {data.composite.usedWeight}/
          {data.composite.totalWeight}
          {data.composite.shrinkFactor < 1 && ` · 중립 보정 후 ${data.composite.final.toFixed(2)}`}
          {` → ${data.score}점`}
        </p>
      )}
    </section>
  );
}

/* -------------------------------------------------------------------------- */
/* ② 변화                                                                      */
/* -------------------------------------------------------------------------- */

function ChangeSections({ data, detail }: { data: ScoreExplainData; detail: DetailLevel }) {
  // 간단에서는 전일 하나. 「어제와 다른가」가 이 카드에서 가장 자주 묻는 질문이다.
  const windows = detail === "brief" ? data.changes.slice(0, 1) : data.changes;
  return (
    <>
      {windows.map((change) => (
        <ChangeSection key={change.window} change={change} detail={detail} />
      ))}
    </>
  );
}

function ChangeSection({ change, detail }: { change: ScoreChange; detail: DetailLevel }) {
  if (!change.available) {
    return (
      <section>
        <SectionTitle>{change.label}</SectionTitle>
        <p className="text-micro text-[var(--text-muted)]">
          {changeUnavailableLabel(change.reason)}
        </p>
      </section>
    );
  }

  const drivers = detail === "brief" ? change.drivers.slice(0, BRIEF_DRIVER_LIMIT) : change.drivers;
  const max = Math.max(1, ...change.drivers.map((d) => Math.abs(d.effect)));
  const delta = change.delta ?? 0;

  return (
    <section>
      <div className="flex flex-wrap items-baseline gap-x-2 gap-y-0.5">
        <SectionTitle className="mb-0">{change.label}</SectionTitle>
        <span className="text-micro tabular-nums text-[var(--text-muted)]">
          {change.fromDate} → {change.toDate}
        </span>
        <span
          className={cn(
            "ml-auto text-cell font-bold tabular-nums",
            delta > 0
              ? "text-[var(--profit)]"
              : delta < 0
                ? "text-[var(--loss)]"
                : "text-[var(--text-muted)]",
          )}
        >
          {change.fromScore} → {change.toScore} ({formatDelta(delta)})
        </span>
      </div>

      {drivers.length === 0 ? (
        <p className="mt-1 text-micro text-[var(--text-muted)]">달라진 것이 없습니다.</p>
      ) : (
        <ul className="mt-1 space-y-0.5">
          {drivers.map((driver) => (
            <DriverRow
              key={`${driver.kind}-${driver.axis ?? ""}`}
              driver={driver}
              max={max}
              detail={detail}
            />
          ))}
          {detail === "full" && Math.abs(change.roundingResidual) >= 0.01 && (
            /*
              반올림 잔차를 적는다. 드라이버 합은 실제 변화(+1.18)에 맞고 머리줄은
              정수(+2)라, 이 줄이 없으면 둘이 어긋나 보인다. 유일한 미설명분이다.
            */
            <li className="flex items-center gap-2 text-micro text-[var(--text-muted)]">
              <span className="min-w-0 flex-1 truncate">표시 반올림</span>
              <DriverBar effect={change.roundingResidual} max={max} neutral />
              <span className="w-12 shrink-0 text-right tabular-nums">
                {formatDelta(change.roundingResidual, 2)}
              </span>
            </li>
          )}
        </ul>
      )}

      {detail === "brief" && change.drivers.length > BRIEF_DRIVER_LIMIT && (
        <p className="mt-1 text-micro text-[var(--text-muted)]">
          외 {change.drivers.length - BRIEF_DRIVER_LIMIT}건은 상세에서 볼 수 있습니다.
        </p>
      )}
    </section>
  );
}

function DriverRow({
  driver,
  max,
  detail,
}: {
  driver: ScoreChangeDriver;
  max: number;
  detail: DetailLevel;
}) {
  const metrics = detail === "full" ? (driver.metrics ?? []).filter(hasMovement) : [];
  return (
    <li>
      <div className="flex items-center gap-2 text-micro">
        {/* 문장이 먼저 잘린다 — 오른쪽의 점수 효과는 이 줄의 요점이라 고정 폭이다. */}
        <span
          className="min-w-0 flex-1 truncate text-[var(--text-secondary)]"
          title={driver.detail}
        >
          {driver.detail}
        </span>
        <DriverBar effect={driver.effect} max={max} />
        <span
          className={cn(
            "w-12 shrink-0 text-right font-semibold tabular-nums",
            driver.effect > 0 ? "text-[var(--profit)]" : "text-[var(--loss)]",
          )}
        >
          {formatDelta(driver.effect, 2)}
        </span>
      </div>

      {metrics.length > 0 && (
        <ul className="mt-0.5 mb-1 ml-2 space-y-0.5 border-l border-[var(--border)] pl-2">
          {metrics.map((metric) => (
            <li key={metric.key} className="flex items-center gap-2 text-micro">
              <span className="min-w-0 flex-1 truncate text-[var(--text-muted)]">
                {metric.label}
              </span>
              <span className="shrink-0 tabular-nums text-[var(--text-muted)]">
                {formatMetricValue(metric.fromRaw, metric.unit)} →{" "}
                <span className="text-[var(--text-secondary)]">
                  {formatMetricValue(metric.toRaw, metric.unit)}
                </span>
              </span>
              {typeof metric.percentileDelta === "number" && metric.percentileDelta !== 0 && (
                <span
                  className={cn(
                    "w-12 shrink-0 text-right tabular-nums",
                    metric.percentileDelta > 0 ? "text-[var(--profit)]" : "text-[var(--loss)]",
                  )}
                  title="동종 순위 백분위 변화"
                >
                  {formatDelta(metric.percentileDelta, 1)}
                </span>
              )}
            </li>
          ))}
        </ul>
      )}
    </li>
  );
}

/** 0을 가운데 두고 좌우로 자라는 작은 막대. 폭이 좁아도 방향은 읽힌다. */
function DriverBar({ effect, max, neutral }: { effect: number; max: number; neutral?: boolean }) {
  const ratio = Math.min(1, Math.abs(effect) / max);
  const width = `${Math.max(2, ratio * 50)}%`;
  return (
    <span className="relative block h-2 w-14 shrink-0" aria-hidden>
      <span className="absolute inset-y-0 left-1/2 w-px bg-[var(--border)]" />
      <span
        className={cn(
          "absolute top-0.5 h-1 rounded-sm",
          neutral
            ? "bg-[var(--text-muted)] opacity-60"
            : effect > 0
              ? "bg-[var(--profit)]"
              : "bg-[var(--loss)]",
        )}
        style={effect > 0 ? { left: "50%", width } : { right: "50%", width }}
      />
    </span>
  );
}

function hasMovement(metric: { fromRaw?: number; toRaw?: number }): boolean {
  if (typeof metric.fromRaw !== "number" || typeof metric.toRaw !== "number") return true;
  return metric.fromRaw !== metric.toRaw;
}

/* -------------------------------------------------------------------------- */
/* ③ 축별 근거 (상세)                                                          */
/* -------------------------------------------------------------------------- */

function EvidenceSection({ data }: { data: ScoreExplainData }) {
  const axes = data.axes.filter((a) => (a.metrics ?? []).length > 0);
  /*
    점수는 있는데 지표 표가 없는 축 — 심리·수급이 그렇다. 이 축은 뉴스 감성과 내부자
    거래에서 나오고 그 재료는 요약 파이프라인이 종목별로 모으므로, 점수 배치가 남기는
    지표·백분위 표에 자리가 없다(service/stock_score_sentiment.go).

    **말없이 빼지 않는다.** 위 구성 막대에 분명히 서 있던 축이 근거 목록에서만
    사라지면 읽는 사람은 자기가 무엇을 놓쳤는지 묻게 된다.
  */
  const noEvidence = data.axes.filter((a) => a.available && (a.metrics ?? []).length === 0);
  if (axes.length === 0 && noEvidence.length === 0) return null;
  return (
    <section>
      <SectionTitle>축별 근거</SectionTitle>
      <div className="space-y-2">
        {axes.map((axis) => (
          <AxisEvidence key={axis.axis} axis={axis} />
        ))}
        {noEvidence.map((axis) => (
          <p key={axis.axis} className="text-micro text-[var(--text-muted)]">
            <span className="font-semibold text-[var(--text-secondary)]">
              {axis.label} {axis.score?.toFixed(1)}
            </span>{" "}
            — 뉴스 감성과 내부자 거래에서 산출되어 지표별 순위가 없습니다.
          </p>
        ))}
      </div>
    </section>
  );
}

function AxisEvidence({ axis }: { axis: ScoreAxisExplain }) {
  const metrics = axis.metrics ?? [];
  const group = dominantGroup(metrics);
  const tone = SCORE_TONE_TEXT[scoreTone(axis.score ?? 0)];

  return (
    <div>
      <div className="flex flex-wrap items-baseline gap-x-2 gap-y-0.5">
        <span className={cn("text-cell font-semibold tabular-nums", tone)}>
          {axis.label} {axis.score?.toFixed(1)}
        </span>
        {/* 비교 집단은 축 머리에 한 번 — 놀라운 순위는 거의 항상 이 값이 설명한다. */}
        {group && (
          <span className="text-micro text-[var(--text-muted)]">{peerGroupLabel(group)} 기준</span>
        )}
      </div>
      <ul className="mt-0.5">
        {metrics.map((metric) => (
          <MetricRow key={metric.key} metric={metric} axisGroup={group} />
        ))}
      </ul>
    </div>
  );
}

function MetricRow({
  metric,
  axisGroup,
}: {
  metric: ScoreMetricExplain;
  axisGroup: string | null;
}) {
  const rank = topPercentLabel(metric.percentile);
  // 이 축의 대세와 다른 집단에서 잰 지표만 자기 줄에 밝힌다.
  const ownGroup = metric.rankedIn && metric.rankedIn !== axisGroup ? metric.rankedIn : null;

  return (
    <li className="flex items-center gap-2 border-b border-[var(--border)] py-0.5 text-micro last:border-b-0">
      <span className="min-w-0 flex-1 truncate text-[var(--text-secondary)]">
        {metric.label}
        {/*
          원값의 방향을 밝힌다. 부채비율 29.9%가 「상위 13%」인 것은 낮을수록 좋기
          때문인데, 그 말이 없으면 표가 스스로 모순돼 보인다.
        */}
        {!metric.higherIsBetter && (
          <span className="ml-1 text-[var(--text-muted)]">(낮을수록 좋음)</span>
        )}
        {ownGroup && (
          <span className="ml-1 text-[var(--text-muted)]">· {peerGroupLabel(ownGroup)}</span>
        )}
      </span>
      <span className="shrink-0 tabular-nums text-[var(--text-primary)]">
        {formatMetricValue(metric.raw, metric.unit)}
      </span>
      <span className="w-14 shrink-0 text-right tabular-nums text-[var(--text-muted)]">
        {rank ?? "—"}
      </span>
    </li>
  );
}

/** 한 축의 지표들이 대체로 어느 집단에서 매겨졌는가. */
function dominantGroup(metrics: ScoreMetricExplain[]): string | null {
  const counts = new Map<string, number>();
  for (const metric of metrics) {
    if (!metric.rankedIn) continue;
    counts.set(metric.rankedIn, (counts.get(metric.rankedIn) ?? 0) + 1);
  }
  let best: string | null = null;
  let bestCount = 0;
  for (const [group, count] of counts) {
    if (count > bestCount) {
      best = group;
      bestCount = count;
    }
  }
  return best;
}

/* -------------------------------------------------------------------------- */
/* ④ 제외된 값 (상세)                                                          */
/* -------------------------------------------------------------------------- */

function RejectedSection({ data }: { data: ScoreExplainData }) {
  const rejected = data.rejected ?? [];
  if (rejected.length === 0) return null;
  return (
    <section>
      <SectionTitle>평가에서 제외한 값</SectionTitle>
      <ul className="space-y-0.5">
        {rejected.map((item) => (
          <li key={item.key} className="flex items-center gap-2 text-micro">
            <span className="min-w-0 flex-1 truncate text-[var(--text-secondary)]">
              {item.label}
            </span>
            <span className="shrink-0 tabular-nums text-[var(--score-low)]">
              {formatMetricValue(item.value, item.unit)}
            </span>
            <span className="shrink-0 tabular-nums text-[var(--text-muted)]">
              신뢰 구간 {item.domainLo.toLocaleString("ko-KR")}~
              {item.domainHi.toLocaleString("ko-KR")}
            </span>
          </li>
        ))}
      </ul>
    </section>
  );
}

/* -------------------------------------------------------------------------- */
/* ⑤ 출처                                                                      */
/* -------------------------------------------------------------------------- */

function ProvenanceSection({ data, detail }: { data: ScoreExplainData; detail: DetailLevel }) {
  const { provenance, composite } = data;
  return (
    <section className="border-t border-[var(--border)] pt-2">
      <p className="flex flex-wrap items-center gap-x-2 gap-y-0.5 text-micro text-[var(--text-muted)]">
        <span className="tabular-nums">{provenance.asOf} 종가 기준</span>
        <span
          className={cn(
            "rounded px-1.5 py-0.5 font-semibold",
            provenance.basis === "actual"
              ? "bg-[var(--score-high)]/12 text-[var(--score-high)]"
              : "bg-[var(--score-mid)]/12 text-[var(--score-mid)]",
          )}
        >
          {provenance.basis === "actual"
            ? "실제값"
            : `일부 축 결측 ${composite.usedWeight}/${composite.totalWeight}`}
        </span>
        {data.peerGroup && <span>{peerGroupLabel(data.peerGroup)} 기준 순위</span>}
      </p>
      {provenance.note && (
        <p className="mt-1 text-micro text-[var(--text-muted)]">{provenance.note}</p>
      )}
      {detail === "full" && (
        <p className="mt-1 text-micro text-[var(--text-muted)]">
          출처 {provenance.sources.join(" · ")}
        </p>
      )}
    </section>
  );
}

/* -------------------------------------------------------------------------- */

function SectionTitle({ children, className }: { children: React.ReactNode; className?: string }) {
  return (
    <h4 className={cn("mb-1 text-cell font-semibold text-[var(--text-primary)]", className)}>
      {children}
    </h4>
  );
}
