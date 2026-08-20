"use client";

import Link from "next/link";
import { useState } from "react";
import { Stamp as StampIcon, X } from "lucide-react";
import {
  StampPredictionForm,
  type StampPredictionValue,
} from "@/components/stamps/stamp-prediction-form";
import { useStampButton } from "@/hooks/use-stamp-button";
import { formatDateTimeKST, formatPrice } from "@/lib/format";
import { STANCE_LABELS } from "@/lib/stamp-types";
import { cn } from "@/lib/utils";

function cooldownLabel(seconds: number): string {
  const m = Math.floor(seconds / 60);
  const s = seconds % 60;
  return `${m}:${String(s).padStart(2, "0")} 후`;
}

type StampButtonProps = {
  symbol: string;
  /** Chart interval on screen — captured with the stamp. */
  interval?: string;
  /** 현재가. 목표가 미리보기와 파생 관점에 쓰인다 (없으면 미리보기만 빠진다). */
  price?: number;
  currency?: "KRW" | "USD";
  week52High?: number;
  week52Low?: number;
  /** Matches the surrounding header's button sizing. */
  size?: "sm" | "md";
  className?: string;
};

/**
 * 스탬프 = 예측이다. v1은 한 번 누르면 바로 저장하고 가설을 나중에 붙였지만,
 * 채점의 근거가 되는 **예상 주가가 필수**가 된 이상 저장할 것이 없는 상태로 행을
 * 만들 수 없다. 그래서 폼이 저장보다 먼저 온다.
 *
 * 시세는 폼을 여는 순간 얼려 화면에 보여준다 — 사용자가 보고 판단한 가격과 서버가
 * 박제하는 가격이 몇 초 차이로 갈라지면 "이 가격을 보고 이렇게 예상했다"는 기록이
 * 어긋난다. (서버는 자기 시세를 쓰므로 완전히 같지는 않지만, 화면에 보인 값이
 * 무엇이었는지는 남는다.)
 */
export function StampButton({
  symbol,
  interval,
  price,
  currency = "KRW",
  week52High,
  week52Low,
  size = "sm",
  className,
}: StampButtonProps) {
  const { count, cooldown, busy, error, lastStamp, stamp, annotate, dismiss } = useStampButton(
    symbol,
    { interval },
  );

  const [formOpen, setFormOpen] = useState(false);
  const [frozenPrice, setFrozenPrice] = useState<number | undefined>(undefined);
  const [note, setNote] = useState("");
  const [noteSaved, setNoteSaved] = useState(false);
  const [saving, setSaving] = useState(false);

  const blocked = cooldown > 0;
  const basePrice = frozenPrice ?? price ?? 0;

  const openForm = () => {
    // 폼을 여는 순간의 시세를 기준값으로 잡는다.
    setFrozenPrice(price);
    setNote("");
    setNoteSaved(false);
    setFormOpen(true);
  };

  const submit = async (value: StampPredictionValue) => {
    const created = await stamp(value);
    if (created) {
      setNote(value.note);
      setFormOpen(false);
    }
  };

  const saveNote = async () => {
    if (!lastStamp) return;
    setSaving(true);
    try {
      await annotate({ conviction: lastStamp.conviction, note, tags: lastStamp.tags });
      setNoteSaved(true);
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className={cn("relative", className)}>
      <button
        type="button"
        onClick={() => (formOpen ? setFormOpen(false) : openForm())}
        disabled={blocked}
        aria-expanded={formOpen}
        title={blocked ? "같은 종목은 5분에 한 번만 찍을 수 있습니다" : "이 순간을 스탬프로 저장"}
        className={cn(
          "inline-flex shrink-0 items-center gap-1 rounded-lg border border-[var(--border)] hover:bg-[var(--surface-elevated)]",
          size === "sm" ? "px-2 py-1 text-[11px]" : "px-2.5 py-1.5 text-[11px]",
          blocked
            ? "cursor-not-allowed text-[var(--text-muted)]"
            : count > 0
              ? "text-[var(--brand)]"
              : "text-[var(--text-secondary)]",
        )}
      >
        <StampIcon className={cn("h-3.5 w-3.5", count > 0 && !blocked && "fill-[var(--brand-muted)]")} />
        {blocked ? cooldownLabel(cooldown) : "스탬프"}
        {count > 0 && (
          <span className="ml-0.5 rounded-full bg-[var(--surface-elevated)] px-1.5 text-[10px] font-semibold text-[var(--text-secondary)]">
            {count}
          </span>
        )}
      </button>

      {error && !lastStamp && (
        <p className="absolute right-0 top-full z-20 mt-1 w-[240px] rounded-md border border-[var(--border)] bg-[var(--surface)] px-2 py-1 text-[10px] text-[var(--text-muted)]">
          {error}
        </p>
      )}

      {/* ── 폼: 저장 전 ─────────────────────────────────────────── */}
      {formOpen && !lastStamp && (
        <div className="absolute right-0 top-full z-30 mt-1.5 w-[292px] rounded-xl border border-[var(--border)] bg-[var(--surface)] p-3 shadow-lg">
          <div className="flex items-start justify-between gap-2">
            <div className="min-w-0">
              <p className="text-xs font-semibold text-[var(--text-primary)]">이 순간을 예측으로 남기기</p>
              {basePrice > 0 && (
                <p className="mt-0.5 text-[10px] text-[var(--text-muted)]">
                  현재가 {formatPrice(basePrice, currency)} 기준
                </p>
              )}
            </div>
            <button
              type="button"
              onClick={() => setFormOpen(false)}
              aria-label="닫기"
              className="rounded-md p-0.5 text-[var(--text-muted)] hover:text-[var(--text-secondary)]"
            >
              <X className="h-3.5 w-3.5" />
            </button>
          </div>

          <p className="mt-2 rounded-md bg-[var(--brand-muted)] px-2 py-1.5 text-[10px] text-[var(--brand)]">
            지금 화면의 시세·점수·뉴스가 그대로 박제됩니다
          </p>

          <div className="mt-2.5">
            <StampPredictionForm
              basePrice={basePrice > 0 ? basePrice : undefined}
              currency={currency}
              week52High={week52High}
              week52Low={week52Low}
              busy={busy}
              error={error}
              onSubmit={submit}
              onCancel={() => setFormOpen(false)}
            />
          </div>
        </div>
      )}
      {/* ── 저장 후 확인 ────────────────────────────────────────── */}
      {lastStamp && (
        <div className="absolute right-0 top-full z-30 mt-1.5 w-[268px] rounded-xl border border-[var(--border)] bg-[var(--surface)] p-3 shadow-lg">
          <div className="flex items-start justify-between gap-2">
            <div className="min-w-0">
              <p className="text-xs font-semibold text-[var(--text-primary)]">스탬프 저장됨</p>
              <p className="mt-0.5 text-[10px] text-[var(--text-muted)]">
                {formatDateTimeKST(lastStamp.stampedAt)} ·{" "}
                {formatPrice(lastStamp.price, lastStamp.currency === "USD" ? "USD" : "KRW")}
                {lastStamp.score != null && ` · 점수 ${lastStamp.score}`}
              </p>
            </div>
            <button
              type="button"
              onClick={() => {
                dismiss();
                setFormOpen(false);
              }}
              aria-label="닫기"
              className="rounded-md p-0.5 text-[var(--text-muted)] hover:text-[var(--text-secondary)]"
            >
              <X className="h-3.5 w-3.5" />
            </button>
          </div>

          <div className="mt-2 rounded-lg border border-[var(--border)] bg-[var(--surface-elevated)] px-2.5 py-2">
            <div className="flex items-baseline justify-between text-[11px]">
              <span className="text-[var(--text-muted)]">예상 주가</span>
              <span className="tabular-nums text-[var(--text-primary)]">
                {formatPrice(
                  lastStamp.targetPrice,
                  lastStamp.currency === "USD" ? "USD" : "KRW",
                )}
                {lastStamp.expectedReturn !== undefined && (
                  <span
                    className={cn(
                      "ml-1",
                      lastStamp.expectedReturn > 0
                        ? "text-[var(--profit)]"
                        : lastStamp.expectedReturn < 0
                          ? "text-[var(--loss)]"
                          : "text-[var(--text-muted)]",
                    )}
                  >
                    {lastStamp.expectedReturn > 0 ? "+" : ""}
                    {lastStamp.expectedReturn.toFixed(1)}%
                  </span>
                )}
              </span>
            </div>
            <div className="mt-0.5 flex items-baseline justify-between text-[11px]">
              <span className="text-[var(--text-muted)]">개봉</span>
              <span className="text-[var(--text-primary)]">
                {formatDateTimeKST(lastStamp.dueAt)}
              </span>
            </div>
            <div className="mt-0.5 flex items-baseline justify-between text-[11px]">
              <span className="text-[var(--text-muted)]">관점</span>
              <span className="text-[var(--text-primary)]">
                {STANCE_LABELS[lastStamp.stance]} · 확신 {lastStamp.conviction}
              </span>
            </div>
          </div>

          <div className="mt-2 flex gap-1">
            <input
              type="text"
              value={note}
              onChange={(e) => {
                setNote(e.target.value);
                setNoteSaved(false);
              }}
              onKeyDown={(e) => {
                if (e.key === "Enter") saveNote();
              }}
              placeholder="이유 한 줄 (선택)"
              className="h-7 min-w-0 flex-1 rounded-md border border-[var(--border)] bg-[var(--surface-elevated)] px-2 text-[11px] text-[var(--text-primary)] placeholder:text-[var(--text-muted)] focus:border-[var(--brand)] focus:outline-none"
            />
            <button
              type="button"
              onClick={saveNote}
              disabled={saving || note === lastStamp.note}
              className="rounded-md bg-[var(--brand)] px-2 text-[11px] font-medium text-white disabled:opacity-40"
            >
              {noteSaved ? "저장됨" : "저장"}
            </button>
          </div>

          <Link
            href={`/stamps?symbol=${encodeURIComponent(symbol)}`}
            className="mt-2 block text-[10px] text-[var(--brand)] hover:underline"
          >
            내 스탬프 보기 →
          </Link>
        </div>
      )}
    </div>
  );
}
