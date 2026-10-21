"use client";

import { useState } from "react";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { StockAvatar } from "@/components/stocks/stock-avatar";
import { formatDateKST, formatPercent, formatPrice } from "@/lib/format";
import { SCORE_AXIS_LABELS, SCORE_AXIS_ORDER, type ScoreAxisId } from "@/lib/stock-score-api";
import {
  BENCHMARK_LABELS,
  HORIZON_LABELS,
  STANCE_LABELS,
  type Stamp,
  type StampUpdateInput,
} from "@/lib/stamp-types";
import { cn } from "@/lib/utils";
import { PriceDisplay } from "@/components/ui/price-display";
import { buttonVariants } from "@/components/ui/button";

/**
 * 개봉 — 이 화면의 하이라이트.
 *
 * v1에서 개봉은 상세를 펼쳤을 때 나오는 작은 버튼 하나였다. 검증 기간이 끝난 예측을
 * 확인하는 순간이 이 기능의 전부인데 그 순간에 아무 일도 일어나지 않았던 셈이다.
 *
 * 여기서는 대기 중인 스탬프를 한 장씩 넘기며 결과를 공개하고, **왜 맞았/틀렸는지
 * 회고 한 줄**을 받는다. 회고는 기존 `note` 뒤에 이어 붙인다 — 원래 적어둔 이유를
 * 지우면 다음에 이 스탬프를 볼 때 판단의 앞뒤가 사라진다.
 */

function toneClass(value: number | undefined): string {
  if (value === undefined) return "text-[var(--text-muted)]";
  if (value > 0) return "text-[var(--profit)]";
  if (value < 0) return "text-[var(--loss)]";
  return "text-[var(--text-secondary)]";
}

/** 회고를 원문 뒤에 이어 붙인다. 이미 붙어 있으면 그 뒤를 갈아끼운다. */
const REFLECTION_MARK = "\n— 회고: ";

function mergeReflection(note: string, reflection: string): string {
  const base = note.split(REFLECTION_MARK)[0].trim();
  const text = reflection.trim();
  if (!text) return base;
  return base ? `${base}${REFLECTION_MARK}${text}` : `회고: ${text}`;
}

export function StampRevealModal({
  open,
  onOpenChange,
  stamps,
  onOpen,
  onAnnotate,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** 개봉 대기 목록. 열려 있는 동안에는 이 배열이 줄어도 인덱스를 유지한다. */
  stamps: Stamp[];
  onOpen: (id: number) => Promise<void>;
  onAnnotate: (id: number, input: StampUpdateInput) => Promise<void>;
}) {
  const [index, setIndex] = useState(0);
  // 회고 입력은 **자기가 어느 카드의 것인지** 들고 다닌다. 인덱스가 바뀌면 키가
  // 어긋나 자동으로 빈 값이 되므로, 카드를 넘길 때 비우는 이펙트가 필요 없다
  // (이펙트로 비우면 이전 카드의 글이 한 프레임 남는다).
  const [draft, setDraft] = useState<{ index: number; text: string }>({ index: 0, text: "" });
  const [busy, setBusy] = useState(false);
  const [queue, setQueue] = useState<Stamp[]>(stamps);

  const reflection = draft.index === index ? draft.text : "";
  const setReflection = (text: string) => setDraft({ index, text });

  // 열릴 때 대기 목록을 한 번 고정한다. 개봉할 때마다 원본 배열에서 빠지므로
  // 그대로 따라가면 "다음"을 누르기도 전에 카드가 사라진다.
  const [wasOpen, setWasOpen] = useState(open);
  if (wasOpen !== open) {
    setWasOpen(open);
    if (open) {
      setQueue(stamps);
      setIndex(0);
      setDraft({ index: 0, text: "" });
    }
  }

  const stamp = queue[index];

  if (!stamp) {
    return (
      <Dialog open={open} onOpenChange={onOpenChange}>
        <DialogContent className="sm:max-w-[440px]">
          <DialogHeader>
            <DialogTitle>개봉할 스탬프가 없습니다</DialogTitle>
            <DialogDescription>
              검증 기간이 끝난 스탬프가 생기면 여기에서 결과를 확인할 수 있습니다.
            </DialogDescription>
          </DialogHeader>
        </DialogContent>
      </Dialog>
    );
  }

  const currency = stamp.currency === "USD" ? "USD" : "KRW";
  const hit = stamp.verdict === "hit";
  const settled = stamp.horizon;
  const remaining = queue.length - index - 1;

  const confirm = async () => {
    setBusy(true);
    try {
      if (reflection.trim()) {
        await onAnnotate(stamp.id, {
          conviction: stamp.conviction,
          note: mergeReflection(stamp.note, reflection),
          tags: stamp.tags,
        });
      }
      await onOpen(stamp.id);
      if (remaining > 0) setIndex((i) => i + 1);
      else onOpenChange(false);
    } finally {
      setBusy(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[88vh] overflow-y-auto p-0 sm:max-w-[460px]">
        <DialogHeader className="sr-only">
          <DialogTitle>
            {stamp.symbolName || stamp.symbol} 스탬프 개봉 — {hit ? "적중" : "빗나감"}
          </DialogTitle>
          <DialogDescription>검증 기간이 끝난 스탬프의 결과를 확인합니다.</DialogDescription>
        </DialogHeader>

        {/* 결과 공개 */}
        <div
          className={cn(
            "px-4 pb-4 pt-6 text-center",
            hit
              ? "bg-gradient-to-b from-[var(--profit)]/12 to-transparent"
              : "bg-gradient-to-b from-amber-500/12 to-transparent",
          )}
        >
          <div className="flex items-center justify-center gap-1.5 text-cell text-[var(--text-muted)]">
            <StockAvatar symbol={stamp.symbol} name={stamp.symbolName} size="xs" />
            {stamp.symbolName || stamp.symbol} · {formatDateKST(stamp.stampedAt)}{" "}
            {STANCE_LABELS[stamp.stance]} · {HORIZON_LABELS[stamp.horizonKind]}
          </div>
          <p
            className={cn(
              "mt-1 text-[26px] font-extrabold tracking-tight",
              hit ? "text-[var(--profit)]" : "text-amber-600 dark:text-amber-400",
            )}
          >
            {stamp.verdict === "unknown" ? "판정 불가" : hit ? "적중" : "빗나감"}
          </p>
          <p className="mt-1 text-cell text-[var(--text-muted)]">
            <PriceDisplay value={stamp.price} currency={currency} size="inherit" /> → <b>{settled?.price !== undefined
              ? formatPrice(settled.price, currency)
              : stamp.current
                ? formatPrice(stamp.current.price, currency)
                : "—"}</b>{" "}
            <span className={toneClass(settled?.return ?? stamp.current?.changeRate)}>
              ({formatPercent(settled?.return ?? stamp.current?.changeRate ?? 0)})
            </span>
            {settled?.alpha !== undefined && (
              <>
                {" · "}
                {BENCHMARK_LABELS[stamp.benchmarkId ?? ""] ?? "벤치마크"} 대비{" "}
                <b className={toneClass(settled.alpha)}>알파 {formatPercent(settled.alpha)}</b>
              </>
            )}
          </p>
          {stamp.targetHit && (
            <p className="mt-1 text-cell text-[var(--profit)]">
              {formatDateKST(stamp.targetHit.date)} 장중 목표가{" "}
              <PriceDisplay value={stamp.targetPrice} currency={currency} size="inherit" />에 도달해 조기 종료됐습니다
            </p>
          )}

          {queue.length > 1 && (
            <>
              <div className="mt-2.5 flex justify-center gap-1">
                {queue.map((s, i) => (
                  <span
                    key={s.id}
                    className={cn(
                      "block h-[3px] w-4 rounded-sm",
                      i === index ? "bg-[var(--brand)]" : "bg-[var(--surface-elevated)]",
                    )}
                  />
                ))}
              </div>
              <p className="mt-1 text-micro text-[var(--text-muted)]">
                {index + 1} / {queue.length}
              </p>
            </>
          )}
        </div>

        <div className="grid gap-2.5 px-4 pb-4">
          <Panel title="내가 예상한 것">
            <Row
              label="예상 주가"
              value={`${formatPrice(stamp.targetPrice, currency)}${
                stamp.expectedReturn !== undefined
                  ? ` (${stamp.expectedReturn > 0 ? "+" : ""}${stamp.expectedReturn.toFixed(1)}%)`
                  : ""
              }`}
            />
            {stamp.targetAccuracy !== undefined && (
              <Row
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
            <Row label="확신도" value={`${stamp.conviction} / 5`} />
            {stamp.note && (
              <p className="mt-1.5 border-t border-[var(--border)] pt-1.5 text-cell text-[var(--text-secondary)]">
                {stamp.note}
              </p>
            )}
          </Panel>

          {(stamp.peakReturn !== undefined || stamp.troughReturn !== undefined) && (
            <Panel title="스탬프 이후 이 종목이 지나온 폭">
              <Row
                label="최고"
                value={stamp.peakReturn === undefined ? "—" : formatPercent(stamp.peakReturn)}
                valueClass={toneClass(stamp.peakReturn)}
              />
              <Row
                label="최저"
                value={stamp.troughReturn === undefined ? "—" : formatPercent(stamp.troughReturn)}
                valueClass={toneClass(stamp.troughReturn)}
              />
            </Panel>
          )}

          {(stamp.score != null || stamp.current?.score != null) && (
            <Panel title="당시 → 지금, 무엇이 달라졌나">
              <Row
                label="종합 점수"
                value={`${stamp.score ?? "—"} → ${stamp.current?.score ?? "—"}`}
              />
              {SCORE_AXIS_ORDER.map((axis: ScoreAxisId) => {
                const then = stamp.snapshot.breakdown?.[axis];
                const now = stamp.current?.breakdown?.[axis];
                if (then === undefined && now === undefined) return null;
                const delta = then !== undefined && now !== undefined ? now - then : undefined;
                if (delta === undefined || Math.abs(delta) < 5) return null;
                return (
                  <Row
                    key={axis}
                    label={SCORE_AXIS_LABELS[axis]}
                    value={`${Math.round(then as number)} → ${Math.round(now as number)}`}
                    valueClass={toneClass(delta)}
                  />
                );
              })}
            </Panel>
          )}

          <Panel title={hit ? "회고 — 왜 맞았나요? (선택)" : "회고 — 무엇을 놓쳤나요? (선택)"}>
            <input
              type="text"
              value={reflection}
              onChange={(e) => setReflection(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter" && !busy) confirm();
              }}
              placeholder={hit ? "예: 수급 근거가 실제로 이어졌다" : "예: 실적 발표 일정을 못 봤다"}
              className="h-8 w-full rounded-md border border-[var(--border)] bg-[var(--surface-elevated)] px-2 text-cell text-[var(--text-primary)] placeholder:text-[var(--text-muted)] focus:border-[var(--brand)] focus:outline-none"
            />
            <p className="mt-1 text-micro text-[var(--text-muted)]">
              원래 적어둔 이유 뒤에 붙습니다 — 지우지 않습니다
            </p>
          </Panel>
        </div>

        <div className="flex items-center gap-2 border-t border-[var(--border)] px-4 py-3">
          <button
            type="button"
            disabled={busy}
            onClick={() => (remaining > 0 ? setIndex((i) => i + 1) : onOpenChange(false))}
            className="rounded-lg px-2.5 py-1.5 text-cell text-[var(--text-muted)] hover:text-[var(--text-secondary)]"
          >
            건너뛰기
          </button>
          <span className="ml-auto text-micro text-[var(--text-muted)]">
            {remaining > 0 ? `남은 ${remaining}건` : "마지막"}
          </span>
          <button
            type="button"
            disabled={busy}
            onClick={confirm}
            className={cn(buttonVariants({ variant: "default", size: "sm" }), "bg-[var(--brand)] text-[var(--brand-foreground)] disabled:opacity-40")}
          >
            {busy ? "저장 중…" : remaining > 0 ? "확인하고 다음 →" : "확인"}
          </button>
        </div>
      </DialogContent>
    </Dialog>
  );
}

function Panel({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <section className="rounded-lg border border-[var(--border)] p-2.5">
      <h4 className="mb-1.5 text-micro font-semibold text-[var(--text-muted)]">{title}</h4>
      {children}
    </section>
  );
}

function Row({
  label,
  value,
  valueClass,
}: {
  label: string;
  value: string;
  valueClass?: string;
}) {
  return (
    <div className="flex items-baseline justify-between py-0.5 text-cell">
      <span className="text-[var(--text-muted)]">{label}</span>
      <span className={cn("tabular-nums text-[var(--text-secondary)]", valueClass)}>{value}</span>
    </div>
  );
}
