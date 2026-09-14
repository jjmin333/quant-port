"use client";

import { cn } from "@/lib/utils";

/*
목록 위 필터 줄의 **부품**. 실시간 차트(마켓 레이더)와 AI 추천 랭킹이 같은 것을 쓴다.

# 왜 부품으로 뺐나 (2026-09-03 사용자 지적)

두 화면을 같은 2단 배치로 맞춘 뒤에도 **왼쪽 목록의 필터 줄만 달랐다.** 한쪽은 탭 +
세그먼트 + 칩이고, 다른 쪽은 카드 머리줄에 작은 세그먼트 두 개였다 — 글꼴 크기도
버튼 모양도 서로 달랐다. 「같게 만들라」는 요구를 클래스 문자열 복사로 맞추면 다음에
한쪽만 고쳐지면서 곧바로 다시 벌어진다.

그래서 **모양을 이 파일이 혼자 쥔다.** 두 화면은 무엇을 고르는지(시장 · 정렬 기준 ·
기간)만 넘기고, 어떻게 생겼는지는 여기서 한 번만 정한다.
*/

export type RadarOption<T extends string> = { id: T; label: string };

/**
 * 국내 / 해외 — **필터가 아니라 서로 다른 목록**이라 탭으로 선다.
 * (실시간 차트 툴바가 이 판단을 먼저 했고, 랭킹도 같은 두 목록이다.)
 */
export function RadarMarketTabs<T extends string>({
  options,
  value,
  onChange,
  compact,
  label = "시장",
}: {
  options: readonly RadarOption<T>[];
  value: T;
  onChange: (id: T) => void;
  compact?: boolean;
  label?: string;
}) {
  return (
    <div
      role="tablist"
      aria-label={label}
      className={cn("flex gap-4 border-b border-[var(--border)]", compact ? "px-3" : "px-2")}
    >
      {options.map((tab) => (
        <button
          key={tab.id}
          type="button"
          role="tab"
          aria-selected={value === tab.id}
          onClick={() => onChange(tab.id)}
          className={cn(
            "-mb-px border-b-2 py-2 text-note font-semibold transition-colors",
            "focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[var(--brand)]",
            value === tab.id
              ? "border-[var(--text-primary)] text-[var(--text-primary)]"
              : "border-transparent text-[var(--text-muted)] hover:text-[var(--text-secondary)]",
          )}
        >
          {tab.label}
        </button>
      ))}
    </div>
  );
}

/**
 * 정렬 기준 — 테두리 한 판 안의 세그먼트.
 *
 * 마켓 레이더는 둘(거래대금·거래량), AI 랭킹은 여섯(종합 + 5축)이다. 좁은 레일에서
 * 여섯은 한 줄에 안 들어가므로 **가로로 스크롤한다** — 줄바꿈하면 그 아래 기간 칩이
 * 밀려 내려가 목록의 첫 줄이 화면 밖으로 나간다.
 */
export function RadarSegmented<T extends string>({
  options,
  value,
  onChange,
  label,
  className,
}: {
  options: readonly RadarOption<T>[];
  value: T;
  onChange: (id: T) => void;
  label: string;
  className?: string;
}) {
  return (
    <div
      role="tablist"
      aria-label={label}
      className={cn(
        // `w-fit`이 **테두리를 버튼 끝에서 끊는다** (2026-09-04 사용자 지적).
        // 이 판은 블록 요소라 기본 폭이 100%였고, AI 추천 랭킹처럼 세그먼트가
        // 한 줄을 혼자 쓰는 툴바에서는 여섯 번째 버튼(「심리·수급」) 오른쪽으로
        // 빈 테두리가 길게 남았다. 실시간 차트 쪽은 flex 행 안이라 이미
        // 내용 크기였으므로 이 줄로 두 화면이 같아진다.
        // `max-w-full`이 있어야 좁은 레일에서 넘칠 때 아래 가로 스크롤이 산다.
        "flex w-fit min-w-0 max-w-full shrink rounded-lg border border-[var(--border)] p-0.5",
        "overflow-x-auto [-ms-overflow-style:none] [scrollbar-width:none] [&::-webkit-scrollbar]:hidden",
        className,
      )}
    >
      {options.map((o) => (
        <button
          key={o.id}
          type="button"
          role="tab"
          onClick={() => onChange(o.id)}
          /*
            선택 상태는 `aria-selected` 하나로 말한다. 예전에는 `aria-pressed`를
            같이 달았는데, 그 속성은 `role="tab"`에 **허용되지 않아** 보조기술이
            상태를 아예 못 읽었다(axe `aria-allowed-attr`, critical). 「눌림」과
            「선택됨」이라는 두 모형을 동시에 주장한 셈이고, 이 판은 위의
            `role="tablist"`가 이미 탭이라고 선언했다.
          */
          aria-selected={value === o.id}
          className={cn(
            "shrink-0 whitespace-nowrap rounded-md px-2.5 py-1 text-cell font-medium transition-colors",
            "focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-[var(--brand)]",
            value === o.id
              ? "bg-[var(--brand-muted)] font-semibold text-[var(--brand)]"
              : "text-[var(--text-muted)] hover:text-[var(--text-secondary)]",
          )}
        >
          {o.label}
        </button>
      ))}
    </div>
  );
}

/** 기간 — 알약 칩. 좁은 레일에서는 가로로 스크롤한다. */
export function RadarChips<T extends string>({
  options,
  value,
  onChange,
  label,
}: {
  options: readonly RadarOption<T>[];
  value: T;
  onChange: (id: T) => void;
  label: string;
}) {
  return (
    <div
      role="tablist"
      aria-label={label}
      className="flex gap-1.5 overflow-x-auto pb-0.5 [-ms-overflow-style:none] [scrollbar-width:none] [&::-webkit-scrollbar]:hidden"
    >
      {options.map((o) => (
        <button
          key={o.id}
          type="button"
          role="tab"
          aria-selected={value === o.id}
          onClick={() => onChange(o.id)}
          className={cn(
            "shrink-0 whitespace-nowrap rounded-full border px-2.5 py-1 text-cell font-medium transition-colors",
            "focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-[var(--brand)]",
            value === o.id
              ? "border-[color-mix(in_oklab,var(--brand)_40%,transparent)] bg-[var(--brand-muted)] font-semibold text-[var(--brand)]"
              : "border-[var(--border)] text-[var(--text-muted)] hover:text-[var(--text-secondary)]",
          )}
        >
          {o.label}
        </button>
      ))}
    </div>
  );
}

/** 툴바 한 판의 바깥 틀 — 두 화면의 여백·경계선을 같게 붙든다. */
export function RadarToolbarShell({
  compact,
  tabs,
  children,
}: {
  compact?: boolean;
  tabs: React.ReactNode;
  children: React.ReactNode;
}) {
  return (
    <div className="min-w-0 shrink-0 border-b border-[var(--border)] bg-[var(--surface)]">
      {tabs}
      <div className={cn("space-y-2", compact ? "px-3 py-2.5" : "px-2 py-2")}>{children}</div>
    </div>
  );
}
