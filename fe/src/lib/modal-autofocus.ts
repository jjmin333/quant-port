/**
 * 모달이 열릴 때 **키보드가 저절로 올라오지 않게** 한다 (2026-09-03 사용자 결정).
 *
 * # 왜
 *
 * 모바일·태블릿에서 모달이 뜨는 즉시 키보드가 화면 절반을 덮었다. 메모를 적으려고
 * 연 창이라도, 사용자가 처음 보는 것은 「무엇을 적는 창인가」(제목·종목·현재가)이지
 * 자판이 아니다. 키보드는 입력칸을 **누른 순간에만** 올라오면 된다.
 *
 * 데스크톱은 반대다. 거기서는 자동 포커스가 바로 타이핑을 시작하게 해 주고, 잃을
 * 화면도 없다. 그래서 **기기로 가른다** — 화면 폭이 아니라 포인터의 성격으로.
 * 좁은 창으로 줄여 놓은 데스크톱 브라우저는 여전히 마우스·키보드이기 때문이다.
 *
 * # 두 갈래를 다 막아야 한다
 *
 * 창이 열릴 때 포커스를 옮기는 주체가 둘이다:
 *
 *   ① Radix의 `onOpenAutoFocus` — 창 안 첫 포커스 가능한 요소로 옮긴다.
 *   ② 입력칸에 직접 붙은 React `autoFocus` 속성 — 마운트될 때 스스로 포커스한다.
 *
 * ①은 `DialogContent`·`SheetContent`가 한 곳에서 막고(그래서 새 모달도 자동으로
 * 같게 동작한다), ②는 그 속성을 쓰는 자리에서 `useModalAutoFocus()`로 갈라 준다.
 * ①만 막으면 자판은 그대로 올라온다 — 실제로 그렇게 새는 자리가 다섯 곳이었다.
 */

export type PointerEnvironment = {
  /** `(pointer: coarse)` — 손가락이 주 입력 수단인가. */
  coarsePointer: boolean;
  /** 화면이 터치를 지원하는가(`navigator.maxTouchPoints`). */
  maxTouchPoints: number;
  /** `(pointer: fine)` — 마우스·트랙패드·펜이 있는가. */
  finePointer: boolean;
};

/**
 * 이 기기에서 모달의 자동 포커스를 막을 것인가. **순수 함수** — 판정 규칙만 담는다.
 *
 * `coarse`이면서 `fine`이 아닌 기기가 폰·태블릿이다. 터치 스크린 노트북은 둘 다
 * 참이라 데스크톱으로 본다 — 거기엔 물리 키보드가 있어서 자동 포커스가 이득이다.
 * 미디어 쿼리를 못 읽는 환경(오래된 브라우저·테스트)에서는 터치 지점 수로 되짚는다.
 */
export function shouldSuppressModalAutoFocus(env: PointerEnvironment): boolean {
  if (env.coarsePointer) return !env.finePointer;
  // coarse를 못 읽었을 때의 폴백. 터치가 있고 마우스가 없으면 폰·태블릿으로 본다.
  if (!env.finePointer && env.maxTouchPoints > 0) return true;
  return false;
}

/** 지금 브라우저의 포인터 환경. 서버에서는 「데스크톱」으로 답한다. */
export function readPointerEnvironment(): PointerEnvironment {
  if (typeof window === "undefined" || typeof window.matchMedia !== "function") {
    return { coarsePointer: false, maxTouchPoints: 0, finePointer: true };
  }
  return {
    coarsePointer: window.matchMedia("(pointer: coarse)").matches,
    finePointer: window.matchMedia("(pointer: fine)").matches,
    maxTouchPoints: typeof navigator === "undefined" ? 0 : navigator.maxTouchPoints ?? 0,
  };
}

/** 폰·태블릿인가 — 위 둘을 이어 붙인 것. */
export function isTouchFirstDevice(): boolean {
  return shouldSuppressModalAutoFocus(readPointerEnvironment());
}

/**
 * Radix `onOpenAutoFocus` 기본 처리. 폰·태블릿에서는 포커스를 **창 자체**에 준다 —
 * 포커스 함정과 Esc 닫기는 그대로 살고, 키보드만 올라오지 않는다.
 */
export function suppressAutoFocusOnTouch(event: Event): void {
  if (!isTouchFirstDevice()) return;
  event.preventDefault();
  (event.currentTarget as HTMLElement | null)?.focus({ preventScroll: true });
}
