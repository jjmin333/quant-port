"use client";

import { useEffect, useSyncExternalStore } from "react";
import { isPinned, pinSymbol, subscribePinned } from "@/lib/pinned-symbols";

/*
「지금 이 종목을 열어 두고 있다」를 앱 전체에 적는 훅 (2026-09-15 사용자 요청 4).

무엇을 위한 것인지는 `lib/pinned-symbols` 머리글에 있다. 이 파일은 **React 생애주기와
그 집합을 잇는 일**만 한다.

# 왜 이펙트인가 (렌더 중이 아니라)

등록은 화면 밖 상태를 바꾸는 일이라 렌더 중에 하면 안 된다. 정리 함수가 곧 해제라서
언마운트·종목 교체·모달 닫기가 전부 같은 길로 정리된다 — 닫는 길이 둘이 되면 어느
쪽이 먼저인지에 따라 남는 핀이 생긴다(허브가 연결을 정리하는 방식과 같은 이유).
*/

/**
 * `symbol`을 「보고 있는 종목」으로 등록한다.
 *
 * @param active 지금 실제로 열려 있는가. 모달은 `open`, 전용 화면은 `true`.
 *
 * 심볼이 비었거나 `active`가 거짓이면 아무것도 등록하지 않는다 — 화면이 종목을 아직
 * 못 정한 동안(로딩) 빈 값으로 부르는 일이 흔하다.
 */
export function usePinnedSymbol(
  symbol: string | null | undefined,
  assetType: string | null | undefined,
  active = true,
): void {
  useEffect(() => {
    if (!active || !symbol || !assetType) return;
    return pinSymbol(symbol, assetType);
  }, [symbol, assetType, active]);
}

/**
 * 이 종목이 지금 어딘가에서 열려 있는가.
 *
 * 스트림을 여는 훅이 「숨어도 이 연결은 놓지 않는다」를 판단할 때 쓴다. 자기 자신이
 * 핀을 건 화면도 참을 받는다 — 그 화면이 곧 「열어 둔」 주체다.
 */
export function useIsPinned(symbol: string | null | undefined): boolean {
  const sym = symbol ?? "";
  return useSyncExternalStore(
    subscribePinned,
    () => isPinned(sym),
    // 서버에는 핀이라는 것이 없다. 거짓을 주면 첫 렌더가 클라이언트와 같아진다.
    () => false,
  );
}
