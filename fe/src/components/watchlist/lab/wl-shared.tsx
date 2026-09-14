"use client";

import { NotepadText, Stamp, Target, Wallet } from "lucide-react";
import { IndexSparkline } from "@/components/market/index-sparkline";
import { StockAvatar } from "@/components/stocks/stock-avatar";
import { ValueFlash } from "@/components/ui/value-flash";
import { useInView } from "@/hooks/use-in-view";
import { useSparklineCandles } from "@/hooks/use-sparkline-candles";
import { formatPercent, formatPrice, formatPriceChange } from "@/lib/format";
import type { PriceTarget } from "@/lib/price-targets";
import type { QuoteSummary } from "@/lib/quotes-types";
import { SCORE_TONE_BADGE, scoreTone } from "@/lib/stock-score-api";
import { trendTextClass } from "@/lib/trend-colors";
import type { SignalTone, WatchSignal } from "@/lib/watchlist-signals";
import type { WatchlistRow } from "@/lib/watchlist-calc";
import { cn } from "@/lib/utils";

/**
 * 관심종목 개편 시안이 함께 쓰는 부품 (2026-09-07).
 *
 * 시안 넷이 **같은 부품**을 쓰는 것이 중요하다. 시안마다 종목 이름·가격·신호를 다르게
 * 그리면 사용자가 보는 차이가 「배치의 차이」가 아니라 「누가 더 예쁘게 그렸나」가 된다.
 * 넷이 다른 것은 **무엇을 어디에 놓느냐**뿐이어야 한다.
 */

/** 이 종목에 붙어 있는 **내 기록**. 목록 화면이 한 번씩만 받아 심볼로 나눠 준다. */
export type SymbolRecord = {
  target?: PriceTarget;
  notes: number;
  stamps: number;
  held: boolean;
};

export const EMPTY_RECORD: SymbolRecord = { notes: 0, stamps: 0, held: false };

/** 시안 넷이 똑같이 받는 것 — 몸통이 만든 행·기록·신호와 두 동작. */
export type WlListProps = {
  rows: WatchlistRow[];
  records: Map<string, SymbolRecord>;
  signals: Map<string, WatchSignal[]>;
  onOpen: (symbol: string) => void;
  onRemove: (symbol: string) => void;
};

const CHIP_TONE: Record<SignalTone, string> = {
  up: "border-[var(--profit)]/35 bg-[var(--profit)]/10 text-[var(--profit)]",
  down: "border-[var(--loss)]/35 bg-[var(--loss)]/10 text-[var(--loss)]",
  mine: "border-[var(--brand)]/40 bg-[var(--brand-muted)] text-[var(--brand)]",
  stop: "border-[var(--danger)]/50 bg-[var(--danger)]/12 font-bold text-[var(--danger)]",
  date: "border-[var(--border)] bg-[var(--surface-elevated)] text-[var(--text-secondary)]",
  note: "border-transparent bg-[var(--surface-elevated)] text-[var(--text-muted)]",
};

export function SignalChip({ signal }: { signal: WatchSignal }) {
  return (
    <span
      title={signal.detail}
      className={cn(
        "inline-flex shrink-0 items-center rounded-full border px-1.5 py-[1px] text-micro font-medium leading-[15px]",
        CHIP_TONE[signal.tone],
      )}
    >
      {signal.label}
    </span>
  );
}

/**
 * 칩으로 **인쇄하지 않을** 신호 — 다른 자리에 이미 같은 숫자가 적혀 있는 것.
 *
 * 시안 셋 모두 「담은 뒤」를 종목 이름 아래(또는 카드 바닥)에 적는다. 칩으로 한 번 더
 * 세우면 같은 값이 한 행에 두 번 나온다(실측: 삼성전자 행에 `담은 뒤 -21.20%`가 2cm
 * 간격으로 두 번). 판정에서는 그대로 쓴다 — 지운 것은 표시뿐이라 주목도·칸 나눔은
 * 달라지지 않는다.
 */
export function chipSignals(signals: WatchSignal[]): WatchSignal[] {
  return signals.filter((s) => s.id !== "since");
}

/**
 * 신호 줄. **없으면 아무것도 그리지 않는다** — 「특이사항 없음」 한 줄을 종목마다
 * 인쇄하면 조용한 종목이 화면에서 제일 시끄러워진다.
 */
export function SignalChipRow({
  signals,
  max = 4,
  className,
}: {
  signals: WatchSignal[];
  max?: number;
  className?: string;
}) {
  if (signals.length === 0) return null;
  const shown = signals.slice(0, max);
  const rest = signals.length - shown.length;
  return (
    <span className={cn("flex min-w-0 flex-wrap items-center gap-1", className)}>
      {shown.map((s) => (
        <SignalChip key={s.id} signal={s} />
      ))}
      {rest > 0 && (
        <span
          className="text-micro text-[var(--text-muted)]"
          title={signals
            .slice(max)
            .map((s) => s.label)
            .join(" / ")}
        >
          +{rest}
        </span>
      )}
    </span>
  );
}

/**
 * 내 기록 표식 — 메모·스탬프·목표가·보유.
 *
 * 신호 칩과 **같은 줄에 두지 않는다.** 신호는 "오늘 생긴 일"이고 이건 "내가 예전에 해
 * 둔 것"이라, 섞으면 어느 쪽이 새 소식인지 알 수 없다. 그래서 표식은 종목 이름 옆에
 * 조용히 붙는다.
 */
export function RecordMarks({
  record,
  className,
}: {
  record: SymbolRecord;
  className?: string;
}) {
  const marks: Array<{ key: string; icon: React.ReactNode; title: string; text?: string }> = [];
  if (record.held) {
    marks.push({ key: "held", icon: <Wallet className="size-3" />, title: "보유 중인 종목" });
  }
  if (record.target) {
    marks.push({
      key: "target",
      icon: <Target className="size-3" />,
      title: `목표가 ${formatPrice(record.target.targetPrice, record.target.currency)}${
        record.target.note ? ` · ${record.target.note}` : ""
      }`,
    });
  }
  if (record.notes > 0) {
    marks.push({
      key: "notes",
      icon: <NotepadText className="size-3" />,
      title: `메모 ${record.notes}건`,
      text: record.notes > 1 ? String(record.notes) : undefined,
    });
  }
  if (record.stamps > 0) {
    marks.push({
      key: "stamps",
      icon: <Stamp className="size-3" />,
      title: `스탬프 ${record.stamps}개`,
      text: record.stamps > 1 ? String(record.stamps) : undefined,
    });
  }
  if (marks.length === 0) return null;

  return (
    <span className={cn("flex shrink-0 items-center gap-1 text-[var(--text-muted)]", className)}>
      {marks.map((m) => (
        <span key={m.key} title={m.title} className="flex items-center gap-[1px]">
          {m.icon}
          {m.text && <span className="font-num text-micro leading-none">{m.text}</span>}
        </span>
      ))}
    </span>
  );
}

/**
 * 최근 흐름 한 줄. **화면에 들어올 때만** 받는다(`useInView`) — 관심종목이 서른 개면
 * 카드마다 일봉을 부르는 것이 그대로 서른 요청이다. 랭킹 목록이 쓰는 것과 같은 부품이다.
 */
export function WlSpark({
  symbol,
  changeRate,
  className,
}: {
  symbol: string;
  changeRate: number;
  className?: string;
}) {
  const { ref, inView } = useInView("160px");
  return (
    <div ref={ref} className={cn("flex items-center justify-end", className)}>
      {inView ? <SparkInner symbol={symbol} changeRate={changeRate} /> : null}
    </div>
  );
}

function SparkInner({ symbol, changeRate }: { symbol: string; changeRate: number }) {
  const { points } = useSparklineCandles(symbol, true);
  if (points.length < 2) return null;
  return <IndexSparkline points={points} changeRate={changeRate} className="max-w-none" />;
}

/** 종목 이름 한 칸 — 아바타·이름·티커·내 기록 표식. 넷이 같은 것을 쓴다. */
export function SymbolLabel({
  row,
  record,
  size = "sm",
  sub,
  className,
}: {
  row: WatchlistRow;
  record: SymbolRecord;
  size?: "sm" | "md";
  /** 티커 오른쪽에 붙는 한 조각. 시안마다 여기에 두는 값이 다르다. */
  sub?: React.ReactNode;
  className?: string;
}) {
  const q = row.quote;
  return (
    <span className={cn("flex min-w-0 items-center gap-2", className)}>
      <StockAvatar
        symbol={row.symbol}
        name={q?.name}
        currency={q?.currency}
        assetType={q?.assetType}
        logoUrl={q?.logoUrl}
        market={q?.market}
        size="xs"
        className={cn("shrink-0", size === "md" && "h-8 w-8")}
      />
      <span className="min-w-0 flex-1">
        <span className="flex min-w-0 items-center gap-1.5">
          <span
            className={cn(
              "truncate font-semibold text-[var(--text-primary)]",
              size === "md" ? "text-section" : "text-note",
            )}
          >
            {q?.name || row.symbol}
          </span>
          <RecordMarks record={record} />
        </span>
        {/*
          좁으면 **티커가 먼저 사라진다.** 375px에서 셋(티커·담은 뒤·이름)이 한 칸을
          다투자 「담은 뒤 -21.20%」가 「담은 뒤 -…」로 잘렸다 — 이 화면의 존재 이유를
          지우고 종목 번호를 남긴 셈이다. 이름은 바로 윗줄에 있고, 티커는 퀵룩에도 있다.
        */}
        <span className="flex min-w-0 items-center gap-1.5 font-num text-micro leading-[14px] text-[var(--text-muted)]">
          <span className="hidden truncate @md:inline">{row.symbol}</span>
          {sub}
        </span>
      </span>
    </span>
  );
}

/** 현재가 + 등락. 목록 화면과 같은 값 플래시를 쓴다. */
export function PriceBlock({
  quote,
  size = "sm",
  align = "right",
  className,
}: {
  quote: QuoteSummary | undefined;
  size?: "sm" | "md";
  align?: "left" | "right";
  className?: string;
}) {
  if (!quote || quote.currentPrice <= 0) {
    return <span className={cn("text-note text-[var(--text-muted)]", className)}>—</span>;
  }
  return (
    <span className={cn("block", align === "right" ? "text-right" : "text-left", className)}>
      <span
        className={cn(
          /* 금액은 절대 접지 않는다 — 375px에서 「1,787,000원」의 「원」이 아랫줄로
             넘어가 행이 한 줄 더 높아졌다(실측). */
          "block whitespace-nowrap font-num font-bold tabular-nums text-[var(--text-primary)]",
          size === "md" ? "text-page" : "text-section",
        )}
      >
        <ValueFlash value={quote.currentPrice} resetKey={quote.symbol}>
          {formatPrice(quote.currentPrice, quote.currency)}
        </ValueFlash>
      </span>
      <span
        className={cn(
          "block font-num text-cell font-semibold tabular-nums",
          trendTextClass(quote.changeRate),
        )}
      >
        {formatPercent(quote.changeRate)}
        {/*
          변화액은 **넓을 때만**. 375px에서는 「+4.86% +12,500원」이 두 줄로 넘쳐
          행 높이를 한 줄 더 먹었다(실측). 등락률만 남겨도 방향과 크기는 그대로 읽힌다.
        */}
        <span className="ml-1 hidden opacity-80 @xl:inline">
          {formatPriceChange(quote.change, quote.currency)}
        </span>
      </span>
    </span>
  );
}

/** 점수 배지 — 목록 행이 쓰던 것과 같은 톤. */
export function ScoreBadge({
  score,
  grade,
  className,
}: {
  score: number | null;
  grade?: string;
  className?: string;
}) {
  if (score == null) {
    return (
      <span
        className={cn("font-num text-cell text-[var(--text-muted)]", className)}
        title="산출된 점수 없음"
      >
        —
      </span>
    );
  }
  return (
    <span
      className={cn(
        "inline-flex items-center gap-0.5 rounded-md px-1.5 py-[1px] font-num text-cell font-bold tabular-nums",
        SCORE_TONE_BADGE[scoreTone(score)],
        className,
      )}
      title={`종합 점수 ${Math.round(score)}${grade ? ` (${grade})` : ""} — 5축 가중합이며 매매 신호가 아닙니다`}
    >
      {Math.round(score)}
      {grade && <span className="text-micro opacity-80">{grade}</span>}
    </span>
  );
}

/**
 * 내가 적어 둔 목표가와 지금 가격의 거리 한 줄.
 *
 * 목표가 화면(`/targets`)이 이미 진행축으로 크게 그리고 있으므로 여기서는 **한 줄**만
 * 말한다 — 관심 목록에서 알고 싶은 것은 "얼마나 남았나"지 궤적 전체가 아니다.
 */
export function TargetLine({
  target,
  price,
  className,
}: {
  target: PriceTarget;
  price: number;
  className?: string;
}) {
  const away = price > 0 ? ((target.targetPrice - price) / price) * 100 : null;
  return (
    <span
      className={cn("truncate text-cell text-[var(--text-muted)]", className)}
      title={target.note || undefined}
    >
      목표{" "}
      <b className="font-num font-semibold tabular-nums text-[var(--text-secondary)]">
        {formatPrice(target.targetPrice, target.currency)}
      </b>
      {away != null && (
        <span className={cn("ml-1 font-num tabular-nums", away <= 0 && "text-[var(--brand)]")}>
          {away <= 0 ? "도달" : `${away.toFixed(1)}% 남음`}
        </span>
      )}
    </span>
  );
}

/** 담은 뒤 한 칸 — 이 화면의 존재 이유라 넷 모두에서 같은 모양으로 나온다. */
export function SinceAdded({ row, className }: { row: WatchlistRow; className?: string }) {
  if (row.sinceAddedRate == null) {
    return (
      <span
        className={cn("text-cell text-[var(--text-muted)]", className)}
        title="담은 가격이 기록되기 전에 추가된 종목입니다"
      >
        담은 뒤 —
      </span>
    );
  }
  return (
    <span className={cn("whitespace-nowrap text-cell text-[var(--text-muted)]", className)}>
      담은 뒤{" "}
      <b className={cn("font-num font-semibold tabular-nums", trendTextClass(row.sinceAddedRate))}>
        {formatPercent(row.sinceAddedRate)}
      </b>
    </span>
  );
}
