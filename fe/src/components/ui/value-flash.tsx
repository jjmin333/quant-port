"use client";

import { useEffect, useState, type ReactNode } from "react";
import {
  clearFlashDirection,
  IDLE_FLASH,
  nextFlashState,
  resetFlashState,
  VALUE_FLASH_MS,
  type FlashDirection,
  type FlashState,
} from "@/lib/value-flash";
import { cn } from "@/lib/utils";

/*
값이 움직이면 그 자리가 잠깐 물든다 — **앱 전체가 이 부품 하나를 쓴다.**

# 왜 하나로 모으는가 (2026-09-04 사용자 요청 4)

이 신호는 마켓 레이더 행에만 있었다(`market-radar-row.tsx`가 자기 `useState` +
`useRef` + 700ms 타이머를 들고 있었다). 그래서 같은 가격이 보유·관심 목록이나
홈 카드에서는 **말없이 바뀌었다** — 사용자는 실시간 차트에서만 시장이 움직이는
것처럼 느낀다.

부품을 복사해 붙이는 대신 하나로 모으는 이유는 규칙이 여럿이기 때문이다: 첫 값에는
색이 없다, 같은 값에는 색이 없다, 종목을 갈아 끼우면 기준을 옮긴다, 색은 다음
갱신 전에 꺼진다. 넷 중 하나만 빠져도 증상은 「가끔 이상하게 깜빡인다」로 똑같고,
복사본이 다섯 개면 그 넷을 다섯 번 맞춰야 한다.

판정 자체는 `lib/value-flash.ts`에 순수 함수로 있다. 여기 남은 것은 **타이머와
색**뿐이다.

# 색은 값의 방향이지 값의 부호가 아니다

`trendTextClass`(등락률 색)와 헷갈리면 안 된다. 저쪽은 「전일 대비 올랐는가」라
하루 종일 같은 색이고, 이쪽은 「방금 움직였는가」라 700ms만 산다. 상승 빨강·하락
파랑이라는 국내 관례는 둘이 공유한다.

# 어디에 쓰는가

가격만이 아니라 **가격에서 나온 모든 금액**이다(사용자 요청 4: 평가금액 등).
값이 시세를 따라 움직이는 자리면 전부 — 평가금액·평가손익·총자산. 반대로 사용자가
입력한 값(매입가·목표가)은 시세로 움직이지 않으므로 감싸지 않는다. 감싸면 화면을
처음 열 때만 한 번 뜨는 색이 되어 뜻이 없다.
*/

/**
 * 방향별 배경. 마켓 레이더가 쓰던 22% 혼합을 그대로 옮겼다 — 눈에 익은 세기다.
 *
 * 내보내는 이유는 `<ValueFlash>`로 감쌀 수 없는 자리가 있기 때문이다 — 이미 자기
 * span을 갖고 통화별로 다르게 그리는 `QuotePrice`가 그렇다. 그런 곳은 클래스만
 * 가져다 쓰되 **판정은 `useValueFlash`로 한다**(색만 복사하고 규칙을 다시 쓰면
 * 이 파일이 하나로 모은 뜻이 없다).
 */
export const FLASH_BG: Record<FlashDirection, string> = {
  up: "bg-[color-mix(in_oklab,var(--profit)_22%,transparent)]",
  down: "bg-[color-mix(in_oklab,var(--loss)_22%,transparent)]",
};

/**
 * 지금 이 값이 물들어야 하는 방향. 아니면 null.
 *
 * `resetKey`는 「이 자리가 다른 것을 그리기 시작했다」는 신호다 — 보통 종목 심볼을
 * 넘긴다. 바뀌면 색 없이 기준만 옮긴다(앞 종목 가격과 비교하지 않기 위해).
 */
export function useValueFlash(
  value: number | null | undefined,
  resetKey?: string | number,
): FlashDirection | null {
  const [state, setState] = useState<FlashState>(IDLE_FLASH);

  /*
    **렌더 중에 조정한다** — 이펙트로 하면 색이 한 프레임 늦게 붙는다(React의
    「prop이 바뀔 때 state 조정」 패턴, `use-quote-stream`이 시장 전환에 같은 것을 쓴다).
  */
  const [seenKey, setSeenKey] = useState(resetKey);
  if (seenKey !== resetKey) {
    setSeenKey(resetKey);
    setState(resetFlashState(value));
  } else {
    const next = nextFlashState(state, value);
    // 참조가 그대로면 아무 일도 없었다는 뜻이라 setState를 부르지 않는다.
    if (next !== state) setState(next);
  }

  /*
    색을 끄는 타이머. **`seq`에 건다** — 같은 방향으로 연달아 움직이면 `direction`은
    안 바뀌는데 타이머는 다시 걸려야 한다(안 그러면 첫 움직임 기준으로 꺼진다).
  */
  useEffect(() => {
    if (state.direction === null) return;
    const id = window.setTimeout(() => setState(clearFlashDirection), VALUE_FLASH_MS);
    return () => window.clearTimeout(id);
  }, [state.direction, state.seq]);

  return state.direction;
}

export type ValueFlashProps = {
  /** 색을 정하는 숫자. 화면에 그리는 것과 **같은 값**이어야 한다. */
  value: number | null | undefined;
  /** 이 자리가 다른 종목을 그리기 시작하면 넘긴다(보통 심볼). */
  resetKey?: string | number;
  children: ReactNode;
  className?: string;
  /**
   * 감싸는 태그. 기본은 `span`이고, 표 셀처럼 이미 자리가 있는 곳은 그대로 둔다.
   * 블록 안에 넣어야 하면 `div`.
   */
  as?: "span" | "div";
};

/**
 * 값이 움직이면 잠깐 물드는 자리.
 *
 * 숫자를 **직접 그리지 않는다** — 가격은 `PriceDisplay`, 금액은 `PriceAmountPlain`,
 * 등락률은 그냥 문자열이라 그리는 방법이 자리마다 다르다. 이 부품이 정하는 것은
 * 「언제 어떤 색인가」뿐이고, 무엇을 그리는지는 children이 정한다.
 */
export function ValueFlash({
  value,
  resetKey,
  children,
  className,
  as = "span",
}: ValueFlashProps) {
  const direction = useValueFlash(value, resetKey);
  const Tag = as;
  return (
    <Tag
      className={cn(
        // rounded/px는 색이 숫자에 딱 붙지 않게 한다 — 없으면 글자가 배경에 갇혀 보인다.
        "rounded px-0.5 transition-colors",
        direction && FLASH_BG[direction],
        className,
      )}
    >
      {children}
    </Tag>
  );
}
