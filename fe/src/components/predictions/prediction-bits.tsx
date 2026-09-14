"use client";

import { PriceDisplay } from "@/components/ui/price-display";
import {
  PREDICTION_STATUS_LABEL,
  type Prediction,
  type PredictionStatus,
} from "@/lib/prediction-types";
import { HORIZON_LABELS } from "@/lib/stamp-types";
import { cn } from "@/lib/utils";

/**
 * 예측 한 건을 그리는 조각들. 목록 행·카드·레일·모달이 전부 이 파일에서 가져간다.
 *
 * 조각을 화면마다 다시 그리면 같은 상태가 자리마다 다른 색으로 나온다 — 목표가
 * 화면의 「도달」은 초록 배지였고 스탬프 화면의 「적중」은 빨간 도장이었는데, 둘이
 * 한 목록에 서면 사용자는 두 색이 무슨 관계인지 알 수 없다.
 */

/* ────────────────────────── 상태 색 ────────────────────────── */

/**
 * 상태별 색.
 *
 * **가격 사건(도달·손절)과 채점 결과(적중·빗나감)를 다른 색 계열로 가른다.**
 * 도달은 초록, 적중은 상승색(빨강)이다. 같은 계열로 칠하면 「무기한 예측이 목표에
 * 닿은 것」과 「기한 예측이 채점에서 이긴 것」이 구별되지 않는데, 둘은 통계에
 * 들어가는 자리부터 다르다.
 */
const STATUS_TONE: Record<PredictionStatus, { text: string; chip: string; border: string }> = {
  live: {
    text: "text-[var(--text-muted)]",
    chip: "bg-[var(--surface-elevated)] text-[var(--text-muted)]",
    border: "border-dotted",
  },
  near: {
    text: "text-amber-600 dark:text-amber-400",
    chip: "bg-amber-500/15 text-amber-600 dark:text-amber-400",
    border: "border-dotted",
  },
  reached: {
    text: "text-emerald-600 dark:text-emerald-400",
    chip: "bg-emerald-500/15 text-emerald-600 dark:text-emerald-400",
    border: "border-solid",
  },
  stop_hit: {
    text: "text-[var(--loss)]",
    chip: "bg-[var(--loss)]/15 text-[var(--loss)]",
    border: "border-solid",
  },
  due: {
    text: "text-[var(--brand)]",
    chip: "bg-[var(--brand-muted)] text-[var(--brand)]",
    border: "border-solid",
  },
  hit: {
    text: "text-[var(--profit)]",
    chip: "bg-[var(--profit)]/12 text-[var(--profit)]",
    border: "border-dashed",
  },
  miss: {
    text: "text-amber-700 dark:text-amber-400",
    chip: "bg-amber-500/15 text-amber-700 dark:text-amber-400",
    border: "border-dashed",
  },
};

export function statusTone(status: PredictionStatus) {
  return STATUS_TONE[status];
}

export function toneForValue(value: number | undefined): string {
  if (value === undefined) return "text-[var(--text-muted)]";
  if (value > 0) return "text-[var(--profit)]";
  if (value < 0) return "text-[var(--loss)]";
  return "text-[var(--text-secondary)]";
}

export function PredictionStatusBadge({
  status,
  className,
}: {
  status: PredictionStatus;
  className?: string;
}) {
  return (
    <span
      className={cn(
        "shrink-0 whitespace-nowrap rounded-md px-1.5 py-0.5 text-micro font-semibold",
        STATUS_TONE[status].chip,
        className,
      )}
    >
      {PREDICTION_STATUS_LABEL[status]}
    </span>
  );
}

/* ────────────────────────── A4 · 판정 실링 ────────────────────────── */

/** 실링 안에 들어가는 「8/18」. 연도는 도장에 넣을 자리가 없다. */
function shortDate(iso?: string): string {
  if (!iso) return "";
  const d = new Date(iso.includes("T") ? iso : `${iso}T00:00:00+09:00`);
  if (Number.isNaN(d.getTime())) return "";
  const kst = new Date(d.getTime() + 9 * 3_600_000);
  return `${kst.getUTCMonth() + 1}/${kst.getUTCDate()}`;
}

/**
 * 실링 두 줄. 윗줄이 상태, 아랫줄이 「왜 그렇게 됐는지」의 가장 짧은 근거다.
 * 아랫줄을 생략하면 「진행」 도장 열 개가 똑같이 생겨 목록에서 아무 정보도 주지 않는다.
 */
function sealText(p: Prediction): { top: string; bottom: string } {
  switch (p.status) {
    case "due":
      return {
        top: "개봉",
        bottom:
          p.daysLeft === undefined || p.daysLeft === 0 ? "오늘" : `${Math.abs(p.daysLeft)}일 지남`,
      };
    case "hit":
    case "miss":
      return {
        top: p.status === "hit" ? "적중" : "빗나감",
        bottom:
          p.targetAccuracy === undefined ? "" : `오차 ${p.targetAccuracy.toFixed(1)}%`,
      };
    case "reached":
      // 실링은 지름 36~46px이다. `formatDateKST`의 「2026년 8월 18일」은 세 줄로
      // 접혀 도장 밖으로 새므로 여기서만 M/D로 줄인다.
      return { top: "도달", bottom: shortDate(p.reachedDate) };
    case "stop_hit":
      return { top: "손절", bottom: "통과" };
    case "near":
      return {
        top: "임박",
        bottom:
          p.toTargetPct === undefined ? "" : `${Math.abs(p.toTargetPct).toFixed(1)}%`,
      };
    default:
      // 무기한 예측은 셀 날이 없다 — D-day 대신 목표까지 남은 거리를 적는다.
      if (p.daysLeft === undefined) {
        return {
          top: "추적",
          bottom: p.toTargetPct === undefined ? "무기한" : `${Math.abs(p.toTargetPct).toFixed(0)}%`,
        };
      }
      return { top: "진행", bottom: p.daysLeft <= 0 ? "곧 개봉" : `D-${p.daysLeft}` };
  }
}

export function PredictionSeal({
  prediction,
  size = "md",
  className,
}: {
  prediction: Prediction;
  size?: "sm" | "md";
  className?: string;
}) {
  const tone = STATUS_TONE[prediction.status];
  const { top, bottom } = sealText(prediction);

  return (
    <span
      aria-label={`${top} ${bottom}`.trim()}
      className={cn(
        "grid shrink-0 place-items-center rounded-full border-[1.5px] text-center leading-[1.05]",
        // 살짝 기울여야 붙인 도장처럼 읽힌다. 각도를 더 주면 글자가 읽기 어려워진다.
        "-rotate-[9deg] opacity-[0.9]",
        size === "sm" ? "size-9" : "size-[46px]",
        "text-micro",
        tone.text,
        tone.border,
        className,
      )}
    >
      <span className="font-extrabold tracking-tight">
        {top}
        {bottom && (
          <span className="block text-micro font-semibold tracking-normal opacity-80">
            {bottom}
          </span>
        )}
      </span>
    </span>
  );
}

/* ────────────────────────── A3 · 진행 게이지 ────────────────────────── */

/**
 * 등록가 → 목표 진척률.
 *
 * **0선(등록가)이 게이지 안에 있어야 한다.** 스탬프의 예전 게이지는 음수 진행률을
 * 왼쪽 끝에서부터 그려서, 목표와 반대로 간 예측이 「조금 진행했다」처럼 보였다.
 * 여기서는 반대로 간 만큼을 0선 **왼쪽**에 칠한다.
 */
export function PredictionGauge({
  prediction,
  className,
}: {
  prediction: Prediction;
  className?: string;
}) {
  const progress = prediction.progress;
  if (progress === undefined) {
    return (
      <span
        className={cn("block h-1.5 rounded-full bg-[var(--surface-elevated)]", className)}
        aria-label="진행률 없음"
      />
    );
  }

  // 반대로 간 예측이 있을 때만 0선을 안쪽으로 들인다. 항상 들이면 정상 진행하는
  // 게이지가 전부 오른쪽 3/4만 쓰게 되어 눈금이 좁아진다.
  const backRoom = progress < 0 ? 28 : 0;
  const zero = backRoom;
  const forward = Math.max(0, Math.min(100, progress));
  const backward = Math.min(backRoom, (Math.abs(Math.min(0, progress)) / 100) * 100);

  const reached = prediction.status === "reached";
  const bad = prediction.status === "stop_hit" || progress < 0;

  return (
    <span
      className={cn("relative block h-1.5 rounded-full bg-[var(--surface-elevated)]", className)}
      role="img"
      aria-label={`목표 진척률 ${Math.round(progress)}%`}
    >
      {backRoom > 0 && (
        <span
          aria-hidden
          className="absolute -top-0.5 bottom-[-2px] w-px bg-[var(--text-muted)] opacity-50"
          style={{ left: `${zero}%` }}
        />
      )}
      <span
        className={cn(
          "absolute inset-y-0 rounded-full",
          reached
            ? "bg-emerald-500"
            : bad
              ? "bg-[var(--loss)]"
              : "bg-[var(--brand)]",
        )}
        style={
          progress >= 0
            ? { left: `${zero}%`, width: `${((100 - zero) * forward) / 100}%` }
            : { left: `${Math.max(0, zero - backward)}%`, width: `${Math.min(backward, zero)}%` }
        }
      />
    </span>
  );
}

/** 게이지 옆에 붙는 한마디. 숫자만 두면 「92」가 무엇의 92인지 알 수 없다. */
export function progressLabel(p: Prediction): { text: string; tone: string } {
  if (p.status === "reached") {
    return { text: "목표 도달", tone: "text-emerald-600 dark:text-emerald-400" };
  }
  if (p.status === "stop_hit") return { text: "손절 통과", tone: "text-[var(--loss)]" };
  if (p.progress === undefined) return { text: "—", tone: "text-[var(--text-muted)]" };
  if (p.progress < 0) {
    return { text: `반대로 ${Math.abs(Math.round(p.progress))}%`, tone: "text-[var(--loss)]" };
  }
  return { text: `목표의 ${Math.round(p.progress)}%`, tone: "text-[var(--text-secondary)]" };
}

/* ────────────────────────── A2 · 가격 축 ────────────────────────── */

type Mark = { at: number; label: string; price: number; tone: string };

/**
 * 라벨이 서로 겹치면 아래로 한 줄 내린다.
 *
 * 축 폭이 300px이고 라벨이 40px쯤이라, 두 눈금이 **15% 안쪽**에 붙으면 글자가
 * 포개진다(실측: 등록 214,000 / 목표 225,000이 5px 겹쳤다). 같은 줄의 직전
 * 라벨과만 비교하면 세 개가 몰릴 때 1번과 3번이 다시 겹치므로 줄별 마지막
 * 위치를 각각 기억한다.
 */
const LABEL_GAP_PCT = 15;

function laneMarks(marks: Mark[]): { mark: Mark; lane: number }[] {
  return [...marks]
    .sort((a, b) => a.at - b.at)
    .reduce<{ mark: Mark; lane: number; lastAt: [number, number] }[]>((acc, mark) => {
      const prev = acc[acc.length - 1];
      const lastAt = prev ? prev.lastAt : ([-Infinity, -Infinity] as [number, number]);
      const lane = mark.at - lastAt[0] >= LABEL_GAP_PCT ? 0 : 1;
      const next: [number, number] = [lastAt[0], lastAt[1]];
      next[lane] = mark.at;
      acc.push({ mark, lane, lastAt: next });
      return acc;
    }, [])
    .map(({ mark, lane }) => ({ mark, lane }));
}

/**
 * 손절 · 등록가 · 현재가 · 1차 · 2차를 한 축에 올린 좌표.
 *
 * 축의 양 끝은 **실제 가격 범위**로 잡는다(최소~최대에 여백 8%). 등록가를 0%,
 * 목표를 100%로 고정하면 목표를 넘어선 예측이 축 밖으로 나가고 손절을 찍을 자리도
 * 없어진다.
 *
 * 라벨은 좌우 끝에서 잘리므로 위치를 6~94%로 가둔다 — 핀은 실제 좌표에 남고
 * 글자만 안으로 들어온다.
 */
export function PredictionAxis({
  prediction,
  className,
}: {
  prediction: Prediction;
  className?: string;
}) {
  const { basePrice, currentPrice, targetPrice, targetPrice2, stopPrice, currency } = prediction;

  const values = [basePrice, currentPrice, targetPrice, targetPrice2, stopPrice].filter(
    (v): v is number => typeof v === "number" && v > 0,
  );
  if (values.length < 2) return null;

  const min = Math.min(...values);
  const max = Math.max(...values);
  const pad = (max - min) * 0.08 || Math.abs(max) * 0.02 || 1;
  const lo = min - pad;
  const hi = max + pad;
  const at = (v: number) => ((v - lo) / (hi - lo)) * 100;

  const marks: Mark[] = [];
  if (stopPrice) {
    marks.push({ at: at(stopPrice), label: "손절", price: stopPrice, tone: "var(--loss)" });
  }
  if (basePrice) {
    marks.push({
      at: at(basePrice),
      label: "등록",
      price: basePrice,
      tone: "var(--text-muted)",
    });
  }
  marks.push({
    at: at(targetPrice),
    label: targetPrice2 ? "1차" : "목표",
    price: targetPrice,
    tone: prediction.upside ? "var(--profit)" : "var(--loss)",
  });
  if (targetPrice2) {
    marks.push({
      at: at(targetPrice2),
      label: "2차",
      price: targetPrice2,
      tone: prediction.upside ? "var(--profit)" : "var(--loss)",
    });
  }

  const laned = laneMarks(marks);
  const twoRows = laned.some((m) => m.lane === 1);

  const nowAt = currentPrice !== undefined ? at(currentPrice) : null;
  const baseAt = basePrice !== undefined ? at(basePrice) : null;

  // 채움은 등록가에서 현재가까지 — "여기서 시작해 여기까지 왔다".
  const fillFrom = nowAt !== null && baseAt !== null ? Math.min(nowAt, baseAt) : null;
  const fillTo = nowAt !== null && baseAt !== null ? Math.max(nowAt, baseAt) : null;

  const fillTone =
    prediction.status === "reached"
      ? "bg-emerald-500/60"
      : prediction.status === "stop_hit" || (prediction.progress ?? 0) < 0
        ? "bg-[var(--loss)]/50"
        : "bg-[var(--brand)]/55";

  const clamp = (v: number) => Math.max(6, Math.min(94, v));

  return (
    <div className={cn("min-w-0", className)}>
      <div className="relative mt-4 h-1.5 rounded-full bg-[var(--surface-elevated)]">
        {fillFrom !== null && fillTo !== null && (
          <span
            aria-hidden
            className={cn("absolute inset-y-0 rounded-full", fillTone)}
            style={{ left: `${fillFrom}%`, width: `${Math.max(fillTo - fillFrom, 0.6)}%` }}
          />
        )}

        {laned.map(({ mark: m, lane }) => (
          <span key={m.label} aria-hidden>
            <span
              className="absolute top-1/2 w-[2px] -translate-x-1/2 rounded-sm"
              style={{
                left: `${m.at}%`,
                background: m.tone,
                // 두 번째 줄로 내려간 라벨은 눈금도 길게 늘여 어느 눈금의 글자인지 잇는다.
                top: "50%",
                height: lane === 1 ? 34 : 16,
                transform: "translate(-50%, -50%)",
                marginTop: lane === 1 ? 9 : 0,
              }}
            />
            <span
              className="absolute -translate-x-1/2 whitespace-nowrap text-center text-[9.5px] leading-[1.15] tabular-nums"
              style={{ left: `${clamp(m.at)}%`, color: m.tone, top: lane === 1 ? 33 : 13 }}
            >
              <span className="block text-[8.5px] opacity-85">{m.label}</span>
              <PriceDisplay value={m.price} currency={currency} size="inherit" />
            </span>
          </span>
        ))}

        {nowAt !== null && currentPrice !== undefined && (
          <>
            <span
              aria-hidden
              className="absolute top-1/2 size-3 -translate-x-1/2 -translate-y-1/2 rounded-full border-[2.5px] border-[var(--surface)] bg-[var(--text-primary)]"
              style={{ left: `${nowAt}%` }}
            />
            <span
              className="absolute bottom-[11px] -translate-x-1/2 whitespace-nowrap text-center text-[9.5px] font-bold leading-[1.15] tabular-nums text-[var(--text-primary)]"
              style={{ left: `${clamp(nowAt)}%` }}
            >
              <span className="block text-[8.5px] font-normal text-[var(--text-muted)]">현재</span>
              <PriceDisplay value={currentPrice} currency={currency} size="inherit" />
            </span>
          </>
        )}
      </div>
      {/*
        라벨이 축 아래로 흐르므로 그만큼 자리를 비워 둔다. 겹쳐서 둘째 줄까지 쓴
        행만 더 높다 — 항상 높게 잡으면 눈금이 둘뿐인 행들이 이유 없이 벌어진다.
      */}
      <div className={twoRows ? "h-11" : "h-6"} aria-hidden />
    </div>
  );
}

/* ────────────────────────── 기한 표시 ────────────────────────── */

export function horizonLabel(p: Prediction): string {
  if (p.horizonKind === "none") return "무기한";
  return HORIZON_LABELS[p.horizonKind] ?? p.horizonKind;
}

/** 만기까지 남은 날. 무기한이면 그렇다고 적는다 — 빈칸으로 두면 데이터 누락처럼 보인다. */
export function PredictionDday({ prediction }: { prediction: Prediction }) {
  if (prediction.horizonKind === "none") {
    return (
      <span className="shrink-0 rounded-md bg-[var(--surface-elevated)] px-1.5 py-0.5 text-micro font-semibold text-[var(--text-muted)]">
        무기한
      </span>
    );
  }
  const d = prediction.daysLeft ?? 0;
  const overdue = d < 0;
  return (
    <span
      className={cn(
        "shrink-0 rounded-md px-1.5 py-0.5 text-micro font-semibold tabular-nums",
        overdue
          ? "bg-[var(--brand-muted)] text-[var(--brand)]"
          : d <= 3
            ? "bg-amber-500/15 text-amber-600 dark:text-amber-400"
            : "bg-[var(--surface-elevated)] text-[var(--text-muted)]",
      )}
    >
      {overdue ? `D+${Math.abs(d)}` : d === 0 ? "D-DAY" : `D-${d}`}
    </span>
  );
}
