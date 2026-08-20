"use client";

import { useState } from "react";
import Link from "next/link";
import { HelpCircle } from "lucide-react";
import { analysisSymbolHref } from "@/lib/analysis-tabs";
import {
  SCORE_AXIS_LABELS,
  SCORE_AXIS_ORDER,
  SCORE_TONE_BADGE,
  scoreTone,
} from "@/lib/stock-score-api";
import { cn } from "@/lib/utils";

/**
 * 「종합점수」 배지 — 카드 우상단에 붙는다.
 *
 * 예전에는 숫자 하나(`30`)만 색 배지로 떠 있었다. 그 앱을 처음 여는 사람에게 그건
 * 아무것도 아니다 — 등수인지, 기사 수인지, 목표가인지 알 방법이 없었다(`title`
 * 속성에만 "종합 점수"라고 적혀 있었는데, 그건 마우스를 올려야 보이고 모바일에는
 * 아예 없다).
 *
 * 그래서 세 가지를 바꾼다 —
 *  1. **이름을 화면에 적는다**(`종합점수 30`). 라벨 없는 숫자는 데이터가 아니다.
 *  2. **100점 만점임을 배지 안에 적는다**(`30`+`/100`) — 30이 낮은 값이라는 걸
 *     기준 없이는 알 수 없다.
 *  3. **ⓘ를 눌러 무엇으로 만든 점수인지** 축 이름까지 펼쳐 보여주고, 매매 신호가
 *     아니라고 못 박는다. 점수는 5축 가중합이라 한 줄 설명으로는 오해를 부른다.
 *
 * 앱 어디서든 같은 배지를 쓴다 — 뉴스 모달·퀵룩·랭킹이 각자 다른 모양이면 같은
 * 숫자를 매번 새로 배워야 한다.
 */
export function StockScoreChip({
  symbol,
  score,
  grade,
  tradeDate,
  className,
  onNavigate,
}: {
  symbol: string;
  score: number | null | undefined;
  grade?: string;
  tradeDate?: string;
  className?: string;
  /** 링크를 눌러 이동할 때 모달 등을 닫을 훅. */
  onNavigate?: () => void;
}) {
  const [open, setOpen] = useState(false);
  if (score == null) return null;
  const tone = scoreTone(score);

  return (
    <span className={cn("relative shrink-0", className)}>
      <button
        type="button"
        onClick={(e) => {
          e.stopPropagation();
          setOpen((v) => !v);
        }}
        aria-expanded={open}
        aria-label="종합점수 설명 보기"
        className={cn(
          "inline-flex items-center gap-1 rounded-md py-0.5 pl-1.5 pr-1 transition-opacity hover:opacity-80",
          SCORE_TONE_BADGE[tone],
        )}
      >
        <span className="text-[10px] font-semibold">종합점수</span>
        <span className="font-mono text-[12px] font-bold tabular-nums">{Math.round(score)}</span>
        <span className="font-mono text-[9px] opacity-70">/100</span>
        {grade && <span className="text-[10px] font-bold">{grade}</span>}
        <HelpCircle className="size-3 opacity-70" />
      </button>

      {open && (
        <>
          {/* 바깥 아무 데나 누르면 닫힌다 — 모달 안이라 ESC는 모달이 먼저 먹는다. */}
          <button
            type="button"
            aria-label="닫기"
            className="fixed inset-0 z-20 cursor-default"
            onClick={(e) => {
              e.stopPropagation();
              setOpen(false);
            }}
          />
          <span
            className="absolute right-0 top-full z-30 mt-1.5 block w-[248px] rounded-xl border border-[var(--border)] bg-[var(--surface-elevated)] p-3 text-left shadow-xl"
            onClick={(e) => e.stopPropagation()}
          >
            <span className="block text-xs font-bold text-[var(--text-primary)]">
              종합점수란?
            </span>
            <span className="mt-1 block text-[11px] leading-relaxed text-[var(--text-secondary)]">
              아래 5개 축을 <b className="text-[var(--text-primary)]">0~100점</b>으로 환산해 합친
              자체 점수입니다. 높을수록 지표가 양호하다는 뜻이며,{" "}
              <b className="text-[var(--text-primary)]">매매 신호가 아닙니다.</b>
            </span>
            <span className="mt-1.5 flex flex-wrap gap-1">
              {SCORE_AXIS_ORDER.map((axis) => (
                <span
                  key={axis}
                  className="rounded border border-[var(--border)] px-1.5 py-px text-[10px] text-[var(--text-muted)]"
                >
                  {SCORE_AXIS_LABELS[axis]}
                </span>
              ))}
            </span>
            {tradeDate && (
              <span className="mt-1.5 block text-[10px] text-[var(--text-muted)]">
                {tradeDate} 기준
              </span>
            )}
            <Link
              href={analysisSymbolHref(symbol, "ai")}
              onClick={onNavigate}
              className="mt-2 block rounded-lg bg-[var(--brand)] px-2 py-1.5 text-center text-[11px] font-semibold text-white hover:opacity-90"
            >
              축별 점수 자세히 보기
            </Link>
          </span>
        </>
      )}
    </span>
  );
}
