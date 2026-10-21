"use client";

import { useState } from "react";

import { LoginRequiredNote } from "@/components/auth/login-required-note";
import { StockCatalogSearchField } from "@/components/market/stock-catalog-search-field";
import { StockAvatar } from "@/components/stocks/stock-avatar";
import { buttonVariants } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { PriceDisplay } from "@/components/ui/price-display";
import { useLoginGate } from "@/hooks/use-login-gate";
import { useModalAutoFocus } from "@/hooks/use-modal-autofocus";
import { formatPercent, formatPrice } from "@/lib/format";
import { fetchQuoteDetailClient } from "@/lib/market-api";
import type { PredictionHorizon } from "@/lib/prediction-types";
import { savePriceTarget } from "@/lib/price-targets";
import type { StockInfo } from "@/lib/quotes-types";
import { createStampClient } from "@/lib/stamp-api";
import {
  estimateDueDate,
  parseTarget,
  toDateInputValue,
} from "@/components/stamps/stamp-prediction-form";
import type { StampHorizonKind } from "@/lib/stamp-types";
import { cn } from "@/lib/utils";

/**
 * 예측 등록 — 목표가 폼과 스탬프 폼을 하나로.
 *
 * # 기한이 저장 경로를 가른다
 *
 * BE는 아직 두 테이블이다. **기한이 있으면 스탬프**(등록 순간의 시세·점수·뉴스가
 * 함께 박제되고 만기에 채점된다), **무기한이면 목표가**(손절·2차 목표를 함께 두고
 * 도달만 추적한다)로 저장한다. 화면은 한 폼이지만 저장되는 칸이 다르므로
 * **그 차이를 감추지 않고 폼 안에서 말한다** — 손절가 칸이 조용히 무시되면
 * 사용자는 적어 둔 줄 안다.
 *
 * # 필수는 목표가 하나뿐
 *
 * 칸을 여섯 개 세워 두면 등록 자체가 줄어든다. 목표가와 기한만 보이고 나머지는
 * 「자세히」 뒤에 접어 둔다.
 */

const HORIZON_CHOICES: { id: PredictionHorizon; label: string; hint: string }[] = [
  { id: "none", label: "무기한", hint: "채점하지 않고 도달만 추적합니다" },
  { id: "session_close", label: "당일 종가", hint: "오늘 장 마감 (마감 후에는 다음 장)" },
  { id: "next_session_close", label: "다음 장", hint: "다음 거래일 마감" },
  { id: "1w", label: "1주", hint: "일주일 뒤 마감" },
  { id: "1m", label: "1개월", hint: "한 달 뒤 마감" },
  { id: "3m", label: "3개월", hint: "석 달 뒤 마감" },
  { id: "custom", label: "날짜 지정", hint: "고른 날짜의 마감" },
];

const WEEKDAYS = ["일", "월", "화", "수", "목", "금", "토"];

function formatEstimate(d: Date): string {
  return `${d.getFullYear()}년 ${d.getMonth() + 1}월 ${d.getDate()}일 (${WEEKDAYS[d.getDay()]})`;
}

type Quote = {
  price: number;
  changeRate?: number;
  currency: "KRW" | "USD";
  week52High?: number;
  week52Low?: number;
};

export function PredictionCreateDialog({
  open,
  onOpenChange,
  onCreated,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onCreated: () => void;
}) {
  const canAutoFocus = useModalAutoFocus();
  const gate = useLoginGate("예측");

  const [stock, setStock] = useState<StockInfo | null>(null);
  const [quote, setQuote] = useState<Quote | null>(null);
  const [loadingQuote, setLoadingQuote] = useState(false);

  const [target, setTarget] = useState("");
  const [horizon, setHorizon] = useState<PredictionHorizon>("1m");
  const [customDate, setCustomDate] = useState("");
  const [target2, setTarget2] = useState("");
  const [stop, setStop] = useState("");
  const [conviction, setConviction] = useState(3);
  const [note, setNote] = useState("");
  const [advanced, setAdvanced] = useState(false);

  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [done, setDone] = useState<string | null>(null);

  // 열고 닫을 때의 초기화는 렌더 중에 한다 — 이펙트로 하면 닫힌 프레임이 한 번 더
  // 그려지면서 이전 종목이 남는다 (앱 공통 패턴).
  const [wasOpen, setWasOpen] = useState(open);
  if (wasOpen !== open) {
    setWasOpen(open);
    if (open) {
      setStock(null);
      setQuote(null);
      setTarget("");
      setTarget2("");
      setStop("");
      setNote("");
      setAdvanced(false);
      setError(null);
      setDone(null);
    }
  }

  const pick = async (picked: StockInfo) => {
    setStock(picked);
    setQuote(null);
    setError(null);
    setDone(null);
    setLoadingQuote(true);
    try {
      const detail = await fetchQuoteDetailClient(picked.symbol);
      const q: Quote = {
        price: detail.currentPrice,
        changeRate: detail.changeRate,
        currency: detail.currency === "USD" ? "USD" : "KRW",
        week52High: detail.week52High,
        week52Low: detail.week52Low,
      };
      setQuote(q);
      setTarget(q.price > 0 ? String(round(q.price, q.currency)) : "");
    } catch {
      // 시세를 못 받아도 등록 자체는 막지 않는다 — 기한 있는 예측은 서버가 자기
      // 시세로 박제하고, 무기한은 목표가만 있어도 유효하다.
      setError("현재가를 불러오지 못했습니다 — 목표가는 직접 입력해 주세요.");
    } finally {
      setLoadingQuote(false);
    }
  };

  const currency = quote?.currency ?? "KRW";
  const base = quote?.price ?? 0;
  const targetValue = parseTarget(target);
  const target2Value = parseTarget(target2);
  const stopValue = parseTarget(stop);
  const dated = horizon !== "none";
  const expected =
    base > 0 && targetValue > 0 ? ((targetValue - base) / base) * 100 : undefined;
  const upside = targetValue > 0 && base > 0 ? targetValue >= base : true;

  // 손익비 — 목표까지 거리 ÷ 손절까지 거리. 「0.06 : 1」만 적으면 좋고 나쁨을 모르므로
  // 아래에 한 줄 해석을 붙인다.
  const riskReward =
    base > 0 && targetValue > 0 && stopValue > 0 && Math.abs(base - stopValue) > 0
      ? Math.abs(targetValue - base) / Math.abs(base - stopValue)
      : undefined;

  const stopSideValid =
    stopValue <= 0 || base <= 0 || (upside ? stopValue < base : stopValue > base);
  const target2SideValid =
    target2Value <= 0 ||
    targetValue <= 0 ||
    (upside ? target2Value > targetValue : target2Value < targetValue);

  const canSave =
    targetValue > 0 &&
    !busy &&
    stopSideValid &&
    target2SideValid &&
    (horizon !== "custom" || customDate !== "");

  const submit = async () => {
    if (!stock || !gate.authed || !canSave) return;
    setBusy(true);
    setError(null);
    try {
      if (dated) {
        await createStampClient({
          symbol: stock.symbol,
          targetPrice: targetValue,
          horizonKind: horizon as StampHorizonKind,
          customDueDate: horizon === "custom" ? customDate : undefined,
          conviction,
          note,
        });
      } else {
        await savePriceTarget({
          symbol: stock.symbol,
          targetPrice: targetValue,
          targetPrice2: target2Value > 0 ? target2Value : undefined,
          stopPrice: stopValue > 0 ? stopValue : undefined,
          currency,
          note,
        });
      }
      setDone(`${stock.name || stock.symbol} 예측을 저장했습니다.`);
      setStock(null);
      setQuote(null);
      setTarget("");
      setTarget2("");
      setStop("");
      setNote("");
      onCreated();
    } catch (err) {
      const status = (err as { status?: number }).status;
      if (status === 409) {
        setError("이 종목은 방금 등록했습니다 — 5분 뒤에 다시 시도해 주세요.");
      } else if (status === 400) {
        setError("목표가를 확인해 주세요 — 현재가에서 너무 멀리 떨어져 있습니다.");
      } else if (status === 401) {
        setError("로그인 후 이용할 수 있습니다.");
      } else {
        setError("저장에 실패했습니다.");
      }
    } finally {
      setBusy(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[88vh] overflow-y-auto sm:max-w-[440px]">
        <DialogHeader>
          <DialogTitle>예측 세우기</DialogTitle>
          <DialogDescription>
            얼마까지 갈 것인지 적습니다. 기한을 함께 정하면 그때의 시세·점수·뉴스가 저장되고
            만기에 채점됩니다.
          </DialogDescription>
        </DialogHeader>

        {done && (
          <p className="rounded-lg border border-[var(--brand)] bg-[var(--brand-muted)] px-3 py-2 text-cell text-[var(--text-primary)]">
            {done} 다른 종목도 이어서 세울 수 있습니다.
          </p>
        )}

        {!stock ? (
          <>
            <StockCatalogSearchField
              autoFocus={canAutoFocus}
              placeholder="종목명·티커 검색 (예: 삼성전자, NVDA)"
              onSelect={pick}
              suggestionsId="prediction-create-suggestions"
            />
            {error && <p className="text-cell text-[var(--loss)]">{error}</p>}
          </>
        ) : (
          <div className="space-y-2.5">
            {/* 고른 종목 */}
            <div className="flex items-center gap-2.5 rounded-lg border border-[var(--border)] bg-[var(--surface-elevated)] px-2.5 py-2">
              <StockAvatar symbol={stock.symbol} name={stock.name} size="sm" />
              <div className="min-w-0 flex-1">
                <p className="truncate text-cell font-semibold text-[var(--text-primary)]">
                  {stock.name || stock.symbol}
                </p>
                <p className="text-micro tabular-nums text-[var(--text-muted)]">
                  {loadingQuote
                    ? "현재가 불러오는 중…"
                    : quote
                      ? `현재가 ${formatPrice(quote.price, quote.currency)}`
                      : "현재가 —"}
                  {quote?.changeRate !== undefined && (
                    <span
                      className={cn(
                        "ml-1",
                        quote.changeRate > 0
                          ? "text-[var(--profit)]"
                          : quote.changeRate < 0
                            ? "text-[var(--loss)]"
                            : undefined,
                      )}
                    >
                      {formatPercent(quote.changeRate)}
                    </span>
                  )}
                </p>
              </div>
              <button
                type="button"
                onClick={() => {
                  setStock(null);
                  setQuote(null);
                }}
                className={cn(
                  buttonVariants({ variant: "outline", size: "xs" }),
                  "shrink-0 text-micro text-[var(--text-muted)]",
                )}
              >
                종목 변경
              </button>
            </div>

            {/* 목표가 */}
            <div>
              <label
                htmlFor="prediction-target"
                className="block text-micro text-[var(--text-muted)]"
              >
                목표가 <span className="text-[var(--loss)]">*</span>
              </label>
              <div className="mt-1 flex items-center gap-1">
                <input
                  id="prediction-target"
                  type="text"
                  inputMode="decimal"
                  value={target}
                  onChange={(e) => setTarget(e.target.value)}
                  onKeyDown={(e) => {
                    if (e.key === "Enter" && canSave) void submit();
                  }}
                  placeholder={base > 0 ? String(round(base, currency)) : "목표 주가"}
                  className="h-8 min-w-0 flex-1 rounded-md border border-[var(--border)] bg-[var(--surface-elevated)] px-2 text-cell tabular-nums text-[var(--text-primary)] placeholder:text-[var(--text-muted)] focus:border-[var(--brand)] focus:outline-none"
                />
                <span className="shrink-0 text-cell text-[var(--text-muted)]">
                  {currency === "USD" ? "USD" : "원"}
                </span>
              </div>
              {base > 0 && (
                <div className="mt-1 flex flex-wrap gap-1">
                  {[-10, -5, 5, 10, 20].map((p) => (
                    <button
                      key={p}
                      type="button"
                      onClick={() => setTarget(String(round(base * (1 + p / 100), currency)))}
                      className="rounded-md border border-[var(--border)] bg-[var(--surface-elevated)] px-1.5 py-0.5 text-micro font-semibold text-[var(--text-muted)] hover:border-[var(--brand)] hover:text-[var(--brand)]"
                    >
                      {p > 0 ? `+${p}%` : `${p}%`}
                    </button>
                  ))}
                  {quote?.week52High ? (
                    <button
                      type="button"
                      onClick={() => setTarget(String(round(quote.week52High as number, currency)))}
                      className="rounded-md border border-[var(--border)] bg-[var(--surface-elevated)] px-1.5 py-0.5 text-micro font-semibold text-[var(--text-muted)] hover:border-[var(--brand)] hover:text-[var(--brand)]"
                    >
                      52주 고가
                    </button>
                  ) : null}
                </div>
              )}
              <TargetRuler
                base={base}
                target={targetValue}
                expected={expected}
                currency={currency}
                week52High={quote?.week52High}
                week52Low={quote?.week52Low}
              />
            </div>

            {/* 기한 */}
            <div>
              <span className="block text-micro text-[var(--text-muted)]">검증 기한</span>
              <div className="mt-1 grid grid-cols-3 gap-1">
                {HORIZON_CHOICES.map((opt) => (
                  <button
                    key={opt.id}
                    type="button"
                    title={opt.hint}
                    aria-pressed={horizon === opt.id}
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
                  aria-label="만기 날짜"
                  value={customDate}
                  min={toDateInputValue(estimateDueDate("next_session_close"))}
                  onChange={(e) => setCustomDate(e.target.value)}
                  className="mt-1.5 h-7 w-full rounded-md border border-[var(--border)] bg-[var(--surface-elevated)] px-2 text-cell text-[var(--text-primary)] focus:border-[var(--brand)] focus:outline-none"
                />
              ) : (
                <p className="mt-1.5 text-micro leading-relaxed text-[var(--text-muted)]">
                  {dated ? (
                    <>
                      {formatEstimate(estimateDueDate(horizon as StampHorizonKind))} 장 마감에
                      정산 · 휴장일이면 다음 거래일로 밀립니다
                    </>
                  ) : (
                    <>채점하지 않습니다 — 목표·손절에 닿는지만 계속 추적합니다</>
                  )}
                </p>
              )}
            </div>

            {/* 접히는 나머지 */}
            <button
              type="button"
              onClick={() => setAdvanced((v) => !v)}
              className="text-micro font-semibold text-[var(--brand)]"
              aria-expanded={advanced}
            >
              {advanced ? "자세히 접기 ▴" : "자세히 (손절가 · 2차 목표 · 확신 · 메모) ▾"}
            </button>

            {advanced && (
              <div className="space-y-2.5 rounded-lg border border-[var(--border)] p-2.5">
                <div className="grid grid-cols-2 gap-2">
                  <div>
                    <label
                      htmlFor="prediction-target2"
                      className="block text-micro text-[var(--text-muted)]"
                    >
                      2차 목표
                    </label>
                    <input
                      id="prediction-target2"
                      type="text"
                      inputMode="decimal"
                      value={target2}
                      disabled={dated}
                      onChange={(e) => setTarget2(e.target.value)}
                      placeholder={dated ? "무기한만" : "선택"}
                      className="mt-1 h-7 w-full rounded-md border border-[var(--border)] bg-[var(--surface-elevated)] px-2 text-cell tabular-nums text-[var(--text-primary)] placeholder:text-[var(--text-muted)] focus:border-[var(--brand)] focus:outline-none disabled:opacity-45"
                    />
                  </div>
                  <div>
                    <label
                      htmlFor="prediction-stop"
                      className="block text-micro text-[var(--text-muted)]"
                    >
                      손절가
                    </label>
                    <input
                      id="prediction-stop"
                      type="text"
                      inputMode="decimal"
                      value={stop}
                      disabled={dated}
                      onChange={(e) => setStop(e.target.value)}
                      placeholder={dated ? "무기한만" : "선택"}
                      className="mt-1 h-7 w-full rounded-md border border-[var(--border)] bg-[var(--surface-elevated)] px-2 text-cell tabular-nums text-[var(--text-primary)] placeholder:text-[var(--text-muted)] focus:border-[var(--brand)] focus:outline-none disabled:opacity-45"
                    />
                  </div>
                </div>

                {dated ? (
                  <p className="text-micro leading-relaxed text-amber-600 dark:text-amber-400">
                    손절가·2차 목표는 <b>무기한 예측에만</b> 저장됩니다. 기한 있는 예측은
                    만기 채점을 위해 목표가 하나만 굳혀 둡니다.
                  </p>
                ) : (
                  <>
                    {!stopSideValid && (
                      <p className="text-micro text-[var(--loss)]">
                        손절가는 목표의 반대쪽이어야 합니다 — 지금 방향은{" "}
                        {upside ? "상방" : "하방"}입니다.
                      </p>
                    )}
                    {!target2SideValid && (
                      <p className="text-micro text-[var(--loss)]">
                        2차 목표는 1차보다 {upside ? "높아야" : "낮아야"} 합니다.
                      </p>
                    )}
                    {riskReward !== undefined && stopSideValid && (
                      <p className="text-micro leading-relaxed text-[var(--text-muted)]">
                        손익비 <b className="text-[var(--text-primary)]">{riskReward.toFixed(2)} : 1</b>
                        {riskReward < 1 && (
                          <span className="text-amber-600 dark:text-amber-400">
                            {" "}
                            — 목표보다 손절이 멀어, 맞아도 얻는 것이 적습니다
                          </span>
                        )}
                      </p>
                    )}
                  </>
                )}

                <div>
                  <span className="block text-micro text-[var(--text-muted)]">
                    확신 {dated ? "" : "· 무기한 예측에는 저장되지 않습니다"}
                  </span>
                  <div className="mt-1 flex gap-1">
                    {[1, 2, 3, 4, 5].map((v) => (
                      <button
                        key={v}
                        type="button"
                        disabled={!dated}
                        aria-pressed={conviction === v}
                        onClick={() => setConviction(v)}
                        className={cn(
                          "flex-1 rounded-md border py-1 text-cell transition-colors disabled:opacity-45",
                          conviction === v && dated
                            ? "border-[var(--brand)] bg-[var(--brand-muted)] text-[var(--text-primary)]"
                            : "border-[var(--border)] text-[var(--text-muted)] hover:text-[var(--text-secondary)]",
                        )}
                      >
                        {v}
                      </button>
                    ))}
                  </div>
                </div>

                <input
                  type="text"
                  value={note}
                  onChange={(e) => setNote(e.target.value)}
                  onKeyDown={(e) => {
                    if (e.key === "Enter" && canSave) void submit();
                  }}
                  aria-label="메모"
                  placeholder="왜 그렇게 보는가 (선택)"
                  className="h-7 w-full rounded-md border border-[var(--border)] bg-[var(--surface-elevated)] px-2 text-cell text-[var(--text-primary)] placeholder:text-[var(--text-muted)] focus:border-[var(--brand)] focus:outline-none"
                />
              </div>
            )}

            {dated && (
              <p className="rounded-lg bg-[var(--surface-elevated)] px-2.5 py-2 text-micro leading-relaxed text-[var(--text-muted)]">
                저장할 때 <b className="text-[var(--text-secondary)]">지금 이 순간</b>도 함께
                박제됩니다 — 시세·종합점수 5축·AI 요약·뉴스·지수. 나중에 「그때 뭘 보고
                그랬지」를 되짚을 수 있습니다.
              </p>
            )}

            {error && <p className="text-micro text-[var(--loss)]">{error}</p>}

            <div className="flex gap-1.5">
              <button
                type="button"
                onClick={() => onOpenChange(false)}
                className="rounded-md border border-[var(--border)] px-2.5 py-1.5 text-cell text-[var(--text-muted)] hover:text-[var(--text-secondary)]"
              >
                취소
              </button>
              <button
                type="button"
                onClick={submit}
                disabled={!gate.authed || !canSave}
                className="flex-1 rounded-md bg-[var(--brand)] py-1.5 text-cell font-medium text-[var(--brand-foreground)] disabled:opacity-40"
              >
                {busy ? "저장 중…" : gate.label("예측 저장")}
              </button>
            </div>
            <LoginRequiredNote gate={gate} />
            {targetValue <= 0 && (
              <p className="text-center text-micro text-[var(--text-muted)]">
                목표가를 입력해야 저장할 수 있습니다
              </p>
            )}
          </div>
        )}
      </DialogContent>
    </Dialog>
  );
}

/** 원화는 정수로, 달러는 소수 둘째 자리까지. 프리셋이 74,200.0000001을 넣지 않게. */
function round(value: number, currency: "KRW" | "USD"): number {
  return currency === "USD" ? Math.round(value * 100) / 100 : Math.round(value);
}

/**
 * B2 · 목표가 눈금자.
 *
 * 빈 칸에 숫자를 적을 때 기준이 없다. 「82,000원」이 이 종목에서 공격적인 숫자인지
 * 아닌지는 52주 범위 위에 찍어 봐야 안다. 우리가 이미 가진 값이라 외부 호출은 없다.
 */
function TargetRuler({
  base,
  target,
  expected,
  currency,
  week52High,
  week52Low,
}: {
  base: number;
  target: number;
  expected?: number;
  currency: "KRW" | "USD";
  week52High?: number;
  week52Low?: number;
}) {
  if (!(base > 0) || !(target > 0)) return null;

  // 목표가 52주 밖으로 나가면 축을 넓혀 잘리지 않게 한다.
  const lo = Math.min(week52Low || base, base, target);
  const hi = Math.max(week52High || base, base, target);
  const span = hi - lo || 1;
  const at = (v: number) => Math.max(0, Math.min(100, ((v - lo) / span) * 100));
  const nowAt = at(base);
  const tgtAt = at(target);
  const tone =
    target > base ? "var(--profit)" : target < base ? "var(--loss)" : "var(--text-muted)";

  return (
    <div className="mt-2 rounded-lg border border-[var(--border)] bg-[var(--surface-elevated)] px-2.5 py-2">
      <div className="relative h-4">
        <span
          aria-hidden
          className="absolute inset-x-0 top-1.5 h-[3px] rounded-full bg-[var(--border)]"
        />
        <span
          aria-hidden
          className="absolute top-1.5 h-[3px] rounded-full"
          style={{
            left: `${Math.min(nowAt, tgtAt)}%`,
            width: `${Math.abs(tgtAt - nowAt)}%`,
            background: tone,
          }}
        />
        <span
          aria-hidden
          className="absolute top-0 h-[15px] w-[2px] bg-[var(--text-muted)]"
          style={{ left: `${nowAt}%` }}
        />
        <span
          aria-hidden
          className="absolute top-[3px] size-[9px] -translate-x-1/2 rounded-full"
          style={{ left: `${tgtAt}%`, background: tone }}
        />
      </div>
      <div className="mt-1 flex items-baseline justify-between gap-2 text-micro text-[var(--text-muted)]">
        <span className="tabular-nums">
          <PriceDisplay value={lo} currency={currency} size="inherit" />
        </span>
        <span style={{ color: tone }} className="font-semibold tabular-nums">
          {expected === undefined ? "" : formatPercent(expected)}
        </span>
        <span className="tabular-nums">
          <PriceDisplay value={hi} currency={currency} size="inherit" />
        </span>
      </div>
      {(week52High ?? 0) > 0 && target > (week52High as number) && (
        <p className="mt-1 text-micro text-[var(--text-muted)]">
          52주 최고가를 넘는 목표입니다 — 신고가를 요구하는 예측입니다
        </p>
      )}
      {(week52Low ?? 0) > 0 && target < (week52Low as number) && (
        <p className="mt-1 text-micro text-[var(--text-muted)]">52주 최저가를 밑도는 목표입니다</p>
      )}
    </div>
  );
}
