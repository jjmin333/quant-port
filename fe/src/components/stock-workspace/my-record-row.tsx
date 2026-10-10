"use client";

import Link from "next/link";

import {
  formatMoney,
  formatQuantity,
  hasAnyRecord,
  verdictLabel,
  verdictTone,
  type SymbolContext,
} from "@/lib/symbol-context-api";
import { COMMUNITY_FEATURES_HIDDEN, PREDICTION_FEATURES_HIDDEN } from "@/lib/hidden-features";
import { cn } from "@/lib/utils";

/**
 * 「이 종목에 대한 내 기록」 한 줄 — 흐름을 닫는 자리 (2026-09-20 사용자 결정 가).
 *
 * # 무엇을 고치는가
 *
 * 종목 화면이 사용자에 대해 아는 것은 `quote.isHeld` **한 비트뿐**이었습니다. 내가
 * 관심에 담았는지, 스탬프를 찍었는지, 목표가를 적었는지, 리포트를 썼는지, 보유 이유
 * 점검이 뭐라 했는지 — 아무것도 몰랐습니다.
 *
 * # 칩 하나가 문 하나입니다
 *
 * 발견 → 근거 → 판단 → 반영 → 추적을 오가는 길이고, 그 길이 없어서 흐름이 끊겨
 * 있었습니다. 특히 **추적 → 근거**로 되돌아갈 방법이 아예 없었습니다.
 *
 * # 행동 버튼이 아닙니다
 *
 * 2026-09-03에 헤더 액션을 여섯에서 셋으로 줄이신 적이 있어 같은 영역이 붐비는 것에
 * 주의했습니다. 이 줄은 **상태 표시**이고, 누르면 그 기록이 있는 곳으로 갈 뿐
 * 새 창을 열거나 무언가를 만들지 않습니다.
 *
 * # 없는 것도 남깁니다
 *
 * 관심에 안 담았으면 점선으로 그립니다 — 「아직 안 했다」도 흐름의 한 칸이고 다음에
 * 할 일을 가리킵니다. 다만 **전부 비어 있으면 줄 자체를 안 그립니다**: 점선 칩
 * 여섯 개는 안내가 아니라 잡음입니다.
 */
export function MyRecordRow({
  context,
  className,
}: {
  context: SymbolContext | null;
  className?: string;
}) {
  if (!hasAnyRecord(context) || !context) return null;

  const verdict = verdictLabel(context.diagnosisVerdict);

  return (
    <div
      className={cn(
        "flex items-center gap-1.5 overflow-x-auto px-3 py-1.5",
        "border-b border-[var(--border)] bg-[var(--card)]",
        className,
      )}
    >
      <span className="shrink-0 text-micro text-[var(--text-muted)]">내 기록</span>

      {/*
        **확인이 필요한 것이 먼저**입니다. 좁은 화면에서 칩이 가로로 흐르므로
        순서가 곧 우선순위이고, 「약화」는 스크롤 없이 보여야 합니다.
      */}
      {verdict ? (
        <Chip href="/holdings" tone={verdictTone(context.diagnosisVerdict)}>
          점검 {verdict}
          {context.diagnosisSignals ? (
            <span className="ml-1 font-normal opacity-80">근거 {context.diagnosisSignals}</span>
          ) : null}
        </Chip>
      ) : null}

      {context.held ? (
        <Chip href="/holdings" tone="on">
          보유 {formatQuantity(context.quantity)}주
          {context.avgPrice != null ? (
            <span className="ml-1 font-normal opacity-80">
              @{formatMoney(context.avgPrice, context.currency)}
            </span>
          ) : null}
        </Chip>
      ) : null}

      {/* 목표가·스탬프·리포트는 비노출 기능이면 칩도 뺀다(lib/hidden-features). */}
      {context.targetPrice != null && !PREDICTION_FEATURES_HIDDEN ? (
        <Chip href="/predictions" tone="on">
          목표 {formatMoney(context.targetPrice, context.currency)}
        </Chip>
      ) : null}

      {context.stampCount > 0 && !PREDICTION_FEATURES_HIDDEN ? (
        <Chip href="/stamps" tone="on">
          스탬프 {context.stampCount}
        </Chip>
      ) : null}

      {context.reportCount > 0 && !COMMUNITY_FEATURES_HIDDEN ? (
        <Chip href="/reports" tone="on">
          리포트 {context.reportCount}
        </Chip>
      ) : null}

      {/* 없는 것은 점선으로 — 지우지 않습니다. */}
      {!context.watchlisted ? <Chip tone="dim">관심 미등록</Chip> : (
        <Chip href="/watchlist" tone="on">
          관심
        </Chip>
      )}
    </div>
  );
}

function Chip({
  children,
  href,
  tone,
}: {
  children: React.ReactNode;
  href?: string;
  tone: "on" | "warn" | "dim";
}) {
  const cls = cn(
    "shrink-0 whitespace-nowrap rounded-full border px-2 py-0.5 text-micro",
    tone === "on" && "border-[var(--brand)] bg-[var(--brand)]/10 font-bold text-[var(--brand)]",
    tone === "warn" &&
      "border-[var(--score-low)] bg-[var(--score-low)]/10 font-bold text-[var(--score-low)]",
    tone === "dim" && "border-dashed border-[var(--border)] text-[var(--text-muted)]",
  );

  if (!href) return <span className={cls}>{children}</span>;
  return (
    <Link href={href} className={cn(cls, "hover:opacity-80")}>
      {children}
      <span aria-hidden className="ml-0.5">
        ›
      </span>
    </Link>
  );
}
