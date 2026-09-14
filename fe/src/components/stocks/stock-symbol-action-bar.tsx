"use client";

import { useState } from "react";
import { Briefcase, ChevronsRight, MoreHorizontal, Target } from "lucide-react";
import { AddHoldingDialog } from "@/components/holdings/add-holding-dialog";
import { StampButton } from "@/components/stamps/stamp-button";
import { TargetPriceDialog } from "@/components/targets/target-price-dialog";
import { Button } from "@/components/ui/button";
import type { StockInfo } from "@/lib/quotes-types";
import { cn } from "@/lib/utils";
import { useLoginGate } from "@/hooks/use-login-gate";

/**
 * 이 줄의 버튼들이 공유하는 **유일한** 색·글자 크기 지정.
 *
 * 높이·모서리·아이콘 크기는 전부 `<Button variant="outline" size="sm">`이 정한다.
 *
 * 글자만 한 단 내렸다(12 → 11px, `text-micro`). 이 줄은 헤더의 보조 동작이고,
 * 바로 왼쪽에 종목명·현재가라는 진짜 주인공이 있다 — 셋이 12px로 서 있으면 좁은
 * 화면에서 종목명보다 이 줄이 먼저 눈에 들어온다. 아이콘 크기는 `size="sm"`이
 * 정한 13px 그대로 둔다(button.tsx의 계단 주석 참조).
 */
export const HEADER_ACTION_CLASS =
  "text-micro text-[var(--text-secondary)] hover:text-[var(--text-primary)]";

type StockSymbolActionBarProps = {
  /** 그릴 종목. **없을 수도 있는 값으로 받는다** — 아래 「빈 값에 죽지 않는다」 참조. */
  stock: StockInfo | null | undefined;
  /** Chart interval on screen — frozen into a stamp when one is taken. */
  chartInterval?: string;
  price?: number;
  week52High?: number;
  week52Low?: number;
  /**
   * 모바일에서 **⋯ 하나로 접는다**. 종목 헤더가 그렇게 쓴다 — 헤더 한 줄에
   * 이름·가격과 나란히 서야 해서 버튼 셋이 들어갈 자리가 없다.
   */
  collapseOnMobile?: boolean;
  /**
   * 접힌 것을 폈는가. **부르는 쪽이 들고 있다** — 펴면 헤더 배치까지 두 줄로
   * 바뀌기 때문에(아래 「⋯ 는 접힘일 뿐이다」) 이 줄만으로는 결정할 수 없다.
   */
  expanded?: boolean;
  onExpandedChange?: (expanded: boolean) => void;
  className?: string;
};

/**
 * 종목 헤더 우측 액션 — **스탬프 생성 · 보유종목 등록 · 목표가 설정** 셋.
 *
 * # 왜 셋인가 (2026-09-03 사용자 결정)
 *
 * 예전에는 여섯 개였다: 관심 · 메모장 · 스탬프 · 보유 · 리포트 작성 · 목표가.
 * 그중 **관심과 메모는 랭킹 목록의 행마다 이미 붙어 있다** — 종목을 고르는 자리에서
 * 누르는 동작이고, 상세로 들어와서까지 같은 버튼을 볼 이유가 없다. 리포트 작성은
 * 이 줄에서 가장 무거운 동작인데 가장 드물게 쓰여서, 자주 쓰는 셋의 자리를 좁히는
 * 값만 하고 있었다(리포트는 `/reports/new`가 제 자리다).
 *
 * 남은 셋은 전부 **이 종목에 무언가를 붙이는** 동작이라 성격이 같다.
 *
 * # 셋 다 그 자리에서 끝난다 (2026-09-03)
 *
 * 예전에는 목표가만 `/targets`로 화면을 떠났다 — 옆의 스탬프는 모달로 끝나는데
 * 혼자 달랐고, 목표가 하나 적자고 보던 차트를 잃었다. 보유 등록은 열리긴 했지만
 * **종목 검색부터** 시작했다: 방금 보고 있던 종목을 다시 찾아 골라야 했다.
 * 지금은 둘 다 이 종목·이 가격이 채워진 채로 열린다.
 *
 * # ⋯ 는 **접힘일 뿐이다** (2026-09-03 사용자 결정)
 *
 * 글자를 단 버튼 셋이 좁은 화면에서 **줄 하나를 통째로** 썼다(실측 390px에서 헤더
 * 134px 중 36px). 그래서 ⋯ 하나로 접어 뒀는데, 처음에는 그것을 누르면 **바텀시트**가
 * 열렸다 — 화면을 덮고, 고르면 닫히고, 다시 쓰려면 또 열어야 했다. 자주 쓰는 셋을
 * 남기자던 결정과 어긋나는 동작이다(`docs/mockups/stock-header-mobile-v1.html`의
 * B안 설명이 그 값을 미리 적어 두었다).
 *
 * 지금은 **그 자리에서 편다**: ⋯ 를 누르면 같은 줄에 아이콘 셋이 서고(같은 목업의
 * A안 「아이콘만 · 두 줄」), 헤더는 이름 줄과 가격 줄 두 줄이 된다. 다시 접는 것은
 * 셋 오른쪽의 ⟫ 버튼이다 — 편 상태가 유지되므로 스탬프를 찍고 이어서 목표가를
 * 적는 것이 두 번 다 한 번의 탭이다.
 */
export function StockSymbolActionBar({
  stock,
  chartInterval,
  price,
  week52High,
  week52Low,
  collapseOnMobile = false,
  expanded = false,
  onExpandedChange,
  className,
}: StockSymbolActionBarProps) {
  const [addOpen, setAddOpen] = useState(false);
  const [targetOpen, setTargetOpen] = useState(false);
  /*
    보유 등록·목표가 창이 쓸 가격은 **버튼을 누른 순간에 얼린다.**

    시세는 5초마다 갱신되므로, 창을 띄워 둔 채로 두면 사용자가 보고 적기 시작한
    숫자가 발밑에서 바뀐다. 스탬프 버튼이 이미 같은 이유로 같은 일을 한다.
  */
  const [frozenPrice, setFrozenPrice] = useState<number | undefined>(undefined);

  /*
    비로그인에게도 **창은 열린다** (2026-09-05 사용자 요청 4).

    09-04에는 이 줄에서 막고 곧장 로그인으로 보냈다. 그러면 401이 날 요청은 확실히
    없지만, 누른 사람은 **자기가 무엇을 놓치고 있는지 보지 못한 채** 로그인 화면으로
    튕긴다 — 스탬프 창이 예상 주가·검증 기간을 받는다는 것도, 보유 등록이 수량·평단을
    받는다는 것도 로그인해야만 알 수 있었다.

    지금은 로그인한 사람과 **같은 창이 같은 모양으로** 열리고, 막는 자리는 창 안의
    저장 버튼 하나다(거기서 `disabled`라 요청이 만들어지지 않는다). 「원천차단」은
    그대로다 — 막는 위치만 화면 앞에서 화면 안으로 옮겼다.

    # 이름은 **하는 일**이다 (2026-09-05 사용자 결정)

    한동안 이 셋은 비로그인일 때 「로그인하고 스탬프」·「로그인하고 보유 등록」·
    「로그인하고 목표가 설정」이었다. 지금은 로그인 여부와 상관없이 **「스탬프 생성」·
    「보유종목 등록」·「목표가 설정」**이다 — 눌렀을 때 하는 일(창을 연다)이 둘 다
    같아졌으므로 이름이 갈라질 이유가 없고, 셋이 나란히 선 줄에서 앞의 다섯 글자가
    똑같으면 무엇이 무엇인지 읽는 데 시간이 든다.

    로그인이 필요하다는 말은 **그것이 실제로 걸리는 자리**에서 한다 — 창 안의 저장
    버튼과 그 아래 한 줄이다. 여기 `gate`는 마우스를 올렸을 때의 툴팁에만 남는다.
  */
  const gate = useLoginGate("등록");

  const openHolding = () => {
    setFrozenPrice(price);
    setAddOpen(true);
  };
  const openTarget = () => {
    setFrozenPrice(price);
    setTargetOpen(true);
  };

  /*
    # 빈 값에 죽지 않는다 (2026-09-03)

    17:39, `/market`이 이 컴포넌트에서 통째로 내려앉았다 —
    `Cannot read properties of undefined (reading 'symbol')`. 부르는 쪽과 이쪽의
    prop 모양이 어긋난 순간(리팩터링 중이었거나 브라우저에 낡은 모듈이 남아 있었다)
    아래 `stock.symbol` 한 줄이 화면 전체를 데리고 갔다. 그때 오류 화면은 페이지에
    있던 브라우저 확장을 지목했지만, 스택은 처음부터 끝까지 우리 코드였다.

    **이 줄은 헤더의 보조 동작이다.** 스탬프·보유·목표가가 없는 헤더는 불편하지만,
    종목명·현재가·차트까지 함께 사라지는 것과는 견줄 수 없다. 그래서 종목을 못 받으면
    **아무것도 그리지 않고 조용히 비킨다.**

    훅보다 **아래**에 둔다 — 위에서 돌려보내면 렌더마다 훅 호출 수가 달라진다.
  */
  if (!stock?.symbol) return null;

  const stampButton = (iconOnly: boolean) => (
    <StampButton
      symbol={stock.symbol}
      interval={chartInterval}
      price={price}
      currency={stock.currency}
      week52High={week52High}
      week52Low={week52Low}
      iconOnly={iconOnly}
    />
  );

  /*
    루트에서 `shrink-0` 을 뺐다. 이 줄은 이미 `flex-wrap` 이라 좁아지면 아래로
    접히는데, `shrink-0` 때문에 **접히기 전에 부모를 밀어내** 폰에서 오른쪽 버튼이
    화면 밖으로 잘렸다. 가로 스크롤 컨테이너도 없어서 닿을 방법이 없었다.
  */
  return (
    <div className={cn("flex min-w-0 items-center gap-1.5", className)}>
      {collapseOnMobile && !expanded && (
        <Button
          variant="outline"
          size="sm"
          onClick={() => onExpandedChange?.(true)}
          aria-label="이 종목 동작 펼치기"
          aria-expanded={false}
          title="스탬프 · 보유 등록 · 목표가"
          className={cn(HEADER_ACTION_CLASS, "px-2 sm:hidden")}
        >
          <MoreHorizontal />
        </Button>
      )}

      {/*
        펼친 상태 — 아이콘만 셋, 그리고 다시 접는 ⟫. 글자를 떼면 셋이 96px에 들어가
        종목명이 살아남는다(목업 A안). 무엇인지는 `aria-label`과 길게 누르기 툴팁이
        말해 주고, 데스크톱에서는 아래 줄이 글자를 그대로 단다.
      */}
      {collapseOnMobile && expanded && (
        <div className="flex shrink-0 items-center gap-1 sm:hidden">
          {stampButton(true)}
          <Button
            variant="outline"
            size="sm"
            onClick={openHolding}
            aria-label="보유종목 등록"
            title={gate.authed ? "이 종목을 내 포트폴리오에 등록" : "로그인하면 이 종목을 포트폴리오에 등록할 수 있습니다"}
            className={cn(HEADER_ACTION_CLASS, "px-2")}
          >
            <Briefcase />
          </Button>
          <Button
            variant="outline"
            size="sm"
            onClick={openTarget}
            aria-label="목표가 설정"
            title={gate.authed ? "이 종목의 목표가 설정" : "로그인하면 이 종목의 목표가를 설정할 수 있습니다"}
            className={cn(HEADER_ACTION_CLASS, "px-2")}
          >
            <Target />
          </Button>
          <Button
            variant="outline"
            size="sm"
            onClick={() => onExpandedChange?.(false)}
            aria-label="이 종목 동작 접기"
            aria-expanded
            title="접기"
            className={cn(HEADER_ACTION_CLASS, "px-1.5")}
          >
            <ChevronsRight />
          </Button>
        </div>
      )}

      <div
        className={cn(
          "flex min-w-0 flex-wrap items-center gap-1.5",
          collapseOnMobile && "hidden sm:flex",
        )}
      >
        {stampButton(false)}
        {/*
          「보유종목 등록」.

          보유를 등록하려면 `/holdings`로 가야 했고, 보유가 0건이면 그 화면에도 추가
          폼이 뜨지 않아(사이드 레일이 0건 분기에서 렌더되지 않는다) 진입점이 아예
          없었다. 종목을 보다가 그 자리에서 담는 것이 자연스러운 순서다.
        */}
        <Button
          variant="outline"
          size="sm"
          onClick={openHolding}
          title={gate.authed ? "이 종목을 내 포트폴리오에 등록" : "로그인하면 이 종목을 포트폴리오에 등록할 수 있습니다"}
          className={HEADER_ACTION_CLASS}
        >
          <Briefcase />
          보유종목 등록
        </Button>
        <Button
          variant="outline"
          size="sm"
          onClick={openTarget}
          title={gate.authed ? "이 종목의 목표가 설정" : "로그인하면 이 종목의 목표가를 설정할 수 있습니다"}
          className={HEADER_ACTION_CLASS}
        >
          <Target />
          목표가 설정
        </Button>
      </div>

      <AddHoldingDialog
        open={addOpen}
        onOpenChange={setAddOpen}
        onAdded={() => setAddOpen(false)}
        stock={stock}
        price={frozenPrice}
      />
      <TargetPriceDialog
        open={targetOpen}
        onOpenChange={setTargetOpen}
        stock={stock}
        currentPrice={frozenPrice}
      />
    </div>
  );
}
