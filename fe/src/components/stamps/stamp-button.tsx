"use client";

import Link from "next/link";
import { useState } from "react";
import { Stamp as StampIcon, X } from "lucide-react";
import {
  StampPredictionForm,
  type StampPredictionValue,
} from "@/components/stamps/stamp-prediction-form";
import { HEADER_ACTION_CLASS } from "@/components/stocks/stock-symbol-action-bar";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { useStampButton } from "@/hooks/use-stamp-button";
import { useLoginGate } from "@/hooks/use-login-gate";
import { formatDateTimeKST, formatPrice } from "@/lib/format";
import { STANCE_LABELS } from "@/lib/stamp-types";
import { cn } from "@/lib/utils";
import { PriceDisplay } from "@/components/ui/price-display";

function cooldownLabel(seconds: number): string {
  const m = Math.floor(seconds / 60);
  const s = seconds % 60;
  return `${m}:${String(s).padStart(2, "0")} 후`;
}

/*
스탬프 폼·확인 패널은 **화면 정중앙 모달**이다 (2026-09-04 사용자 지적).

예전에는 버튼에 매달린 팝오버였고, 좁은 화면에서 기준이 화면 밖을 가리키는 문제를
`lib/anchored-popover`가 「`max-sm`에서 화면 아래에 붙인다」로 덮고 있었다. 잘리지는
않게 됐지만, 그 결과가 **화면 아래 가장자리에 붙은 467px짜리 판**이었다: 예상 주가
입력칸에 손이 닿으면 소프트 키보드가 올라와 판을 덮고, 판 안에서 다시 스크롤해야
했다. 같은 헤더의 옆 버튼 둘(보유종목 등록·목표가 설정)은 이미 정중앙 모달인데
스탬프만 달랐다.

지금은 셋이 같은 `Dialog`를 쓴다. 그러면 덤으로 두 가지가 따라온다 — 모달이 열려
있는 동안 뒤로가기가 앱을 떠나지 않고(`ui/dialog`의 히스토리 점유), 폰·태블릿에서
열자마자 자판이 올라오지 않는다(`lib/modal-autofocus`).
*/

type StampButtonProps = {
  symbol: string;
  /** Chart interval on screen — captured with the stamp. */
  interval?: string;
  /** 현재가. 목표가 미리보기와 파생 관점에 쓰인다 (없으면 미리보기만 빠진다). */
  price?: number;
  currency?: "KRW" | "USD";
  week52High?: number;
  week52Low?: number;
  /**
   * 글자 없이 **아이콘만** 그린다. 모바일 종목 헤더가 동작 셋을 한 줄에 펼칠 때
   * 쓴다(`StockSymbolActionBar`의 「⋯ 는 접힘일 뿐이다」 참조) — 글자를 달면 셋이
   * 종목명의 폭을 다 가져간다.
   */
  iconOnly?: boolean;
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
  iconOnly = false,
  className,
}: StampButtonProps) {
  const { count, cooldown, busy, error, lastStamp, stamp, annotate, dismiss, dismissError } =
    useStampButton(symbol, { interval });
  /*
    비로그인에게도 **폼은 열린다** (2026-09-05 사용자 요청 4).

    09-04에는 여기서 막고 로그인으로 보냈다 — 401이 날 요청은 안 나갔지만, 스탬프가
    무엇을 받는 물건인지(예상 주가·검증 기간·확신도) 로그인 전에는 볼 수 없었다.
    지금은 로그인한 사람과 같은 폼이 열리고, 막는 자리는 폼 안의 저장 버튼 하나다
    (`stamp-prediction-form`에서 `disabled`라 요청이 만들어지지 않는다).

    # 이름은 **하는 일**이다 (2026-09-05 사용자 결정)

    한동안 이 버튼은 비로그인일 때 「로그인하고 스탬프」였다. 지금은 로그인 여부와
    상관없이 **「스탬프 생성」**이다 — 눌렀을 때 하는 일(창을 연다)이 둘 다 같아졌으므로,
    이름이 갈라질 이유가 없어졌다. 헤더에 선 셋이 「로그인하고 …」로 시작하면 세 이름의
    앞부분이 똑같아져 무엇이 무엇인지도 흐려진다.

    로그인이 필요하다는 말은 **그것이 실제로 걸리는 자리**에서 한다 — 창 안의 저장
    버튼(「로그인하고 스탬프」)과 그 아래 한 줄이다. 여기 `gate`는 마우스를 올렸을 때의
    툴팁에만 남는다.
  */
  const gate = useLoginGate("스탬프");

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
      {/* 크기는 공용 `<Button size="sm">`이 정한다 — 옆의 둘과 같은 규격이다.
          글자 크기는 어느 분기로 가든 `text-micro`로 고정한다: 쿨다운·보유 스탬프
          분기가 색만 바꾸는데 HEADER_ACTION_CLASS를 통째로 갈아 끼우는 구조라,
          그 안의 글자 크기까지 같이 빠져 옆 버튼과 어긋나 있었다. */}
      <Button
        variant="outline"
        size="sm"
        onClick={() => (formOpen ? setFormOpen(false) : openForm())}
        disabled={blocked}
        aria-expanded={formOpen}
        // 아이콘만 그리는 모바일 헤더에는 글자가 없으므로 aria-label이 이름을 말한다.
        // 로그인 여부는 여기서 말하지 않는다 — 아래 「이름은 하는 일이다」 참조.
        aria-label={iconOnly ? "스탬프 생성" : undefined}
        title={
          blocked
            ? "같은 종목은 5분에 한 번만 찍을 수 있습니다"
            : gate.authed
              ? "이 순간을 스탬프로 저장"
              : "로그인하면 이 순간을 스탬프로 저장할 수 있습니다"
        }
        className={cn(
          "text-micro",
          iconOnly && "px-2",
          blocked
            ? "cursor-not-allowed text-[var(--text-muted)]"
            : count > 0
              ? "text-[var(--brand)] hover:text-[var(--brand)]"
              : HEADER_ACTION_CLASS,
        )}
      >
        <StampIcon className={cn(count > 0 && !blocked && "fill-[var(--brand-muted)]")} />
        {/* 아이콘만 그릴 때도 쿨다운은 말해 준다 — 눌리지 않는 이유가 화면에 없으면
            사용자는 버튼이 고장 난 줄 안다. */}
        {iconOnly
          ? blocked
            ? cooldownLabel(cooldown)
            : null
          : blocked
            ? cooldownLabel(cooldown)
            : "스탬프 생성"}
        {count > 0 && (
          <span className="ml-0.5 rounded-full bg-[var(--surface-elevated)] px-1.5 text-micro font-semibold text-[var(--text-secondary)]">
            {count}
          </span>
        )}
      </Button>

      {/*
        실패 문구는 **닫을 수 있고 스스로도 사라진다** (2026-09-04 사용자 지적 2).

        예전에는 지우는 길이 하나도 없어서 화면에 눌어붙었다. 자동 소멸은 훅이
        맡고(8초), 여기서는 **기다리지 않고 지울 길**을 준다 — 좁은 화면에서 이
        말풍선은 바로 아래 내용을 덮으므로, 읽고 나서 치울 수 있어야 한다.

        `role="status"`는 스크린 리더가 이 문구를 읽게 한다. 저장 실패를 눈으로만
        알리면 그 사용자에게는 아무 일도 안 일어난 것이 된다.
      */}
      {error && !lastStamp && (
        <div
          role="status"
          className="absolute right-0 top-full z-20 mt-1 flex w-[240px] max-w-[calc(100vw-1.5rem)] items-start gap-1.5 rounded-md border border-[var(--border)] bg-[var(--surface)] px-2 py-1 text-micro text-[var(--text-muted)] shadow-sm"
        >
          <span className="min-w-0 flex-1">{error}</span>
          <button
            type="button"
            onClick={dismissError}
            aria-label="알림 닫기"
            className="-mr-0.5 shrink-0 rounded p-0.5 text-[var(--text-muted)] hover:bg-[var(--surface-elevated)] hover:text-[var(--text-primary)]"
          >
            <X className="size-3" aria-hidden="true" />
          </button>
        </div>
      )}

      {/* ── 폼: 저장 전 ─────────────────────────────────────────── */}
      <Dialog
        open={formOpen && !lastStamp}
        onOpenChange={(next) => {
          if (!next) setFormOpen(false);
        }}
      >
        <DialogContent className="sm:max-w-[420px]">
          <DialogHeader>
            <DialogTitle>이 순간을 예측으로 남기기</DialogTitle>
            <DialogDescription>
              {basePrice > 0 ? (
                <>
                  현재가 <PriceDisplay value={basePrice} currency={currency} size="inherit" />
                  {" 기준으로 지금 화면의 시세·점수·뉴스가 그대로 박제됩니다."}
                </>
              ) : (
                "지금 화면의 시세·점수·뉴스가 그대로 박제됩니다."
              )}
            </DialogDescription>
          </DialogHeader>

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
        </DialogContent>
      </Dialog>
      {/* ── 저장 후 확인 ────────────────────────────────────────── */}
      <Dialog
        open={Boolean(lastStamp)}
        onOpenChange={(next) => {
          if (next) return;
          dismiss();
          setFormOpen(false);
        }}
      >
        {lastStamp && (
          <DialogContent className="sm:max-w-[380px]">
            <DialogHeader>
              <DialogTitle>스탬프 저장됨</DialogTitle>
              <DialogDescription>
                {formatDateTimeKST(lastStamp.stampedAt)} ·{" "}
                <PriceDisplay
                  value={lastStamp.price}
                  currency={lastStamp.currency === "USD" ? "USD" : "KRW"}
                  size="inherit"
                />
                {lastStamp.score != null && ` · 점수 ${lastStamp.score}`}
              </DialogDescription>
            </DialogHeader>

            <div className="rounded-lg border border-[var(--border)] bg-[var(--surface-elevated)] px-2.5 py-2">
              <div className="flex items-baseline justify-between text-cell">
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
              <div className="mt-0.5 flex items-baseline justify-between text-cell">
                <span className="text-[var(--text-muted)]">개봉</span>
                <span className="text-[var(--text-primary)]">
                  {formatDateTimeKST(lastStamp.dueAt)}
                </span>
              </div>
              <div className="mt-0.5 flex items-baseline justify-between text-cell">
                <span className="text-[var(--text-muted)]">관점</span>
                <span className="text-[var(--text-primary)]">
                  {STANCE_LABELS[lastStamp.stance]} · 확신 {lastStamp.conviction}
                </span>
              </div>
            </div>

            <div className="flex gap-1">
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
                className="h-8 min-w-0 flex-1 rounded-md border border-[var(--border)] bg-[var(--surface-elevated)] px-2 text-cell text-[var(--text-primary)] placeholder:text-[var(--text-muted)] focus:border-[var(--brand)] focus:outline-none"
              />
              <button
                type="button"
                onClick={saveNote}
                disabled={saving || note === lastStamp.note}
                className="rounded-md bg-[var(--brand)] px-3 text-cell font-medium text-[var(--brand-foreground)] disabled:opacity-40"
              >
                {noteSaved ? "저장됨" : "저장"}
              </button>
            </div>

            <Link
              href={`/predictions?symbol=${encodeURIComponent(symbol)}`}
              className="text-micro text-[var(--brand)] hover:underline"
            >
              내 예측 보기 →
            </Link>
          </DialogContent>
        )}
      </Dialog>
    </div>
  );
}
