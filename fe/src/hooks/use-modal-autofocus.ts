"use client";

import { useState } from "react";
import { isTouchFirstDevice } from "@/lib/modal-autofocus";

/**
 * 모달 안 입력칸에 `autoFocus`를 줘도 되는가 — 폰·태블릿이면 false.
 *
 * **마운트 시점에 한 번만 읽는다.** `autoFocus`는 마운트 때만 쓰이는 속성이라
 * 나중에 값이 바뀌어도 소용이 없고, 모달 본문은 사용자가 연 뒤에야 마운트되므로
 * 렌더 중에 브라우저를 읽어도 서버 렌더와 어긋날 일이 없다.
 *
 * 배경과 규칙은 `lib/modal-autofocus.ts` 헤더에 있다.
 */
export function useModalAutoFocus(): boolean {
  const [allowed] = useState(() => !isTouchFirstDevice());
  return allowed;
}
