"use client";

import { useState } from "react";
import { ChevronDown, PanelRight } from "lucide-react";
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import { useIsRailColumn, useRailLayout } from "@/lib/layout-lab";
import { cn } from "@/lib/utils";

/**
 * 좁은 화면에서 **레일을 어디에 세울지**를 실험대 값에 따라 갈라 준다
 * (`lib/layout-lab` 머리말 · 2026-09-06 사용자 요청 4~9 2차).
 *
 * 실서비스는 언제나 `stack`이라 이 파일의 분기를 타지 않는다 — `PageColumns`가
 * `stack`일 때는 예전 그대로의 `<aside>`를 그리고 여기까지 오지 않는다.
 *
 * 레일 **내용은 손대지 않는다.** 카드 하나도 지우거나 줄이지 않고, 접힘을 열거나
 * 시트를 열면 지금과 완전히 같은 것이 나온다. 바꾸는 것은 그것이 첫 화면의 세로
 * 예산을 얼마나 먹느냐뿐이다.
 */
/**
 * `PageColumns`의 오른쪽 레일 기둥 — 실험대 값을 읽는 자리라 클라이언트다.
 *
 * `stack`(=실서비스 기본)일 때 그리는 것은 **예전 `RailColumn sticky`와 같은
 * 마크업**이다. 실험대 밖에서 이 컴포넌트가 화면을 바꾸는 일은 없다.
 *
 * `sheet`에서만 격자 순서를 뒤집는다(`max-xl:order-first`) — 「관련 정보」 줄이
 * 목록 **위**에 서야 첫 화면에서 한 번에 닿는다. 지금 문제의 절반이 「이 화면의 주
 * 동작(종목 추가·메모 검색·목표가 추가)이 목록 전체 아래에 있다」는 것이었다.
 */
export function LabRailColumn({ children }: { children: React.ReactNode }) {
  const layout = useRailLayout();
  const isColumn = useIsRailColumn();
  return (
    <aside
      className={cn(
        "flex min-w-0 flex-col gap-3",
        "xl:sticky xl:top-3 xl:max-h-[calc(100dvh-5.5rem)] xl:overflow-y-auto xl:overscroll-contain",
        layout === "sheet" && !isColumn && "order-first",
      )}
    >
      <NarrowRail>{children}</NarrowRail>
    </aside>
  );
}

function NarrowRail({ children }: { children: React.ReactNode }) {
  const layout = useRailLayout();
  const isColumn = useIsRailColumn();
  const [open, setOpen] = useState(false);

  // 기둥으로 설 수 있는 폭이면 지금과 같다 — 사용자가 PC 배치는 만족한다고 했다.
  if (isColumn || layout === "stack") return <>{children}</>;

  if (layout === "fold") {
    return <RailFold>{children}</RailFold>;
  }

  return (
    <>
      <button
        type="button"
        onClick={() => setOpen(true)}
        className="flex w-full items-center justify-between gap-2 rounded-xl border border-[var(--border)] bg-[var(--surface)] px-3 py-2.5 text-left"
      >
        <span className="flex items-center gap-1.5 text-cell font-semibold text-[var(--text-primary)]">
          <PanelRight className="h-3.5 w-3.5 text-[var(--text-muted)]" aria-hidden />
          관련 정보
        </span>
        <span className="text-micro text-[var(--text-muted)]">열기</span>
      </button>
      <Sheet open={open} onOpenChange={setOpen}>
        {/*
          시트는 **아래에서 올라오고 화면의 92%를 쓴다.** 오른쪽에서 나오는 기본
          방향은 폰에서 폭이 3/4로 잘려, 레일을 넓게 보려고 연 것이 다시 좁은 칸이
          된다. 안쪽은 그대로 세로 스크롤이라 카드 순서도 지금과 같다.
        */}
        <SheetContent
          side="bottom"
          className="max-h-[92dvh] rounded-t-2xl p-0 pt-3"
        >
          <SheetHeader className="px-4 pb-2 pt-0">
            <SheetTitle className="text-section">관련 정보</SheetTitle>
            <SheetDescription className="text-cell">
              목록 옆에 서던 카드들입니다. 넓은 화면에서는 오른쪽 기둥에 그대로 있습니다.
            </SheetDescription>
          </SheetHeader>
          <div className="flex min-h-0 flex-1 flex-col gap-3 overflow-y-auto px-4 pb-6">
            {children}
          </div>
        </SheetContent>
      </Sheet>
    </>
  );
}

/**
 * 접힌 한 칸. 기본은 **닫힘**이다 — 열어 두면 지금과 같아져 비교할 것이 없어진다.
 *
 * 「N개」를 세지 않는다. 레일은 `<EarningsSideRail />` 같은 컴포넌트 **하나**로
 * 넘어오므로 카드 수를 알 수 없고, 모르는 수를 지어내면 그 자체가 거짓말이다.
 */
function RailFold({ children }: { children: React.ReactNode }) {
  const [open, setOpen] = useState(false);
  return (
    <section className="rounded-xl border border-[var(--border)] bg-[var(--surface)]">
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        aria-expanded={open}
        className="flex w-full items-center justify-between gap-2 px-3 py-2.5 text-left"
      >
        <span className="flex items-center gap-1.5 text-cell font-semibold text-[var(--text-primary)]">
          <PanelRight className="h-3.5 w-3.5 text-[var(--text-muted)]" aria-hidden />
          관련 정보
        </span>
        <ChevronDown
          className={cn(
            "h-4 w-4 shrink-0 text-[var(--text-muted)] transition-transform",
            open && "rotate-180",
          )}
        />
      </button>
      {open && (
        <div className="flex flex-col gap-3 border-t border-[var(--border)] p-3">{children}</div>
      )}
    </section>
  );
}
