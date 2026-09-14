"use client";

import { useEffect, useRef, useState } from "react";

import {
  HORIZON_OPTIONS,
  STANCE_LABELS,
  stanceForTarget,
  type StampHorizonKind,
} from "@/lib/stamp-types";
import { cn } from "@/lib/utils";
import { PriceDisplay } from "@/components/ui/price-display";
import { LoginRequiredNote } from "@/components/auth/login-required-note";
import { useModalAutoFocus } from "@/hooks/use-modal-autofocus";
import { useLoginGate } from "@/hooks/use-login-gate";

/**
 * 예상 주가 · 검증 기간 · 확신도 · 메모를 받는 폼 본체.
 *
 * 종목 화면의 도장 버튼(팝오버)과 스탬프 화면의 「스탬프 찍기」(모달) 둘 다 이걸 쓴다.
 * 같은 입력을 두 벌로 두면 한쪽에만 규칙이 붙는다 — 예컨대 목표가 상한 안내나
 * 파생 관점 표시가 한쪽에서만 보이는 식으로 갈라진다.
 */

/** 오늘+n일에서 주말만 건너뛴 날짜. 휴장일은 서버 캘린더만 아는 값이라 안내 문구로 덮는다. */
export function estimateDueDate(kind: StampHorizonKind): Date {
  const d = new Date();
  switch (kind) {
    case "session_close":
      break;
    case "next_session_close":
      d.setDate(d.getDate() + 1);
      break;
    case "1w":
      d.setDate(d.getDate() + 7);
      break;
    case "3m":
      d.setMonth(d.getMonth() + 3);
      break;
    default:
      d.setMonth(d.getMonth() + 1);
  }
  while (d.getDay() === 0 || d.getDay() === 6) d.setDate(d.getDate() + 1);
  return d;
}

const WEEKDAYS = ["일", "월", "화", "수", "목", "금", "토"];

function formatEstimate(d: Date): string {
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(
    d.getDate(),
  ).padStart(2, "0")} (${WEEKDAYS[d.getDay()]})`;
}

export function toDateInputValue(d: Date): string {
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(
    d.getDate(),
  ).padStart(2, "0")}`;
}

/** 입력 문자열에서 숫자만 남긴다 — 사용자가 콤마를 찍든 말든 받는다. */
export function parseTarget(raw: string): number {
  const cleaned = raw.replace(/[^0-9.]/g, "");
  const value = Number.parseFloat(cleaned);
  return Number.isFinite(value) ? value : 0;
}

export type StampPredictionValue = {
  targetPrice: number;
  horizonKind: StampHorizonKind;
  customDueDate?: string;
  conviction: number;
  note: string;
};

export function StampPredictionForm({
  basePrice,
  currency = "KRW",
  week52High,
  week52Low,
  busy,
  error,
  submitLabel = "스탬프 찍기",
  onSubmit,
  onCancel,
  autoFocus = true,
}: {
  /** 폼을 연 시점의 시세. 목표가 미리보기와 파생 관점의 기준이 된다. */
  basePrice?: number;
  currency?: "KRW" | "USD";
  week52High?: number;
  week52Low?: number;
  busy?: boolean;
  error?: string | null;
  submitLabel?: string;
  onSubmit: (value: StampPredictionValue) => void;
  onCancel?: () => void;
  autoFocus?: boolean;
}) {
  const [target, setTarget] = useState(basePrice ? String(Math.round(basePrice)) : "");
  const [horizon, setHorizon] = useState<StampHorizonKind>("1m");
  const [customDate, setCustomDate] = useState("");
  const [conviction, setConviction] = useState(3);
  const [note, setNote] = useState("");
  const targetRef = useRef<HTMLInputElement>(null);

  // 종목이 바뀌면(모달에서 다른 종목을 고르면) 기준가도 바뀌므로 입력을 새로 채운다.
  const [lastBase, setLastBase] = useState(basePrice);
  if (lastBase !== basePrice) {
    setLastBase(basePrice);
    setTarget(basePrice ? String(Math.round(basePrice)) : "");
  }

  /*
    비로그인이면 저장 버튼이 **눌리지 않는다** (2026-09-05 사용자 요청 4).

    이 폼을 여는 길은 둘이다 — 종목 헤더의 도장 버튼과 스탬프 화면의 「스탬프 찍기」.
    둘 다 이제 **로그인 여부와 무관하게 폼을 연다**(같은 요청). 그래서 401이 날 요청을
    막는 자리는 여기 하나뿐이고, 그 자리가 `disabled`다 — 눌리지 않으니 요청이
    만들어질 길이 없다.

    로그인으로 가는 길은 버튼이 아니라 **아래 한 줄**이 연다(`LoginRequiredNote`).
    죽은 버튼만 두면 다음에 할 일이 화면에 없다.
  */
  const gate = useLoginGate("스탬프");

  /*
    폰·태블릿에서는 **자판을 올리지 않는다** (lib/modal-autofocus.ts). 이 폼은
    언제나 모달 안에서 열리는데, 열리자마자 목표가 칸이 선택되면서 화면 절반이
    자판에 덮여 바로 위의 현재가·52주 범위가 가려졌다.
  */
  const canAutoFocus = useModalAutoFocus();
  useEffect(() => {
    if (autoFocus && canAutoFocus) targetRef.current?.select();
  }, [autoFocus, canAutoFocus]);

  const targetValue = parseTarget(target);
  const base = basePrice ?? 0;
  const stance = stanceForTarget(base, targetValue);
  const expected =
    base > 0 && targetValue > 0 ? ((targetValue - base) / base) * 100 : undefined;
  const canSave = targetValue > 0 && !busy && (horizon !== "custom" || customDate !== "");
  const estimate = estimateDueDate(horizon);

  const submit = () => {
    // 버튼이 이미 막혀 있지만, 폼 어딘가에서 엔터로 들어오는 길이 생겨도 요청이
    // 나가지 않도록 여기서도 한 번 더 확인한다.
    if (!gate.authed || !canSave) return;
    onSubmit({
      targetPrice: targetValue,
      horizonKind: horizon,
      customDueDate: horizon === "custom" ? customDate : undefined,
      conviction,
      note,
    });
  };

  return (
    <div>
      <label className="block text-micro text-[var(--text-muted)]">
        예상 주가 <span className="text-[var(--loss)]">*</span>
      </label>
      <div className="mt-1 flex items-center gap-1">
        <input
          ref={targetRef}
          type="text"
          inputMode="decimal"
          value={target}
          onChange={(e) => setTarget(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter" && canSave) submit();
          }}
          placeholder={base > 0 ? String(Math.round(base)) : "목표 주가"}
          className="h-8 min-w-0 flex-1 rounded-md border border-[var(--border)] bg-[var(--surface-elevated)] px-2 text-cell tabular-nums text-[var(--text-primary)] placeholder:text-[var(--text-muted)] focus:border-[var(--brand)] focus:outline-none"
        />
        <span className="shrink-0 text-cell text-[var(--text-muted)]">
          {currency === "USD" ? "USD" : "원"}
        </span>
      </div>

      <TargetPreview
        basePrice={base}
        target={targetValue}
        expected={expected}
        stance={stance}
        currency={currency}
        week52High={week52High}
        week52Low={week52Low}
      />

      <label className="mt-2.5 block text-micro text-[var(--text-muted)]">
        검증 기간 <span className="text-[var(--loss)]">*</span>
      </label>
      <div className="mt-1 grid grid-cols-3 gap-1">
        {HORIZON_OPTIONS.map((opt) => (
          <button
            key={opt.id}
            type="button"
            title={opt.hint}
            onClick={() => setHorizon(opt.id)}
            className={cn(
              "rounded-md border px-1 py-1 text-micro transition-colors",
              horizon === opt.id
                ? "border-[var(--brand)] bg-[var(--brand-muted)] text-[var(--text-primary)]"
                : "border-[var(--border)] text-[var(--text-muted)] hover:text-[var(--text-secondary)]",
            )}
          >
            {opt.label}
          </button>
        ))}
      </div>
      {horizon === "custom" ? (
        <input
          type="date"
          value={customDate}
          min={toDateInputValue(estimateDueDate("next_session_close"))}
          onChange={(e) => setCustomDate(e.target.value)}
          className="mt-1.5 h-7 w-full rounded-md border border-[var(--border)] bg-[var(--surface-elevated)] px-2 text-cell text-[var(--text-primary)] focus:border-[var(--brand)] focus:outline-none"
        />
      ) : (
        <p className="mt-1.5 text-micro text-[var(--text-muted)]">
          개봉 예정 {formatEstimate(estimate)} 장 마감 · 휴장일이면 다음 거래일로 밀립니다
        </p>
      )}

      <label className="mt-2.5 block text-micro text-[var(--text-muted)]">확신도</label>
      <div className="mt-1 flex gap-1">
        {[1, 2, 3, 4, 5].map((v) => (
          <button
            key={v}
            type="button"
            onClick={() => setConviction(v)}
            className={cn(
              "flex-1 rounded-md border py-1 text-cell transition-colors",
              conviction === v
                ? "border-[var(--brand)] bg-[var(--brand-muted)] text-[var(--text-primary)]"
                : "border-[var(--border)] text-[var(--text-muted)] hover:text-[var(--text-secondary)]",
            )}
          >
            {v}
          </button>
        ))}
      </div>

      <input
        type="text"
        value={note}
        onChange={(e) => setNote(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === "Enter" && canSave) submit();
        }}
        placeholder="이유 한 줄 (선택)"
        className="mt-2.5 h-7 w-full rounded-md border border-[var(--border)] bg-[var(--surface-elevated)] px-2 text-cell text-[var(--text-primary)] placeholder:text-[var(--text-muted)] focus:border-[var(--brand)] focus:outline-none"
      />

      {error && <p className="mt-1.5 text-micro text-[var(--loss)]">{error}</p>}

      <div className="mt-2.5 flex gap-1.5">
        {onCancel && (
          <button
            type="button"
            onClick={onCancel}
            className="rounded-md border border-[var(--border)] px-2.5 py-1.5 text-cell text-[var(--text-muted)] hover:text-[var(--text-secondary)]"
          >
            취소
          </button>
        )}
        <button
          type="button"
          onClick={submit}
          /* 로그인 전에는 아예 눌리지 않는다 — 저장 요청이 만들어질 길을 없앤다. */
          disabled={!gate.authed || !canSave}
          className="flex-1 rounded-md bg-[var(--brand)] py-1.5 text-cell font-medium text-white disabled:opacity-40"
        >
          {busy ? "저장 중…" : gate.label(submitLabel)}
        </button>
      </div>
      <LoginRequiredNote gate={gate} />
      {targetValue <= 0 && (
        <p className="mt-1 text-center text-micro text-[var(--text-muted)]">
          예상 주가를 입력해야 채점할 수 있습니다
        </p>
      )}
    </div>
  );
}

/**
 * 목표가를 52주 범위 위에 찍어 보여준다. 숫자만 있으면 "82,000원"이 이 종목에서
 * 어느 정도의 주장인지 감이 안 온다 — 52주 고가를 넘는 목표인지 아닌지가 한눈에
 * 보여야 한다.
 */
function TargetPreview({
  basePrice,
  target,
  expected,
  stance,
  currency,
  week52High,
  week52Low,
}: {
  basePrice: number;
  target: number;
  expected?: number;
  stance: "bullish" | "bearish" | "neutral";
  currency: "KRW" | "USD";
  week52High?: number;
  week52Low?: number;
}) {
  if (!(basePrice > 0) || !(target > 0)) return null;

  // 트랙은 52주 범위를 쓰되, 목표가가 그 밖으로 나가면 범위를 넓혀 잘리지 않게 한다.
  const lo = Math.min(week52Low || basePrice, basePrice, target);
  const hi = Math.max(week52High || basePrice, basePrice, target);
  const span = hi - lo || 1;
  const pct = (v: number) => Math.max(0, Math.min(100, ((v - lo) / span) * 100));
  const nowPct = pct(basePrice);
  const tgtPct = pct(target);
  const tone =
    stance === "bullish"
      ? "var(--profit)"
      : stance === "bearish"
        ? "var(--loss)"
        : "var(--text-muted)";

  return (
    <div className="mt-2 rounded-lg border border-[var(--border)] bg-[var(--surface-elevated)] px-2.5 py-2">
      <div className="relative h-4">
        <span className="absolute inset-x-0 top-1.5 h-[3px] rounded-full bg-[var(--border)]" />
        <span
          className="absolute top-1.5 h-[3px] rounded-full"
          style={{
            left: `${Math.min(nowPct, tgtPct)}%`,
            width: `${Math.abs(tgtPct - nowPct)}%`,
            background: tone,
          }}
        />
        <span
          className="absolute top-0 h-[15px] w-[2px] bg-[var(--text-muted)]"
          style={{ left: `${nowPct}%` }}
        />
        <span
          className="absolute top-[3px] size-[9px] -translate-x-1/2 rounded-full"
          style={{ left: `${tgtPct}%`, background: tone }}
        />
      </div>
      <div className="mt-1 flex items-baseline justify-between text-micro text-[var(--text-muted)]">
        <span><PriceDisplay value={lo} currency={currency} size="inherit" /></span>
        <span style={{ color: tone }}>
          {STANCE_LABELS[stance]}
          {expected !== undefined && (
            <>
              {" "}
              <b>
                {expected > 0 ? "+" : ""}
                {expected.toFixed(1)}%
              </b>
            </>
          )}
        </span>
        <span><PriceDisplay value={hi} currency={currency} size="inherit" /></span>
      </div>
      {(week52High ?? 0) > 0 && target > (week52High ?? 0) && (
        <p className="mt-1 text-micro text-[var(--text-muted)]">52주 최고가를 넘는 목표입니다</p>
      )}
      {(week52Low ?? 0) > 0 && target < (week52Low ?? 0) && (
        <p className="mt-1 text-micro text-[var(--text-muted)]">52주 최저가를 밑도는 목표입니다</p>
      )}
    </div>
  );
}
